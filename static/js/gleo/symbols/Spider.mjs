import GleoSymbol from "./Symbol.mjs";
import Callout from "./Callout.mjs";
// import ExtrudedPoint from "./ExtrudedPoint.mjs";

/**
 * @class Spider
 * @inherits GleoSymbol
 * @relationship compositionOf GleoSymbol, 0..1, 0..n
 *
 * Similar to `MultiSymbol`, a `Spider` is a logical grouping of `GleoSymbol`s.
 *
 * A `Spider` has *two* sets of symbols: one collapsed, and one expanded; and
 * at any given time it will be displayed as either.
 *
 * When `click`ing on any of the collapsed symbols, they will be replaced by
 * the expanded ones. Clicking on the map will switch to the collapsed ones again.
 *
 * In addition, symbols in the expanded set will be automatically offset, and
 * `Callout`s will be added - these are the "legs" of the `Spider`.
 *
 */

const τ = Math.PI * 2; // Tau
// const halfπ = Math.PI / 2;

export default class Spider extends GleoSymbol {
	#collapsed = [];
	#expanded = [];
	#callouts = [];
	#expandedState = false;

	#boundOnMapClick;
	#target;
	#calloutOptions = {};
	#calloutLength = 0;

	/**
	 * @constructor Spider(collapsedSymbols: Array of GleoSymbol, expandedSymbols: Array of GleoSymbol, opts?: Spider Options)
	 */
	constructor(
		collapsed,
		expanded,
		{
			/**
			 * @option width: Number = undefined
			 * The width of the `Callout`s. If not specified, the `Callout` default is used.
			 * @option colour: Colour = undefined
			 * The colour of the `Callout`s. If not specified, the `Callout` default is used.
			 * @option length: Number = 60
			 * The length of the `Callout`s, in CSS pixels.
			 * @option expandAnimationDuration
			 * The animation of the leg expansion animation, in milliseconds. Set to
			 * zero to disable.
			 */
			width = undefined,
			colour = undefined,
			length = 60,
			expandAnimationDuration = 500,
		} = {}
	) {
		super();
		this.#collapsed = collapsed;
		// this.#expanded = expanded;
		this.#boundOnMapClick = this.collapse.bind(this);
		const onCollapseClick = this.expand.bind(this);

		this.#collapsed.forEach((s) => {
			s.on("click", onCollapseClick);
			s.cursor = "pointer";
		});

		this.#expanded = expanded.filter((s) => !!s.geometry);

		this.#calloutOptions = { width, colour };
		this.#expandAnimationDuration = expandAnimationDuration;
		this.#calloutLength = length;
	}

	addTo(target) {
		if (!target.crs) {
			console.warn("Cannot add a spider unless the target platina has a known CRS");
		}

		// Calculate simplistic centroid of expanded symbols
		let [x, y] = [0, 0];
		this.#expanded.forEach((ext, i) => {
			const geom = ext.geometry.toCRS(target.crs);
			x += geom.coords[0];
			y += geom.coords[1];
		});
		const l = this.#expanded.length;
		x /= l;
		y /= l;

		// Order expanded symbols by their angle relative to the spider's centroid
		// - Wrap symbols in a data structure
		// - Calculate delta to centroid and store its arcTan on the data structure
		// - Sort
		// - Unwrap
		const items = this.#expanded
			.map((s) => {
				const geom = s.geometry.toCRS(target.crs);
				const Δx = geom.coords[0] - x;
				const Δy = geom.coords[1] - y;
				const θ = Math.atan2(Δx, Δy);
				return {
					symbol: s,
					θ: θ >= 0 ? θ : θ + τ,
				};
			})
			.sort((a, b) => a.θ - b.θ);

		const Δ = τ / l;

		let bias = (this.#bias = items.reduce((b, item, i) => b + item.θ - Δ * i, 0) / l);

		this.#expanded = items.map((w) => w.symbol);

		this.#callouts = this.#expanded.map((ext, i) => {
			const θ = i * Δ + bias;
			ext.offset = [
				this.#calloutLength * Math.sin(θ),
				this.#calloutLength * Math.cos(θ),
			];
			return new Callout(ext.geom, { ...this.#calloutOptions, offset: ext.offset });
		});

		if (this.#expandedState) {
			target.multiAdd(this.#expanded);
			target.multiAdd(this.#callouts);
			target.once("click", this.#boundOnMapClick);
		} else {
			target.multiAdd(this.#collapsed);
		}
		this.#target = target;
		return this;
	}

	remove() {
		this.#expandedState
			? (this.#target.multiRemove(this.#expanded),
			  this.#target.multiRemove(this.#callouts))
			: this.#target.multiRemove(this.#collapsed);
		this.#target.off("click", this.#boundOnMapClick);
		this.#target = undefined;
		return this;
	}

	setGeometry(geom) {
		this.geom = geom;
		this.#expanded.forEach((s) => s.setGeometry(geom));
		this.#callouts.forEach((s) => s.setGeometry(geom));
		this.#collapsed.forEach((s) => s.setGeometry(geom));
		return this;
	}

	/**
	 * @section Lifetime methods
	 * @method expand(): this
	 * Displays the "expanded" set of symbols, and removes the "collapsed" ones.
	 */
	expand(ev) {
		if (this.#expandedState) {
			return;
		}
		this.#expandedState = true;
		if (this.#target) {
			this.#target.multiRemove(this.#collapsed);
			this.#target.multiAdd(this.#expanded);
			this.#target.multiAdd(this.#callouts);
			this.#target.on("click", this.#boundOnMapClick);

			if (this.#expandAnimationDuration > 0) {
				this.#expandStartTimestamp = performance.now();
				this.#expandFrame();
			}
		}

		if (ev) {
			ev.stopPropagation();
		}
		/// @event expand: CustomEvent
		/// Fired when the spider expands for any reason
		this.fire("expand");
		return this;
	}

	#expandStartTimestamp;
	#expandAnimationDuration;
	#bias;
	#expandFrame() {
		const elapsed = Math.min(
			1,
			(performance.now() - this.#expandStartTimestamp) /
				this.#expandAnimationDuration
		);

		const l = this.#expanded.length;
		const Δ = τ / l;

		this.#callouts.forEach((callout, i) => {
			const θ = i * Δ + this.#bias;
			this.#expanded[i].offset = callout.offset = [
				this.#calloutLength * Math.sin(θ) * elapsed,
				this.#calloutLength * Math.cos(θ) * elapsed,
			];
		});

		if (elapsed < 1) {
			requestAnimationFrame(this.#expandFrame.bind(this));
		}
	}

	/**
	 * @method collapse(): this
	 * Displays the "collapsed" set of symbols, and removes the "expanded" ones.
	 */
	collapse() {
		if (!this.#expandedState) {
			return;
		}
		this.#expandedState = false;
		if (this.#target) {
			this.#target.multiAdd(this.#collapsed);
			this.#target.multiRemove(this.#expanded);
			this.#target.multiRemove(this.#callouts);
			this.#target.off("click", this.#boundOnMapClick);
		}
		/// @event collapse: CustomEvent
		/// Fired when the spider collapses for any reason
		this.fire("collapse");
		return this;
	}

	/**
	 * @method toggle(): this
	 */
	toggle() {
		if ((this.#expandedState = !this.#expandedState)) {
			return this.expand();
		} else {
			return this.collapse();
		}
	}
}
