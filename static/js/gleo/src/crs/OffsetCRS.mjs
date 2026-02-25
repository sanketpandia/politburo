import BaseCRS from "./BaseCRS.mjs";

/**
 * @class OffsetCRS
 * @inherits BaseCRS
 *
 * Represents a Coordinate Reference System, with the same properties than a
 * `BaseCRS`, but with the `[0,0]` center of coordinates being offset.
 *
 * The rationale is the difference of precision between numbers.
 * A Gleo coordinate is a floating point number, which typically
 * will be represented by:
 * - 8 bytes / 64 bits (`Number`) when defined by the programmer or red by an API
 * - 4 bytes / 32 bits (`Float32Array`) when stored in a WebGL-friendly typed array
 * - 3 bytes / 24 bits (`highp float`) when running inside a GLSL 1.00 shader
 *
 * In order to keep the numerical precision, it's important to keep the numbers low
 * (so the mantissa has a precision of less than one screen pixel).
 *
 * The way to do so is to offset the coordinates. A system based on vector tiles
 * does this implicitly (coordinates inside a typical vector tile range from 0 to 4096,
 * and the CRS coordinate of each tile's corner is the implicit CRS offset).
 *
 * Gleo does this explicitly, translating coordinates to a `offsetCRS` which has a center
 * relatively near the screen center. In other words, when the user pans or zooms the map
 * far enough from the CRS' origin, then Gleo shall establish a new origin near
 * the updated user's viewport, and translate (AKA "offset") all `Geometry`s.
 * This doesn't lose (significant) precision since numbers are originally stored
 * in 64-bit floats (`Number`s).
 *
 */

export default class OffsetCRS extends BaseCRS {
	/**
	 * @section
	 * Build a new offset CRS, given a point `Geometry`. The CRS name, wrap
	 * periods and other `BaseCRS Options` are taken from the `Geometry`'s CRS,
	 * and the offset is the *absolute* value of the `Geometry`'s coordinates.
	 * @constructor OffsetCRS(offset: Geometry)
	 */
	constructor(offset) {
		super(offset.crs.name, {
			wrapPeriodX: offset.crs.wrapPeriodX,
			wrapPeriodY: offset.crs.wrapPeriodY,
			distance: offset.crs.distance,
			flipAxes: offset.crs.flipAxes,
			minSpan: offset.crs.minSpan,
			maxSpan: offset.crs.maxSpan,
			viewableBounds: offset.crs.viewableBounds,
		});

		if (offset.coords.length !== offset.dimension) {
			throw new Error("Offset geometry must be a point");
		}

		this.offset = offset.coords;
		this.offset[0] %= offset.crs.wrapPeriodX;
		this.offset[1] %= offset.crs.wrapPeriodY;

		/// TODO: Decide whether to:
		/// Change this so that the `offset.crs`'s own offset is added to the
		/// offset absolute value - that is, offsets are accumulated.
		///  or
		/// Keep it this way and ensure that all offsets are relative to the base,
		/// i.e. `offsetToBase()` is applied.
	}

	/**
	 * @method offsetToBase(xys: Array of Number): Array of Number
	 * Given a set of coordinates `[x1, y1, x2, y2, ... xn, yn]`, returns
	 * those coordinates as if they were using the `BaseCRS`'s (0,0) origin
	 * of coordinates.
	 */
	offsetToBase(xys) {
		const l = xys.length;
		const out = new Array(l);
		const ox = this.offset[0];
		const oy = this.offset[1];
		for (let i = 0; i < l; i += 2) {
			out[i] = xys[i] + ox;
			out[i + 1] = xys[i + 1] + oy;
		}
		return out;
	}

	/**
	 * @method offsetFromBase(xy: Array of Number): Array of Number
	 * Given a set of coordinates `[x1, y1, x2, y2, ... xn, yn]` in the
	 * `BaseCRS` of this CRS, returns those coordinates as if they were using
	 * the origin of coordinates of this offset CRS.
	 */
	offsetFromBase(xys) {
		const l = xys.length;
		const out = new Array(l);
		const ox = this.offset[0];
		const oy = this.offset[1];
		for (let i = 0; i < l; i += 2) {
			out[i] = xys[i] - ox;
			out[i + 1] = xys[i + 1] - oy;
		}
		return out;
	}
}

/*
 * TODO: store the exponent of the (x,y) offset(s) somehow. There should be some way
 * to map the full exponent resolution of float64s into float24s.
 *
 * In other words, float24 only has 7 bits of exponent, so how to represent things
 * in the order of e.g. 2^-129 (smaller than 2^-128), which can be represented in float64 ?
 *
 * The exponent of the scale delta (i.e. how many CRS units per *one* pixel) should be
 * used here as well.
 *
 * For now, I'll just assume that using numbers with magnitudes smaller than 2^-127
 * or larger than 2^127 is not a foreseeable use case.
 */
