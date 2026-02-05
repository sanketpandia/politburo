import AcetateStroke from "../acetates/AcetateStroke.mjs";
import GleoSymbol from "./Symbol.mjs";
import parseColour from "../3rd-party/css-colour-parser.mjs";
import Point from "../3rd-party/point-geometry/point-geometry.mjs";

import { LINEJOIN, LINELOOP, LINECAP } from "../util/pointExtrusionTypeConstants.mjs";

/**
 * @miniclass Join type (Stroke)
 * @section
 * Static property constants that define how the line joins are drawn; use
 * one of these in the `joins` option of the `Stroke` constructor. For context, see
 * [https://developer.mozilla.org/en-US/docs/Web/API/CanvasRenderingContext2D/lineJoin](https://developer.mozilla.org/en-US/docs/Web/API/CanvasRenderingContext2D/lineJoin)
 * @property MITER: Symbol
 * "Miter" joins are sharp, and look bad with very acute angles, but require
 * less vertices and triangles to be drawn.
 * @property BEVEL: Symbol
 * "Bevel" joins look like miters cut in a straight edge. They use points
 * extruded perpendicularly to each segment.
 * @property OUTBEVEL: Symbol
 * "Outer bevel" joins look like bevels, in such a way that a circle of the same
 * diameter as the stroke width, positioned at the intersection of two
 * segments, would be  tangent to each segment and to the bevel edge.
 */

export const MITER = Symbol("MITER");
export const BEVEL = Symbol("BEVEL");
export const OUTBEVEL = Symbol("OUTBEVEL");

/**
 * @miniclass Cap type (Stroke)
 * @section
 * Static property constants that define how the line caps are drawn;
 * use one of these in the `caps` option of the `Stroke` constructor.
 * For context, see [https://developer.mozilla.org/en-US/docs/Web/API/CanvasRenderingContext2D/lineCap](https://developer.mozilla.org/en-US/docs/Web/API/CanvasRenderingContext2D/lineCap)
 *
 * @property BUTT: Symbol
 * "Butt" caps are perpendicular to the first/last segment, and are located
 * exactly at the line's endpoints.
 * @property SQUARE: Symbol
 * "Square" caps are perpendicular to the first/last segment, and are extruded
 * an amount equal to half the stroke's width. In other words: extrudes
 * half a square on each cap.
 * @property HEX: Symbol
 * Short for "hexagon" - extrudes half a hexagon on each cap.
 */

export const BUTT = Symbol("BUTT");
export const SQUARE = Symbol("SQUARE");
export const HEX = Symbol("HEX");

const SQRT3 = Math.sqrt(3);

/**
 * @class Stroke
 * @inherits GleoSymbol
 * @relationship dependsOn AcetateStroke
 *
 * A stroked line, with variable width, colour, dashing, and style of line joins.
 *
 * The `Geometry` used in the constructor might have any depth. If the depth
 * is 1, a single continuous stroke line is created. If it's deeper (e.g.
 * geometries for polygons, multipolylines or multipolygons), then multiple
 * line strokes are created, one per ring.
 */

/*
 Internally represented series of extruded vertices, two per geometry point.
 The extrusion is a function of the stroke width and the angle between
 consecutive points.
*/

export default class Stroke extends GleoSymbol {
	/// @section Static properties
	/// @property Acetate: Prototype of AcetateStroke
	// The `Acetate` class that draws this symbol.
	static Acetate = AcetateStroke;

	#joins;
	static get MITER() {
		return MITER;
	}
	static get BEVEL() {
		return BEVEL;
	}
	static get OUTBEVEL() {
		return OUTBEVEL;
	}

	#caps;
	static get BUTT() {
		return BUTT;
	}
	static get SQUARE() {
		return SQUARE;
	}
	static get HEX() {
		return HEX;
	}

	#colour;
	#width;
	#dashArray;
	#centerline;
	#offset;

	/**
	 * @class Stroke
	 * @constructor Stroke(geom: Geometry, opts?: Stroke Options)
	 */
	constructor(
		geom,
		{
			/**
			 * @section
			 * @aka Stroke Options
			 * @option colour: Colour = '#3388ff'
			 * The colour of the stroke.
			 * @alternative
			 * @option colour: Array of Colour
			 * The colour of each point of the chain. There must be enough elements.
			 */
			colour = "#3388ff",
			/**
			 * @option width: Number = 2
			 * The width of the stroke, in CSS pixels
			 */
			width = 2,

			/**
			 * @option dashArray: undefined = undefined
			 * An undefined (or falsy) value for `dashArray` disables line dashing.
			 * @alternative
			 * @option dashArray: Array of Number
			 * An `Array` of either 2 or 4 `Number`s, defining the line dashing.
			 * Works as per [2D Canvas' `setLineDash`](https://developer.mozilla.org/en-US/docs/Web/API/CanvasRenderingContext2D/setLineDash),
			 * but the array **must** have either **0**, **2** or **4** values.
			 */
			dashArray = undefined,

			/**
			 * @option joins: Join type = Stroke.OUTBEVEL
			 * Defines the shape of line joins. Must be one of `Stroke.MITER`,
			 * `Stroke.BEVEL` or `Stroke.OUTBEVEL`.
			 */
			/// TODO: Implement "tent" joins (4 vertices) and "dome" joins
			/// (5 vertices)
			joins = OUTBEVEL,

			/**
			 * @option caps: Cap type = Stroke.BUTT
			 * Defines the shape of line caps. Must be one of `Stroke.BUTT`,
			 * `Stroke.SQUARE`, or `Stroke.HEX`.
			 */
			/// TODO: Implement more types of line ends
			caps = BUTT,

			/**
			 * @option centerline: Boolean = false
			 * Whether the stroke has vertices along its centerline, or not.
			 */
			centerline = false,

			/**
			 * @option offset: Number = 0
			 * Line offset, in CSS pixels. Positive means to the right of the
			 * line (when going through the geometry from first to last point).
			 * Use this to create line strokes parallel to each other.
			 */
			offset = 0,

			/// TODO: feather,

			interactive = true,

			...opts
		} = {}
	) {
		super(null, { interactive, ...opts });
		this.#joins = joins;
		this.#caps = caps;
		this.#centerline = centerline ? 1 : 0;
		this.#offset = -offset; // Note inverted sign, so offset goes right
		this.geometry = geom; // Calculates storage

		this.#colour = this.constructor._parseColour(colour);
		if (this.#colour === null && Array.isArray(colour)) {
			this.#colour = colour.map(this.constructor._parseColour);
		}

		this.#dashArray = dashArray;

		this.#width = width;

		if (this.#joins === Stroke.MITER) {
			this._fillLineJoin = this._fillLineJoinMiter;
		} else {
			this._fillLineJoin = this._fillLineJoinBevel;
		}

		if (this.#caps === Stroke.BUTT) {
			this._fillLineEnd = this._fillLineEndButt;
		} else if (this.#caps === Stroke.SQUARE) {
			this._fillLineEnd = this._fillLineEndSquare;
		} else {
			this._fillLineEnd = this._fillLineEndHex;
		}

		// Calculations for attrLength and idxLength are offloaded.
		// this.#calcStorage();
	}

	get geometry() {
		return super.geometry;
	}
	set geometry(geom) {
		const ac = this._inAcetate;
		if (ac) {
			ac.remove(this);
			super.geometry = geom;
			this.#calcStorage();
			ac.add(this);
		} else {
			super.geometry = geom;
			this.#calcStorage();
		}
		return this;
	}

	/**
	 * @property dashArray: Array of Number
	 * Runtime value of the `dashArray` constructor option. Can be updated.
	 */
	get dashArray() {
		return this.#dashArray;
	}
	set dashArray(d) {
		this.#dashArray = d;
		this.#updateColourDash();
	}

	/**
	 * @property colour: Colour
	 * Runtime value of the `colour` constructor option. Can be updated.
	 */
	get colour() {
		return this.#colour;
	}
	set colour(c) {
		this.#colour = this.constructor._parseColour(c);
		if (this.#colour === null && Array.isArray(c)) {
			this.#colour = c.map(this.constructor._parseColour);
		}
		this.#updateColourDash();
	}

	/**
	 * @property verticesPerEnd: Number
	 * Read-only getter for the number of line vertices used per line end/cap.
	 */
	get verticesPerEnd() {
		return this.#caps === Stroke.BUTT ? 2 : 4;
	}

	/**
	 * @property trianglesPerEnd: Number
	 * Read-only getter for the number of triangles per line end/cap.
	 */
	get trianglesPerEnd() {
		return this.#caps === Stroke.BUTT ? 0 : this.centerline ? 3 : 2;
	}

	/**
	 * @property verticesPerJoin: Number
	 * Read-only getter for the number of line vertices used per line join.
	 */
	get verticesPerJoin() {
		return this.#joins === Stroke.MITER ? 2 : 3;
	}

	/**
	 * @property centerline: Number
	 * Read-only getter for whether there's vertices in the stroke centerline.
	 */
	get centerline() {
		return this.#centerline ? 1 : 0;
	}

	/**
	 * @property width: Number
	 * Read-only getter for the `stroke` constructor option.
	 */
	get width() {
		return this.#width;
	}

	get joins() {
		return this.#joins;
	}
	get caps() {
		return this.#caps;
	}

	// Calculate amount of vertices/triangles needed.
	#calcStorage() {
		// Assuming two vertices per point
		// Even when line (ring) start and end are copunctual, attributes might
		// be different, specifically line length for the dashing. So, always two
		// vertices per point.

		this.idxLength = 0;
		this.attrLength = 0;
		const center = this.centerline ? 1 : 0;

		// this._verticesPerPoint = new Array(
		// 	this.geometry.coords.length / this.geometry.dimension
		// ).fill(this.verticesPerJoin);

		this.geometry.mapRings((start, end, _length, r) => {
			const pointCount = end - start;
			const segCount = pointCount - 1;
			const joinCount = pointCount - 2;

			// Segment triangles
			this.idxLength += (this.centerline ? 12 : 6) * segCount;

			if (this.geometry.loops[r]) {
				// Join triangles
				this.idxLength += segCount * (this.verticesPerJoin - 2) * 3;

				this.attrLength += (center + this.verticesPerJoin) * segCount;
				this.attrLength += 2 + center;
			} else {
				// Join triangles
				this.idxLength += joinCount * (this.verticesPerJoin - 2) * 3;

				// Line cap triangles
				this.idxLength += this.trianglesPerEnd * 6;

				this.attrLength += (center + this.verticesPerEnd) * 2;
				this.attrLength += (center + this.verticesPerJoin) * joinCount;

				// this._verticesPerPoint[start] = this.verticesPerEnd;
				// this._verticesPerPoint[end] = this.verticesPerEnd;
			}
		});
		// console.log(this._verticesPerPoint);
	}

	// _setGlobalStrides(stridedColour, stridedDash, strideExtrude, strideDistance, coordData) {
	// 	this._setGlobalStridesGeom(strideExtrude, strideDistance, coordData);
	// 	this._setGlobalStrides(stridedColour, stridedDash);
	// }

	// Takes strided arrays for extrusion and distance (zero-indexed), plus
	// this symbol's geometry *projected* to the platina's CRS.
	// _setGlobalStridesGeom(strideExtrude, strideDistance, typedIdxs, geom, miterLimit) {
	_setGeometryStrides(
		geom,
		strideExtrude,
		strideDistance,
		miterLimit,
		perPointStrides,
		typedIdxs
	) {
		let vtx = this.attrBase;
		let idx = this.idxBase;
		const coords = geom.coords;

		// Extrusion width. TODO: feather.
		const width = this.#width / 2;

		// `segments` contains vectors from the n-th point to the n+1-th point.
		// Each vector is represented as a `Point` instance.
		const segments = Array.from(new Array(coords.length / 2 - 1), (_, i) => {
			const offset = i * 2;
			return new Point(
				coords[offset + 2] - coords[offset + 0],
				coords[offset + 3] - coords[offset + 1]
			);
		});

		// `mags` contains the magnitudes of the segments, i.e. the lengths of
		// `the segments
		const mags = segments.map((s) => s.mag());

		// `angles` contains the **heading** angles from the n-th
		// point to the n+1-th point. In radians.
		// const angles = segments.map((s) => s.angle());

		// As `segments`, but with unit vectors
		const units = segments.map((s, i) => s.div(mags[i]));

		// By making an object with the data, we can have a cheap version
		// of pass-by-reference, so that _fillLineEnd and _fillLineJoin can
		// update some members
		const data = {
			idx,
			vtx,
			width,
			segments,
			mags,
			// angles,
			units,
			miterLimit: miterLimit,
			accDistance: 0,
			lastLeftVtx: 0,
			lastRightVtx: 0,
			lastCenterVtx: 0,
			strideExtrude: strideExtrude,
			strideDistance: strideDistance,
			typedIdxs,
			perPointStrides: perPointStrides,
		};

		geom.mapRings((start, end, length, r) => {
			if (length === 1) {
				// Skip degenerate geometries. Can be triggered by applying
				// "stroke" symbols to point geometries, which in turn can
				// be triggered by vector tile stylesheets that don't filter
				// geometries before deciding which symbol to apply.
				return;
			}

			data.accDistance = 0;

			// Is this a closed ring?
			const loop = geom.loops[r];

			for (let i = start; i < end; i++) {
				if (i === start) {
					// First vertex of the stroke
					if (loop) {
						const minSegLength = Math.min(mags[end - 2], mags[start]);
						this._fillLineJoin(
							units[end - 2],
							units[start],
							minSegLength,
							geom,
							data,
							i,
							true
						);
					} else {
						this._fillLineEnd(units[start], data, geom, i, true);
					}
				} else if (i === end - 1) {
					// Last vertex of the stroke
					if (loop) {
						const minSegLength = Math.min(mags[end - 2], mags[start]);
						this._fillLineJoin(
							units[end - 2],
							units[start],
							minSegLength,
							geom,
							data,
							i,
							false
						);
					} else {
						this._fillLineEnd(units[end - 2], data, geom, i, false);
					}
				} else {
					// Minimum segment length (of previous and next), populates aInnerAdjustment.
					const minSegLength = Math.min(mags[i - 1], mags[i]);
					this._fillLineJoin(
						units[i - 1],
						units[i],
						minSegLength,
						geom,
						data,
						i,
						false
					);
				}

				if (i !== end - 1) {
					data.accDistance += mags[i];

					if (this.centerline) {
						// Indices for four triangles between two vertices,
						// using centerline
						// prettier-ignore
						typedIdxs.set([
							data.lastCenterVtx, data.vtx + 0, data.lastLeftVtx,
							data.lastCenterVtx, data.vtx + 1, data.vtx + 0,
							data.lastRightVtx, data.vtx + 1, data.lastCenterVtx,
							data.lastRightVtx, data.vtx + 2, data.vtx + 1,
						], data.idx);

						data.idx += 12;
					} else {
						// Indices for two triangles between two vertices
						// prettier-ignore
						typedIdxs.set( [
							data.lastRightVtx, data.vtx + 1, data.lastLeftVtx,
							data.lastLeftVtx, data.vtx + 1, data.vtx + 0,
						], data.idx );

						data.idx += 6;
					}
				}
			}
		});
	}

	// Runs as part of _setGlobalStrides: sets data for the vertices for a line end
	// on the i-th coordinate of the geometry (and possibly, for the triangles
	// spawned by that line end)
	_fillLineEndButt(heading, data, geom, i) {
		// Fills two vertices with a line butt cap: extrusion perpendicular
		// to the ehading of first/last segment.

		const perp = heading.perp(); // vector perpendicular to heading
		const extrude = perp.mult(data.width);
		const offset = perp.mult(this.#offset);

		// ._add({x: this.#offset, y:0})

		if (this.centerline) {
			this._setPerPointGeomStrides(
				i,
				LINECAP,
				data.vtx,
				3,
				geom,
				...data.perPointStrides
			);

			// prettier-ignore
			data.strideExtrude.set( [
				offset.x + extrude.x, offset.y + extrude.y, data.accDistance, 0, 0,
				offset.x,             offset.y,             data.accDistance, 0, 0,
				offset.x - extrude.x, offset.y - extrude.y, data.accDistance, 0, 0,
			], data.vtx);
			data.lastLeftVtx = data.vtx;
			data.lastCenterVtx = data.vtx + 1;
			data.lastRightVtx = data.vtx + 2;
			data.vtx += 3;
		} else {
			this._setPerPointGeomStrides(
				i,
				LINECAP,
				data.vtx,
				2,
				geom,
				...data.perPointStrides
			);

			// prettier-ignore
			data.strideExtrude.set( [
				offset.x + extrude.x, offset.y + extrude.y, data.accDistance, 0,0,
				offset.x - extrude.x, offset.y - extrude.y, data.accDistance, 0,0,
			], data.vtx );
			data.lastLeftVtx = data.vtx;
			data.lastRightVtx = data.vtx + 1;
			data.vtx += 2;
		}
	}

	_fillLineEndSquare(heading, data, geom, i, first) {
		// Fills *four* vertices with a square cap.

		const perp = heading.perp(); // vector perpendicular to heading
		const extrude = perp.mult(data.width);
		const offset = perp.mult(this.#offset);
		const widthHeading = heading.mult(first ? -data.width : data.width);
		const leftExtrude = widthHeading.add(extrude);
		const rightExtrude = widthHeading._sub(extrude);

		if (this.centerline) {
			this._setPerPointGeomStrides(
				i,
				LINECAP,
				data.vtx,
				5,
				geom,
				...data.perPointStrides
			);

			// prettier-ignore
			data.strideExtrude.set( [
				offset.x + extrude.x,      offset.y + extrude.y, data.accDistance, 0, 0,
				offset.x ,                 offset.y, data.accDistance, 0,0,
				offset.x - extrude.x,      offset.y - extrude.y, data.accDistance, 0, 0,
				offset.x + leftExtrude.x,  offset.y + leftExtrude.y, data.accDistance, 0, 0,
				offset.x + rightExtrude.x, offset.y + rightExtrude.y, data.accDistance, 0, 0,
			], data.vtx);

			if (first) {
				// prettier-ignore
				data.typedIdxs.set([
					data.vtx + 0, data.vtx + 3, data.vtx + 1,
					data.vtx + 1, data.vtx + 3, data.vtx + 4,
					data.vtx + 1, data.vtx + 4, data.vtx + 2,
				], data.idx);
			} else {
				// prettier-ignore
				data.typedIdxs.set([
					data.vtx + 0, data.vtx + 1, data.vtx + 3,
					data.vtx + 1, data.vtx + 4, data.vtx + 3,
					data.vtx + 1, data.vtx + 2, data.vtx + 4,
				], data.idx);
			}
			data.idx += 9;

			data.lastLeftVtx = data.vtx + 0;
			data.lastCenterVtx = data.vtx + 1;
			data.lastRightVtx = data.vtx + 2;
			data.vtx += 5;
		} else {
			this._setPerPointGeomStrides(
				i,
				LINECAP,
				data.vtx,
				4,
				geom,
				...data.perPointStrides
			);

			// prettier-ignore
			data.strideExtrude.set( [
				offset.x + extrude.x,      offset.y + extrude.y, data.accDistance, 0, 0,
				offset.x - extrude.x,      offset.y - extrude.y, data.accDistance, 0, 0,
				offset.x + leftExtrude.x,  offset.y + leftExtrude.y, data.accDistance, 0, 0,
				offset.x + rightExtrude.x, offset.y + rightExtrude.y, data.accDistance, 0, 0,
			], data.vtx );

			if (first) {
				// prettier-ignore
				data.typedIdxs.set([
					data.vtx + 0, data.vtx + 2, data.vtx + 1,
					data.vtx + 1, data.vtx + 2, data.vtx + 3,
				], data.idx);
			} else {
				// prettier-ignore
				data.typedIdxs.set([
					data.vtx + 0, data.vtx + 1, data.vtx + 3,
					data.vtx + 0, data.vtx + 3, data.vtx + 2,
				], data.idx);
			}

			data.idx += 6;
			data.lastLeftVtx = data.vtx + 0;
			data.lastRightVtx = data.vtx + 1;
			data.vtx += 4;
		}
	}

	_fillLineEndHex(heading, data, geom, i, first) {
		// Fills *four* vertices with a half-hexagon cap.

		const hexHeight = data.width * 0.5 * SQRT3;

		const perp = heading.perp(); // vector perpendicular to heading
		const extrude = perp.mult(data.width);
		const offset = perp.mult(this.#offset);
		const halfExtrude = extrude.mult(0.5);
		const widthHeading = heading.mult(first ? -hexHeight : hexHeight);
		const leftExtrude = widthHeading.add(halfExtrude);
		const rightExtrude = widthHeading._sub(halfExtrude);

		// The rest of the method is identical to _fillLineEndSquare

		if (this.centerline) {
			this._setPerPointGeomStrides(
				i,
				LINECAP,
				data.vtx,
				5,
				geom,
				...data.perPointStrides
			);

			// prettier-ignore
			data.strideExtrude.set( [
				offset.x + extrude.x,      offset.y + extrude.y, data.accDistance, 0, 0,
				offset.x ,                 offset.y, data.accDistance, 0,0,
				offset.x - extrude.x,      offset.y - extrude.y, data.accDistance, 0, 0,
				offset.x + leftExtrude.x,  offset.y + leftExtrude.y, data.accDistance, 0, 0,
				offset.x + rightExtrude.x, offset.y + rightExtrude.y, data.accDistance, 0, 0,
			], data.vtx);

			if (first) {
				// prettier-ignore
				data.typedIdxs.set([
					data.vtx + 0, data.vtx + 3, data.vtx + 1,
					data.vtx + 1, data.vtx + 3, data.vtx + 4,
					data.vtx + 1, data.vtx + 4, data.vtx + 2,
				], data.idx);
			} else {
				// prettier-ignore
				data.typedIdxs.set([
					data.vtx + 0, data.vtx + 1, data.vtx + 3,
					data.vtx + 1, data.vtx + 4, data.vtx + 3,
					data.vtx + 1, data.vtx + 2, data.vtx + 4,
				], data.idx);
			}
			data.idx += 9;

			data.lastLeftVtx = data.vtx + 0;
			data.lastCenterVtx = data.vtx + 1;
			data.lastRightVtx = data.vtx + 2;
			data.vtx += 5;
		} else {
			this._setPerPointGeomStrides(
				i,
				LINECAP,
				data.vtx,
				4,
				geom,
				...data.perPointStrides
			);

			// prettier-ignore
			data.strideExtrude.set( [
				offset.x + extrude.x,      offset.y + extrude.y, data.accDistance, 0, 0,
				offset.x - extrude.x,      offset.y - extrude.y, data.accDistance, 0, 0,
				offset.x + leftExtrude.x,  offset.y + leftExtrude.y, data.accDistance, 0, 0,
				offset.x + rightExtrude.x, offset.y + rightExtrude.y, data.accDistance, 0, 0,
			], data.vtx );

			if (first) {
				// prettier-ignore
				data.typedIdxs.set([
					data.vtx + 0, data.vtx + 2, data.vtx + 1,
					data.vtx + 1, data.vtx + 2, data.vtx + 3,
				], data.idx);
			} else {
				// prettier-ignore
				data.typedIdxs.set([
					data.vtx + 0, data.vtx + 1, data.vtx + 3,
					data.vtx + 0, data.vtx + 3, data.vtx + 2,
				], data.idx);
			}

			data.idx += 6;
			data.lastLeftVtx = data.vtx + 0;
			data.lastRightVtx = data.vtx + 1;
			data.vtx += 4;
		}
	}

	// As _fillLineEnd, but for joins. Miter version.
	_fillLineJoinMiter(headingFrom, headingTo, minSegLength, geom, data, i) {
		const prevNormal = headingFrom.perp();
		const nextNormal = headingTo.perp();
		const joinNormal = prevNormal.add(nextNormal);
		if (joinNormal.x !== 0 || joinNormal.y !== 0) {
			joinNormal._unit();
		} else {
			// Degenerate case: 180° angle
			joinNormal.x = prevNormal.x;
			joinNormal.y = prevNormal.y;
		}

		// const cosα = prevNormal.x * nextNormal.x + prevNormal.y * nextNormal.y;
		const cosHalfα = joinNormal.x * nextNormal.x + joinNormal.y * nextNormal.y;
		// joinNormal._div(cosHalfα || 1)._mult(data.width);
		joinNormal._div(cosHalfα || 1);

		const leftOffset = joinNormal.mult(data.width + this.#offset);
		const rightOffset = joinNormal.mult(data.width - this.#offset);

		const isRightTurn = prevNormal.x * nextNormal.y - prevNormal.y * nextNormal.x < 0;

		// Put min segment length and extrusion ratio in either left or right
		// aInnerAdjustment.
		const leftAdj1 = isRightTurn ? 0 : minSegLength;
		const leftAdj2 = isRightTurn ? 0 : 1 / cosHalfα;
		const rightAdj1 = isRightTurn ? minSegLength : 0;
		const rightAdj2 = isRightTurn ? 1 / cosHalfα : 0;

		if (this.centerline) {
			this._setPerPointGeomStrides(
				i,
				LINEJOIN,
				data.vtx,
				3,
				geom,
				...data.perPointStrides
			);

			// prettier-ignore
			data.strideExtrude.set([
				leftOffset.x, leftOffset.y, data.accDistance, leftAdj1, leftAdj2,
				0, 0, data.accDistance, 0,0,
				-rightOffset.x, -rightOffset.y, data.accDistance, rightAdj1, rightAdj2,
			], data.vtx);

			data.lastLeftVtx = data.vtx;
			data.lastCenterVtx = data.vtx + 1;
			data.lastRightVtx = data.vtx + 2;

			data.vtx += 3;
		} else {
			this._setPerPointGeomStrides(
				i,
				LINEJOIN,
				data.vtx,
				2,
				geom,
				...data.perPointStrides
			);

			// prettier-ignore
			data.strideExtrude.set([
				leftOffset.x, leftOffset.y, data.accDistance, leftAdj1,leftAdj2,
				-rightOffset.x, -rightOffset.y, data.accDistance, rightAdj1, rightAdj2,
			], data.vtx);

			data.lastLeftVtx = data.vtx;
			data.lastRightVtx = data.vtx + 1;

			data.vtx += 2;
		}
	}

	// As _fillLineEnd, but for joins. Bevel version.
	// Splits code path into left- and right-turns, since left/right extrusion is
	// different between each. Order of vertices is always left-(center)-right-to:
	// bevel is formed between left-right-to without centerline vertex, or
	// left-center-to / right-center-to with centerline vertex.
	_fillLineJoinBevel(headingFrom, headingTo, minSegLength, geom, data, i, first) {
		const prevNormal = headingFrom.perp();
		const nextNormal = headingTo.perp();
		const joinNormal = prevNormal.add(nextNormal);
		if (joinNormal.x !== 0 || joinNormal.y !== 0) {
			joinNormal._unit();
		} else {
			// Degenerate case: 180° angle
			joinNormal.x = prevNormal.x;
			joinNormal.y = prevNormal.y;
		}

		// const cosα = prevNormal.x * nextNormal.x + prevNormal.y * nextNormal.y;
		const cosHalfα = joinNormal.x * nextNormal.x + joinNormal.y * nextNormal.y;
		// const sinHalfα = Math.sqrt(1 - cosHalfα * cosHalfα);
		// joinNormal._div(cosHalfα || 1)._mult(data.width);
		joinNormal._div(cosHalfα || 1);
		const joinNormalZeroOffset = joinNormal.mult(data.width);

		const deltaAngle =
			(Math.PI * 2 + headingTo.angle() - headingFrom.angle()) % (Math.PI * 2);

		// The tangent of a quarter of the angle (or half the angle between
		// join normal and prev/next normal), needed for outer bevels
		const tgQuarterα = Math.tan(deltaAngle / 4);

		const isRightTurn = prevNormal.x * nextNormal.y - prevNormal.y * nextNormal.x < 0;

		let centerVertex = this.centerline ? [0, 0, data.accDistance, 0, 0] : [];
		const firstOffset = first ? 0 : 1;

		this._setPerPointGeomStrides(
			i,
			first /* && isRightTurn*/ ? LINELOOP : LINEJOIN, // FIXME: left turns should work
			data.vtx,
			(this.centerline ? 3 : 2) + firstOffset,
			geom,
			...data.perPointStrides
		);

		if (isRightTurn) {
			if (this.#joins === Stroke.OUTBEVEL) {
				// Outer bevel offset:
				prevNormal._add(headingFrom.div(tgQuarterα));
				nextNormal._sub(headingTo.div(tgQuarterα));
			}

			joinNormal._mult(data.width - this.#offset);
			const joinNormalDelta = joinNormal.sub(joinNormalZeroOffset);
			prevNormal._mult(data.width)._sub(joinNormalDelta);
			nextNormal._mult(data.width)._sub(joinNormalDelta);

			const prevVertex = first
				? []
				: [prevNormal.x, prevNormal.y, data.accDistance, 0, 0];

			// prettier-ignore
			data.strideExtrude.set([
				// Left vertex, from previous segment
				...prevVertex,

				// Center vertex
				...centerVertex,

				// Right vertex, common to segments
				-joinNormal.x, -joinNormal.y, data.accDistance, minSegLength, 1 / cosHalfα,

				// Left vertex, to next segment
				nextNormal.x, nextNormal.y , data.accDistance, 0, 0,
			], data.vtx);

			if (!first) {
				data.typedIdxs.set(
					[data.vtx, data.vtx + 1, data.vtx + this.centerline + 2],
					data.idx
				);
				data.idx += 3;
			}

			data.lastRightVtx = data.vtx + firstOffset + this.centerline;
			data.lastLeftVtx = data.lastRightVtx + 1;
			data.lastCenterVtx = data.vtx + firstOffset;
		} else {
			if (this.#joins === Stroke.OUTBEVEL) {
				// Outer bevel offset:
				prevNormal._sub(headingFrom.mult(tgQuarterα));
				nextNormal._add(headingTo.mult(tgQuarterα));
			}

			joinNormal._mult(data.width + this.#offset);
			const joinNormalDelta = joinNormal.sub(joinNormalZeroOffset);
			prevNormal._mult(data.width)._sub(joinNormalDelta);
			nextNormal._mult(data.width)._sub(joinNormalDelta);

			const prevVertex = first
				? []
				: [-prevNormal.x, -prevNormal.y, data.accDistance, 0, 0];

			// prettier-ignore
			data.strideExtrude.set([
				// Left vertex, common to segments
				joinNormal.x, joinNormal.y, data.accDistance, minSegLength, 1 / cosHalfα,

				// Center vertex
				...centerVertex,

				// Right vertex, coming from previous segment
				...prevVertex,

				// Right vertex, to next segment
				-nextNormal.x, -nextNormal.y , data.accDistance, 0, 0,
			], data.vtx);

			if (!first) {
				data.typedIdxs.set(
					[
						data.vtx + this.centerline,
						data.vtx + this.centerline + 1,
						data.vtx + this.centerline + 2,
					],
					data.idx
				);
				data.idx += 3;
			}
			data.lastRightVtx = data.vtx + 1 + this.centerline + firstOffset;
			data.lastCenterVtx = data.vtx + 1;
			data.lastLeftVtx = data.vtx;
		}
		data.vtx += 2 + this.centerline + firstOffset;
	}

	#updateColourDash() {
		if (!this._inAcetate) {
			return;
		}

		const stridedArrays = this._inAcetate._getStridedArrays(
			this.attrBase + this.attrLength,
			this.idxBase + this.idxLength
		);
		this._setGlobalStrides(...stridedArrays);

		// this._inAcetate._attrs.commit(this.attrBase, this.attrLength);
		this._inAcetate._commitStridedArrays(
			this.attrBase,
			this.attrLength,
			this.idxBase,
			this.idxLength
		);
		this._inAcetate.dirty = true;
	}

	_setGlobalStrides(strideDash, perPointStrides) {
		// Normalize dasharray into an accumulated 4-element array.
		let dashArray;
		if (!this.dashArray || this.dashArray.length === 0) {
			dashArray = Uint8Array.from([1, 1, 1, 1]);
		} else if (this.dashArray.length === 2) {
			const [d0, d1] = this.dashArray;
			// dashArray = Uint8Array.from([d0, d1 + d0, d0 + d1 + d0, d1 + d0 + d1 + d0]);
			dashArray = Uint8Array.from([d0, d1 + d0, 0, d1 + d0]);
		} else if (this.dashArray.length === 4) {
			const [d0, d1, d2, d3] = this.dashArray;
			dashArray = Uint8Array.from([d0, d1 + d0, d2 + d1 + d0, d3 + d2 + d1 + d0]);
		} else {
			throw new Error("Invalid length of dashArray in stroke.");
		}

		for (let i = this.attrBase, end = this.attrBase + this.attrLength; i < end; i++) {
			strideDash.set(dashArray, i);
		}

		/// TODO: get per point strides

		let vtx = this.attrBase;
		const vtxJoin = this.verticesPerJoin + (this.centerline ? 1 : 0);
		const vtxEnd = this.verticesPerEnd + (this.centerline ? 1 : 0);
		const isBevel = this.#joins === Stroke.BEVEL || this.#joins === Stroke.OUTBEVEL;

		this.geometry.mapRings((start, end, length, r) => {
			if (length === 1) {
				// Skip degenerate geometries.
				return;
			}

			for (let i = start; i < end; i++) {
				if (i === start || i === end - 1) {
					// First or last vertex of the stroke
					if (this.geometry.loops[r]) {
						this._setPerPointStrides(
							i,
							LINEJOIN,
							vtx,
							vtxJoin - (isBevel && i === start ? 1 : 0),
							...perPointStrides
						);
						vtx += vtxJoin - (isBevel && i === start ? 1 : 0);
					} else {
						this._setPerPointStrides(
							i,
							LINECAP,
							vtx,
							vtxEnd,
							...perPointStrides
						);
						vtx += vtxEnd;
					}
				} else {
					this._setPerPointStrides(
						i,
						LINEJOIN,
						vtx,
						vtxJoin,
						...perPointStrides
					);
					vtx += vtxJoin;
				}
			}
		});
	}

	/**
	 * @section Acetate Interface
	 * @uninheritable
	 * @method _setPerPointStrides(n: Number, pointType: Symbol, vtx: Number, vtxCount: Number ...): this
	 * As `_setGlobalStrides`, but only affects the n-th point in the symbol's
	 * geometry.
	 *
	 * Takes the following parameters:
	 * - Index for the `n`th point in the geometry
	 * - Type of point extrusion: line join, line cap, or bevel-less loop line join
	 * - Index for the vertex attribute data
	 * - Number of vertices spawned for this geometry point
	 * - strided arrays, as per `_getPerPointStridedArrays`.
	 *
	 * This method can be overriden or extended by subclasses and/or decorators.
	 */
	_setPerPointStrides(n, _pointType, vtx, vtxCount, strideColour, ..._strides) {
		const pointColour =
			Array.isArray(this.#colour) && Array.isArray(this.#colour[0])
				? this.#colour[n]
				: this.#colour;

		for (let i = 0; i < vtxCount; i++) {
			strideColour?.set(pointColour, vtx + i);
		}
	}

	/**
	 * @section Acetate Interface
	 * @uninheritable
	 * @method _setPerPointGeomStrides(n: Number, pointType: Symbol, vtx: Number, geom: Geometry, vtxCount: Number ...): this
	 * As `_setPerPointStrides`, but also takes the projected geometry. This gets
	 * run whenever the grometry is reprojected, as per `_setGeometryStrides`.
	 */
	_setPerPointGeomStrides(_n, _pointType, _vtx, _vtxCount, _geom, ..._strides) {
		// noop
	}

	// Can be overriden by subclasses or the `intensify` decorator
	static _parseColour = parseColour;
}
