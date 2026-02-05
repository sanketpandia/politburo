import AbstractSymbolGroup from "./AbstractSymbolGroup.mjs";
import Loader from "./Loader.mjs";

import RBush from "../3rd-party/rbush.mjs";
import knn from "../3rd-party/rbush-knn.mjs";

import ExpandBox from "../geometry/ExpandBox.mjs";
import Geometry from "../geometry/Geometry.mjs";

import Spider from "../symbols/Spider.mjs";
import MultiSymbol from "../symbols/MultiSymbol.mjs";
import CircleStroke from "../symbols/CircleStroke.mjs";
import CircleFill from "../symbols/CircleFill.mjs";
import TextLabel from "../symbols/TextLabel.mjs";

import intersectBboxes from "./cluster/intersectBboxes.mjs";

function defaultSymbolizer(symbols) {
	const geom = symbols[0].geometry;

	return [
		new CircleStroke(geom, { radius: 40 }),
		new CircleFill(geom, { radius: 40, colour: "#3388ff80" }),
		new TextLabel(geom, {
			str: symbols.length,
			align: "center",
			baseline: "middle",
			cache: true,
			interactive: false,
			outlineColour: "black",
			outlineWidth: 0.15,
		}),
	];
}

// The default action when clicking on a (non-spider) cluster is to zoom
// to the bounding box of the components of that cluster, but no closer
// than the clusterer's `scaleLimit`.
function defaultOnClusterClick(ev, clusterer, items, bbox) {
	let platina = ev.target.symbols[0]._inAcetate?.platina;
	let map = platina?.map;

	const { minX, minY, maxX, maxY } = bbox;
	const [w, h] = platina.pxSize;

	const center = new Geometry(ev.target.geometry.crs, [
		(minX + maxX) / 2,
		(minY + maxY) / 2,
	]);
	const scale = Math.max((maxX - minX) / w, (maxY - minY) / h, clusterer.scaleLimit);

	return (map ?? platina).setView({ center, scale });
}

class PointRBush extends RBush {
	toBBox({ x, y }) {
		return { minX: x, minY: y, maxX: x, maxY: y };
	}
	compareMinX(a, b) {
		return a.x - b.x;
	}
	compareMinY(a, b) {
		return a.y - b.y;
	}
}

/**
 * @class Clusterer
 * @inherits AbstractSymbolGroup
 *
 * Handles symbols with point geometries, and clusters them together whenever
 * they're too close to each other.
 */

/*
 * This implements a naïve algorithm:
 * - Discrete steps of clustering, depending on scale. Similar to "zoom levels".
 * - One r-bush per "zoom level"
 *   - Contains clusters for that grouping
 *   - A cluster of just one symbol is passed through
 *   - A cluster of several symbols gets replaced with a cluster symbol
 * - Symbols can be added **and** removed from the clusterer
 *   - Adding a symbol shall do a kNN search on the r-bush for any close cluster
 *   - All known r-bushes will add/remove symbols being added/removed.
 * - Changing the geometry of a smbol shall remove and re-add it
 */

// TODO: Somehow move the rbush generation code to a worker: when the log2scale
// changes, do all the work of calculating the clusters in a worker.

export default class Clusterer extends AbstractSymbolGroup {
	constructor({
		/**
		 * @option clusterSymbolizer: Function
		 * Defines how to spawn symbols for the clusters. The function will
		 * receive an `Array` of `GleoSymbol`s as its first parameter, and must
		 * return an `Array` of `GleoSymbol`s which must represent the cluster.
		 */
		clusterSymbolizer = defaultSymbolizer,

		/**
		 * @option distance: Number = 80
		 * The minimum distance, in CSS pixels, for two `GleoSymbol`s to not be
		 * clustered. By implication, that's also the maximum diameter of a cluster.
		 */
		distance = 80,

		/**
		 * @option clusterSetFactor: Number = 1
		 * How many cluster sets to calculate per every doubling/halving
		 * of the scale.
		 *
		 * e.g. The default value of 1 will create a set of clusters for every
		 * doubling of the scale. A value of 2 will create a set of clusters
		 * every time the scale varies by a factor of square root of 2, and
		 * a value of e.g. 0.5 will create a set of clusters every time the
		 * scale quadruples.
		 */
		clusterSetFactor = 1,

		/**
		 * @option scaleLimit: Number = 0
		 * Clusters will not be calculated past this scale factor. Instead, the
		 * most detailed clusters will be expandable `Spider`s.
		 *
		 * TODO: Default to `undefined`, and calculate from the platina's `minSpan`.
		 */
		scaleLimit = 1000,

		/**
		 * @option onClusterClick: Function
		 * An event handler that will run when clicking on a non-spider cluster.
		 *
		 * This function receives as parameters: the event, a reference to this
		 * `Clusterer`, and `Array` of `GleoSymbol`s with the items in the cluster,
		 * and an `ExpandBox` covering those items.
		 *
		 * The default is to perform a `fitBounds` to the bounding box of the
		 * items in that cluster, zooming up to `scaleLimit` at most.
		 * @alternative
		 * @option onClusterClick: Boolean
		 * Setting this to `false` will disable cluster click events.
		 */
		onClusterClick = defaultOnClusterClick,

		/**
		 * @option spiderOptions: Spider Options
		 *
		 * A set of options for the `Spider` constructor, that shall be applied
		 * to any `Spider`s spawned by this clusterer.
		 */
		spiderOptions = {},

		...opts
	} = {}) {
		super(opts);

		this.#boundOnViewChange = this.#onViewChange.bind(this);
		this.#boundOnCrsChange = this.#onCrsChange.bind(this);
		this.#distance = distance * (devicePixelRatio ?? 1);
		this.#clusterSymbolizer = clusterSymbolizer;
		this.#clusterSetFactor = clusterSetFactor;
		this.#onClusterClick = onClusterClick;
		this.#scaleLimit = scaleLimit;
		this.#boundRelayEvent = this.#relayEvent.bind(this);
		this.#spiderOptions = spiderOptions;
	}

	#distance;
	#rbushes = new Map();
	#log2scale;
	#log2offset = 0;
	#boundOnViewChange;
	#boundOnCrsChange;
	#crs;
	#onClusterClick;
	#clusterSymbolizer;
	#clusterSetFactor;
	#visibleSymbols = [];
	#bbox; // Last platina bbox where visibility was (re)calculated
	#dataBbox = new ExpandBox(); // Extents of the contained symbols
	#scaleLimit;
	#spiderScaleLog = -Infinity;
	#spiderOptions;

	_addToPlatina(platina) {
		super._addToPlatina(platina);

		this.platina.on("viewchanged", this.#boundOnViewChange);
		this.platina.on("crsoffset", this.#boundOnCrsChange);
		this.platina.on("crschange", this.#boundOnCrsChange);
		this.#crs = this.platina.crs;
		this.#resetBushes();

		// Calculate an offset to (later) snap the log2 of the scale factor to round numbers.
		// This is an optimization for a common use case: snapping to the scale factor
		// of a tile pyramid. It's a somehow naïf approach since it assumes the pyramid
		// uses power-of-two scaling.
		const zoomSnapActuator = (
			this.target.actuators ?? this.target.map?.actuators
		)?.get("zoomsnap");
		if (zoomSnapActuator) {
			this.#log2offset =
				(Math.log2(zoomSnapActuator.snapScale(this.target.scale)) *
					this.#clusterSetFactor) %
				1;
		}

		// Calculate the minimum log2(scale), which is when clusters are
		// spiderified.
		if (this.#scaleLimit) {
			this.#spiderScaleLog = Math.ceil(
				Math.log2(this.#scaleLimit) * this.#clusterSetFactor - this.#log2offset
			);
		}

		return this;
	}

	#animFrame;

	/// @property bbox: Array of Number
	/// The bounding box of the data for the clusterer, in the CRS of the
	/// platina this clusterer is in, in the form `[minX, minY, maxX, maxY]`.
	/// Read-only.
	get bbox() {
		if (!this.#crs) {
			throw new Error("The clusterer needs to be in a platina with a CRS");
		}
		const log2scale = this.getCurrentLog2scale();
		if (!this.#rbushes.has(log2scale)) {
			this.#buildBush(log2scale);
		}
		const bush = this.#rbushes.get(log2scale).data;

		return [bush.minX, bush.minY, bush.maxX, bush.maxY];
	}

	_addSymbols(symbols) {
		cancelAnimationFrame(this.#animFrame);
		this.#animFrame = requestAnimationFrame(() => this.#resetBushes());
		symbols.forEach((s) => this.#dataBbox.expandGeometry(s.geometry));

		super._addSymbols(symbols);

		return this;
	}

	// Does *not* shrink this.#dataBbox
	remove(symbol) {
		if (symbol) {
			if (symbol instanceof Loader) {
				return super.remove(symbol);
			} else {
				cancelAnimationFrame(this.#animFrame);
				this.#animFrame = requestAnimationFrame(() => this.#resetBushes());
				return super.remove(symbol);
			}
		} else if (this.platina) {
			this.platina.off("viewchanged", this.#boundOnViewChange);
			this.platina.off("crsoffset", this.#boundOnCrsChange);
			this.platina.off("crschange", this.#boundOnCrsChange);

			// Remove currently visible clusters, by removing all clusters with
			// the r-bush for the current scale (skipping `undefined` cluster
			// symbols that haven't been needed yet)
			const removableSymbols = this.#rbushes
				.get(this.getCurrentLog2scale())
				.all()
				.map((item) => {
					return item.symbol;
				})
				.filter((i) => !!i)
				.flat();
			this.fire("symbolsremoved", {
				symbols: removableSymbols,
			});
			this.target.multiRemove(removableSymbols);

			// Reset state, so removing & re-adding the clusterer does a refresh
			this.#visibleSymbols = [];
			this.#bbox = undefined;
			this.#log2scale = undefined;

			super.remove();
		}
		return this;
	}

	_removeSymbols(symbols) {
		cancelAnimationFrame(this.#animFrame);
		this.#animFrame = requestAnimationFrame(() => this.#resetBushes());
		return super._removeSymbols(symbols);
	}

	empty() {
		cancelAnimationFrame(this.#animFrame);
		this.#animFrame = requestAnimationFrame(() => this.#resetBushes());
		return super.empty();
	}

	// Aux, intended for internal use
	getCurrentLog2scale() {
		return Math.max(
			Math.floor(
				Math.log2(this.platina.scale) * this.#clusterSetFactor - this.#log2offset
			),
			this.#spiderScaleLog
		);
	}

	/**
	 * @property scaleLimit
	 * Value of the `scaleLimit` option. Read-only.
	 */
	get scaleLimit() {
		return this.#scaleLimit;
	}

	#onViewChange(ev) {
		if (!this.platina?.scale) {
			return;
		}
		if (!this.#crs) {
			return;
		}
		const log2scale = this.getCurrentLog2scale();

		const crs = this.#crs;
		const rawBBox = this.platina.bbox;
		const platinaBBox = new ExpandBox();
		// platinaBBox.expandPair(crs.offsetToBase([rawBBox.minX, rawBBox.minY]));
		// platinaBBox.expandPair(crs.offsetToBase([rawBBox.maxX, rawBBox.maxY]));
		platinaBBox.expandPair([rawBBox.minX, rawBBox.minY]);
		platinaBBox.expandPair([rawBBox.maxX, rawBBox.maxY]);

		if (log2scale !== this.#log2scale) {
			// console.info("Clusterer scale change: ", this.#log2scale, "→", log2scale);

			if (!this.#rbushes.has(log2scale)) {
				this.#buildBush(log2scale);
			}
		} else if (this.#bbox?.containsBox(platinaBBox)) {
			return;
		}

		this.#log2scale = log2scale;
		this.#bbox = platinaBBox.clone().expandPercentage(0.2);

		let removableSymbols = this.#visibleSymbols;
		const bush = this.#rbushes.get(log2scale);

		// Usually, the cluster domain and the viewport have a simple intersection
		// (`bush.search(this.#bbox)`), but edge cases
		// involving the antimeridian call for calculating multiple intersections
		const intersections = intersectBboxes(bush.data, this.#bbox, crs);
		const spiders = this.#spiderScaleLog === log2scale;

		let addableSymbols = intersections
			.map((bushbox) =>
				bush.search(bushbox).map((item) => this.#symbolizeCluster(item, spiders))
			)
			.flat();

		let uniqueAddableSymbols = addableSymbols.filter(
			(s) => !removableSymbols.includes(s)
		);
		let uniqueRemovableSymbols = removableSymbols.filter(
			(s) => !addableSymbols.includes(s)
		);

		this.target.multiRemove(uniqueRemovableSymbols);
		this.target.multiAdd(uniqueAddableSymbols);

		this.#visibleSymbols = addableSymbols;
	}

	// Expects a rbush item as parameter
	#symbolizeCluster(item, spiders) {
		if (item.symbol) {
			return item.symbol;
		}

		let symbol;
		if (item.sources.length === 1) {
			// Clusters of only one symbol don't need symbolization; reuse that single symbol
			symbol = item.sources[0];
		} else if (spiders) {
			// At the highest zoom level (lowest scale), clickable spiders are used
			symbol = new Spider(
				this.#clusterSymbolizer(item.sources),
				item.sources,
				this.#spiderOptions
			);

			/**
			 * @event expand: CustomEvent
			 * Fired when one of the `Spider`s of the clusterer expands.
			 * The `Spider` in question is in the event's `detail`
			 * @event collapse: CustomEvent
			 * Fired when one of the `Spider`s of the clusterer collapses.
			 * The `Spider` in question is in the event's `detail`
			 */
			symbol.on("collapse", this.#boundRelayEvent);
			symbol.on("expand", this.#boundRelayEvent);
		} else {
			// At any other zoom levels, use a MultiSymbol
			symbol = new MultiSymbol(this.#clusterSymbolizer(item.sources));

			if (this.#onClusterClick) {
				symbol.cursor = "pointer";

				let bbox = new ExpandBox();
				item.sources.forEach((s) =>
					bbox.expandGeometry(s.geometry.toCRS(this.platina.crs))
				);

				symbol.on("click", (ev) =>
					this.#onClusterClick(ev, this, item.sources, bbox)
				);
			}
		}

		return (item.symbol = symbol);
	}

	#boundRelayEvent;

	#relayEvent(ev) {
		const myEv = new ev.constructor(ev.type, { ...ev, detail: ev.target });
		this.dispatchEvent(myEv);
	}

	#buildBush(log2scale) {
		// Max distance between points to cluster together, in CRS units
		const dist = Math.pow(2, log2scale / this.#clusterSetFactor) * this.#distance;
		const crs = this.#crs;

		const bush = new PointRBush();

		this.symbols.forEach((s) => {
			// if (s.geometry.crs.name !== crs.name) {
			/// FIXME: This can lead to a chain of reprojections, and a
			/// subsequent loss of precision, if the map/platina changes
			/// CRSs frequently.
			/// TODO: Maybe use a `WeakMap` to hold the reprojected geometries?
			s.geometry = s.geometry.toCRS(crs);
			// }
			const [x, y] = s.geometry.coords;
			if (!isFinite(x) || !isFinite(y)) {
				return console.warn(
					`Could not add symbol to cluster: non-finite coordinates`
				);
			}

			/// TODO: Consider implementing wrapping logic in the clusterer.
			/// Take the first known point and store as wrapping reference;
			/// Any subsequent points undergo wrapping logic: if away more than
			/// half a period, point gets wrapped.

			const nearest = knn(bush, x, y, 1, undefined, dist);

			if (nearest?.length) {
				// Add to existing cluster
				nearest[0].sources.push(s);
			} else {
				// Create a new cluster with no symbol
				bush.insert({
					x,
					y,
					sources: [s],
					symbol: undefined,
				});
			}
		});

		this.#rbushes.set(log2scale, bush);
		/**
		 * @event build: CustomEvent
		 * Fired when one of the the internal data structures have been built (due to
		 * data being added or removed).
		 */
		this.fire("build", log2scale);
		return bush;
	}

	// Removes all of the bushes, forcing their (re)building
	#resetBushes() {
		/**
		 * @event reset: CustomEvent
		 * Fired when the internal data structures have been reset (due to
		 * data being added or removed).
		 */
		this.fire("reset");
		this.#rbushes.clear();
		this.#log2scale = NaN;
		this.#bbox = undefined;
		return this.#onViewChange();
	}

	#onCrsChange(ev) {
		this.#crs = ev.detail.newCRS;
		return this.#resetBushes();
	}

	/**
	 * @method getParent(symbol: GleoSymbol, scale?: Number): GleoSymbol
	 * Returns the symbol representing the cluster that the given symbol belongs to,
	 * at the given scale. If scale is not given, the current scale will be
	 * used.
	 *
	 * Akin to leaflet-markercluster's `getVisibleParent`.
	 */
	getParent(symbol, scale) {
		const log2scale = Math.max(
			Math.floor(
				Math.log2(scale ?? this.platina.scale) * this.#clusterSetFactor -
					this.#log2offset
			),
			this.#spiderScaleLog
		);

		const [x, y] = symbol.geometry.toCRS(this.#crs).coords;

		const dist = Math.pow(2, log2scale / this.#clusterSetFactor) * this.#distance;
		const bush = this.#rbushes.get(log2scale) ?? this.#buildBush(log2scale);
		const nearest = knn(bush, x, y, 1, undefined, dist)[0];

		this.#symbolizeCluster(nearest, this.#spiderScaleLog === log2scale);
		return nearest.symbol;
	}

	/**
	 * @method getUnclusterScaleFor(symbol: GleoSymbol): Number
	 * Returns the minimum scale (in the map's CRS units) where the given symbol
	 * is not in a cluster.
	 *
	 * If the symbol cannot be shown outside of a cluster (i.e. it belongs to a
	 * `Spider` at the `Clusterer`s `scaleLimit`), then the return value will
	 * be zero.
	 */
	getUnclusterScaleFor(symbol) {
		const [x, y] = symbol.geometry.toCRS(this.#crs).coords;

		const log2scale = this.getCurrentLog2scale();

		for (let i = log2scale; i >= this.#spiderScaleLog; i--) {
			// Max distance between points to cluster together, in CRS units
			const dist = Math.pow(2, i / this.#clusterSetFactor) * this.#distance;

			const bush = this.#rbushes.get(i) ?? this.#buildBush(i);

			// Get *one* cluster. It'll be the one containing the symbol.
			const nearest = knn(bush, x, y, 1, undefined, dist)[0];

			if (nearest.sources.length === 1) {
				// The cluster has only one symbol in it - this is how far to
				// zoom in.

				return Math.pow(2, (i + this.#log2offset) / this.#clusterSetFactor);
			} else if (i === this.#spiderScaleLog) {
				return 0;
			}
		}

		return this;
	}

	/**
	 * @method zoomToShowSymbol(symbol: GleoSymbol, setViewOpts?: SetView Options): this
	 * Akin to leaflet-markercluster's `zoomToShowLayer()` - zooms into the map
	 * far enough so the given symbol is not clustered; if it's in a spider
	 * it will zoom into it and expand the spider.
	 */
	zoomToShowSymbol(symbol, setViewOpts = {}) {
		const scale = this.getUnclusterScaleFor(symbol);

		// Find a target that has setView (i.e. traverse through possible nested
		// SymbolGroups)
		let target = this.target;
		while (!target.setView) {
			if (!target) {
				return;
			}
			target = target.target;
		}

		if (scale > 0) {
			target.setView({
				duration: 2000,
				...setViewOpts,
				center: symbol.geometry,
				scale: scale,
			});
			return this;
		} else {
			// const scale = Math.pow( 2, (this.#spiderScaleLog + this.#log2offset) / this.#clusterSetFactor );

			target.setView({
				duration: 2000,
				...setViewOpts,
				center: symbol.geometry,
				scale: this.#scaleLimit,
			});

			// Delay the spider expansion by one frame, to prevent a race
			// condition with a delayed #resetBushes(). This can trigger when
			// `zoomToShowSymbol` is called just after adding data to a clusterer.
			requestAnimationFrame(() => {
				const spider = this.getParent(symbol, scale);
				spider.expand();
			});
		}
	}
}
