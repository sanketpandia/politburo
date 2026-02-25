import ButtonGroup from "./ButtonGroup.mjs";
import ButtonToggle from "./ButtonToggle.mjs";

import Geometry from "../geometry/Geometry.mjs";

import MultiSymbol from "../symbols/MultiSymbol.mjs";

import Stroke from "../symbols/Stroke.mjs";
import Fill from "../symbols/Fill.mjs";
import CircleStroke from "../symbols/CircleStroke.mjs";
import CircleFill from "../symbols/CircleFill.mjs";

const IDLE = Symbol("IDLE"),
	NEW_POINT = Symbol("NEW_POINT"),
	NEW_LINE = Symbol("NEW_LINE"),
	NEW_POLY = Symbol("NEW_POLY");

function defaultPointSymbolizer(geometry) {
	return [new CircleStroke(geometry), new CircleFill(geometry)];
}
function defaultLineSymbolizer(geometry) {
	return [new Stroke(geometry)];
}
function defaultPolygonSymbolizer(geometry) {
	return [
		new Stroke(geometry),
		new Fill(geometry, { colour: [0x33, 0x88, 0xff, 0x88] }),
	];
}
function defaultNodeSymbolizer(geometry, interactive) {
	return [
		new CircleStroke(geometry, { colour: [255, 0, 0, 255], radius: 10, interactive }),
		new CircleFill(geometry, { colour: [255, 0, 0, 128], radius: 10, interactive }),
	];
}

/**
 * @class EditBar
 * @inherits ButtonGroup
 * @relationship compositionOf ButtonToggle, 0..1, 0..n
 *
 * Offers editing tools - creating new points, lines or polygons (properly symbolized),
 * as well as editing any of those.
 *
 */

export default class EditBar extends ButtonGroup {
	// Current editbar state: idle, creating point, creating line, creating
	// polygon, editing point, editing line, editing polygon.
	#state = IDLE;

	#buttons = [];

	// Symbolizer function for newly created points
	#pointSymbolizer;
	#lineSymbolizer;
	#polygonSymbolizer;
	#nodeSymbolizer;

	// Transient sets of symbols for use in `pointermove` when drawing
	#transientPoint;
	#transientLine;
	#transientPolygon;
	#transientNode;

	#dragNodes = [];

	/// TODO: Transient line, polygon. Same as transient point (part of editor loader)
	/// TODO: Drag points. These are draggabilified circles, and are **NOT** part
	/// of the editor loader. Their drag events do update the transient line
	/// geometry.

	constructor({
		/// @option createPoint: Boolean = true
		/// Whether to show a "Create point" button.
		createPoint = true,

		/// @option createLine: Boolean = true
		/// Whether to show a "Create line" button.
		createLine = true,

		/// @option createPolygon: Boolean = true
		/// Whether to show a "Create polygon" button.
		createPolygon = true,

		/// @option pointSymbolizer: Function = *
		/// Defines how newly created points are symbolized. This must be a
		/// function that takes in a `Geometry` and must return an `Array` of
		/// `GleoSymbol`s.
		pointSymbolizer = defaultPointSymbolizer,

		/// @option pointSymbolizer: Function = *
		/// Idem, but for newly created lines.
		lineSymbolizer = defaultLineSymbolizer,

		/// @option pointSymbolizer: Function = *
		/// Idem, but for newly created polygons.
		polygonSymbolizer = defaultPolygonSymbolizer,

		/// @option nodeSymbolizer: Function = *
		/// Idem, but for drag nodes, i.e. vertices of lines/polygons being edited.
		/// This must be a fucntion that takes *two* parameters. The second one
		/// is a boolean `interactive` flag (it must return interactive symbols
		/// only when this flag is `true`)
		nodeSymbolizer = defaultNodeSymbolizer,

		direction = "vertical",
		...opts
	} = {}) {
		const buttons = [];
		let createPointButton;
		let createLineButton;
		let createPolyButton;

		if (createPoint) {
			createPointButton = new ButtonToggle({
				svgString:
					'<svg width="24" height="24" xmlns="http://www.w3.org/2000/svg"><circle style="fill:none;stroke:#464646;stroke-width:2;stroke-opacity:1" cx="12" cy="12" r="2"/></svg>',
				title: "Create point",
			});

			createPointButton.on("click", (ev) => {
				this.cancel();
				this.#state = NEW_POINT;
				this._map.add(this.#transientPoint);
				createPointButton.setActive(true);
				// this.#transientPoint.geometry = ([NaN, NaN]);
			});
			buttons.push(createPointButton);
		}

		if (createLine) {
			createLineButton = new ButtonToggle({
				svgString: `<svg width="24" height="24" xmlns="http://www.w3.org/2000/svg"><circle style="fill:none;stroke:#464646;stroke-width:2;stroke-opacity:1" cx="3" cy="21" r="2"/><circle style="fill:none;stroke:#464646;stroke-width:2;stroke-opacity:1" cx="8" cy="3" r="2"/><circle style="fill:none;stroke:#464646;stroke-width:2;stroke-opacity:1" cx="21" cy="21" r="2"/><path style="fill:none;stroke:#464646;stroke-width:2;stroke-linecap:butt;stroke-linejoin:miter;stroke-opacity:1" d="m3.5 19 4-14m12 14L9.5 5"/></svg>`,
				title: "Create line",
			});

			createLineButton.on("click", (ev) => {
				this.cancel();
				this.#state = NEW_LINE;
				this.#transientNode.geometry = [NaN, NaN];
				this._map.add(this.#transientNode);
				createLineButton.setActive(true);
			});
			buttons.push(createLineButton);
		}

		if (createPolygon) {
			createPolyButton = new ButtonToggle({
				svgString: `<svg width="24" height="24" xmlns="http://www.w3.org/2000/svg"><ellipse style="fill:none;stroke:#464646;stroke-width:2;stroke-opacity:1" cx="3" cy="21" rx="2" ry="2"/><ellipse style="fill:none;stroke:#464646;stroke-width:2;stroke-opacity:1" cx="8" cy="3" rx="2" ry="2"/><ellipse style="fill:none;stroke:#464646;stroke-width:2;stroke-opacity:1" cx="21" cy="21" rx="2" ry="2"/><path style="fill:none;stroke:#464646;stroke-width:2;stroke-linecap:butt;stroke-linejoin:miter;stroke-dasharray:none;stroke-opacity:1" d="M3.5 19 7.5 5M4.5 21h14.5M19.5 19 9.5 5"/><path style="fill:#464646;fill-opacity:1;stroke:none" d="M5.5 19 9 7.5 17 19Z"/></svg>`,
				title: "Create polygon",
			});

			createPolyButton.on("click", (ev) => {
				this.cancel();
				this.#state = NEW_POLY;
				this.#transientNode.geometry = [NaN, NaN];
				this._map.add(this.#transientNode);
				createPolyButton.setActive(true);
			});
			buttons.push(createPolyButton);
		}

		super({
			direction,
			buttons: buttons,
			...opts,
		});

		this.#buttons = buttons;

		if (createPointButton) {
			createPointButton.ribbons = { Cancel: this.cancel.bind(this) };
		}
		if (createLineButton) {
			createLineButton.ribbons = {
				Finish: this.#finishNewLine.bind(this),
				Cancel: this.cancel.bind(this),
			};
		}
		if (createPolyButton) {
			createPolyButton.ribbons = {
				Finish: this.#finishNewLine.bind(this),
				Cancel: this.cancel.bind(this),
			};
		}

		this.#pointSymbolizer = pointSymbolizer;
		this.#lineSymbolizer = lineSymbolizer;
		this.#polygonSymbolizer = polygonSymbolizer;
		this.#nodeSymbolizer = nodeSymbolizer;
		this.#transientPoint = new MultiSymbol(this.#pointSymbolizer([NaN, NaN]));
		this.#transientLine = new MultiSymbol(this.#lineSymbolizer([NaN, NaN]));
		this.#transientPolygon = new MultiSymbol(this.#polygonSymbolizer([NaN, NaN]));
		this.#transientNode = new MultiSymbol(this.#nodeSymbolizer([NaN, NaN], false));

		this.#boundOnMapClick = this.#onMapClick.bind(this);
		this.#boundOnMapMove = this.#onMapMove.bind(this);
		this.#boundOnNodeClick = this.#onNodeClick.bind(this);
	}

	addTo(map) {
		map.on("click", this.#boundOnMapClick);
		map.on("pointermove", this.#boundOnMapMove);
		super.addTo(map);
	}

	remove() {
		this.cancel();
		return super.remove();
	}

	#boundOnMapClick;
	#boundOnMapMove;
	#boundOnNodeClick;

	#onMapClick(ev) {
		if (this.#state === IDLE) {
			return;
		}

		ev.preventDefault();

		if (this.#state === NEW_POINT) {
			this.cancel();

			const symbols = this.#pointSymbolizer(ev.geometry);
			map.multiAdd(symbols);
			// this.register(ev.geometry, symbols);

			/**
			 * @event createpoint
			 * Fired whenever a new point is created.
			 */
			this.fire("createpoint", { geometry: ev.geometry, symbols: symbols });

			return this;
		}

		if (this.#state === NEW_LINE) {
			const node = new MultiSymbol(this.#nodeSymbolizer(ev.geometry, true));
			node.addTo(this._map).on("click", this.#boundOnNodeClick);
			this.#dragNodes.push(node);
			if (this.#dragNodes.length >= 2) {
				// This is a bit naïve, since geometries from different drag
				// nodes could have different CRSs - but we'll assume the user
				// is not changing the map's CRS during a edit operation.
				const lineGeom = new Geometry(
					this._map.crs,
					this.#dragNodes.map((n) => n.geometry.coords)
				);
				this.#transientLine.geometry = lineGeom;
				this.#transientLine.addTo(this._map);
			}
		}

		if (this.#state === NEW_POLY) {
			// Mostly a copy of NEW_LINE handling
			const node = new MultiSymbol(this.#nodeSymbolizer(ev.geometry, true));
			node.addTo(this._map);
			if (this.#dragNodes.length === 0) {
				// Only the first node shall be interactive, so the polygon closes
				node.on("click", this.#boundOnNodeClick);
			}
			this.#dragNodes.push(node);
			const polyGeom = new Geometry(
				this._map.crs,
				this.#dragNodes.map((n) => n.geometry.coords)
			);
			this.#transientPolygon.geometry = polyGeom;
			this.#transientPolygon.addTo(this._map);
		}
	}

	#onMapMove(ev) {
		if (this.#state === IDLE) {
			return;
		}

		ev.preventDefault();

		if (this.#state === NEW_POINT) {
			this.#transientPoint.geometry = ev.geometry;
		} else if (this.#state === NEW_LINE || this.#state === NEW_POLY) {
			this.#transientNode.geometry = ev.geometry;

			if (this.#dragNodes.length >= 1) {
				const geom = new Geometry(
					this._map.crs,
					this.#dragNodes
						.map((n) => n.geometry.coords)
						.concat([this.#transientNode.geometry.coords])
				);
				if (this.#state === NEW_LINE) {
					this.#transientLine.geometry = geom;
					this.#transientLine.addTo(this._map);
				} else {
					this.#transientPolygon.geometry = geom;
					this.#transientLine.addTo(this._map);
				}
			}
		}
	}

	#onNodeClick(ev) {
		if (this.#state === NEW_LINE || this.#state === NEW_POLY) {
			ev.stopPropagation();
			ev.preventDefault();

			// Set last point of the line geometry to the geometry of the clicked node
			const geom = new Geometry(
				this._map.crs,
				this.#dragNodes
					.map((n) => n.geometry.coords)
					.concat([ev.target.geometry.coords])
			);
			return this.#coalesceNewLine(geom);
		}
	}

	#finishNewLine() {
		// Called from the ribbon buttons - must ignore last transient node,
		// and create a closed geometry when creating a polygon
		let geom;
		if (this.#state === NEW_LINE) {
			geom = new Geometry(
				this._map.crs,
				this.#dragNodes.map((n) => n.geometry.coords)
			);
		} else {
			geom = new Geometry(
				this._map.crs,
				this.#dragNodes.concat(this.#dragNodes[0]).map((n) => n.geometry.coords)
			);
		}
		return this.#coalesceNewLine(geom);
	}

	#coalesceNewLine(geom) {
		if (this.#state === NEW_LINE) {
			this.#transientLine.geometry = geom;
			/**
			 * @event createline
			 * Fired whenever a new line is created.
			 */
			this.fire("createline", {
				geometry: this.#transientLine.geometry,
				symbol: this.#transientLine,
			});

			// Leave the current transient line in the map, and create a new one -
			// effectively sets the previously transient line as "permanent".
			this.#transientLine = new MultiSymbol(this.#lineSymbolizer([NaN, NaN]));
		} else {
			this.#transientPolygon.geometry = geom;
			/**
			 * @event createpolygon
			 * Fired whenever a new polygon is created.
			 */
			this.fire("createpolygon", {
				geometry: this.#transientPolygon.geometry,
				symbol: this.#transientPolygon,
			});

			this.#transientPolygon = new MultiSymbol(this.#polygonSymbolizer([NaN, NaN]));
		}
		return this.cancel();
	}

	/**
	 * @method cancel(): this
	 * Aborts the current editing action. Geometries being drawn will be destroyed.
	 */
	cancel() {
		/// FIXME

		if (this.#state === NEW_POINT) {
			this.#transientPoint.remove();
		}
		if (this.#state === NEW_LINE || this.#state === NEW_POLY) {
			this.#transientNode.remove();
			this.#transientLine.isActive() && this.#transientLine.remove();
			this.#transientPolygon.isActive() && this.#transientPolygon.remove();
			this._map.multiRemove(this.#dragNodes);
			this.#dragNodes = [];
		}

		this.#buttons.forEach((b) => b.setActive(false));

		this.#state = IDLE;
		return this;
	}

	// /**
	//  * method register(geometry: RawGeometry, symbols: Array of GleoSymbol): this
	//  * Registers a geometry, and the symbols it's represented as, as editable.
	//  */
	// register(geometry, symbols) {
	// 	this.#knownSymbols.set(geometry, symbols);
	//
	// 	/// TODO: Add event handlers
	//
	// 	return this;
	// }
}
