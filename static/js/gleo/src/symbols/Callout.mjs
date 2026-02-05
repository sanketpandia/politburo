import AcetateSolidExtrusion from "../acetates/AcetateSolidExtrusion.mjs";
import ExtrudedPoint from "./ExtrudedPoint.mjs";
import parseColour from "../3rd-party/css-colour-parser.mjs";

/**
 * @class Callout
 * @inherits ExtrudedPoint
 * @relationship drawnOn AcetateSolidExtrusion
 *
 * A line segment symbol. One end of the segment is always placed at the symbol's
 * point geometry; the dimensions of the line are defined by the symbol's `offset`
 * (measured in CSS pixels).
 *
 * @example
 * ```js
 * new Callout([0, 0], {
 * 	colour: "red",
 * 	offset: [40, 10],
 * 	width: 4,
 * }).addTo(map);
 * ```
 */

export default class Callout extends ExtrudedPoint {
	/// @section Static properties
	/// @property Acetate: Prototype of AcetateSolidExtrusion
	// The `Acetate` class that draws this symbol.
	static Acetate = AcetateSolidExtrusion;

	#width;
	#colour;

	/**
	 * @constructor Callout(geom: Geometry, opts?: Callout Options)
	 */
	constructor(
		geom,
		{
			/**
			 * @option width: Number = 2; The line segment's width, in CSS pixels
			 * @option colour: Colour = '#3388ff33'; The line segment's colour
			 */
			width = 2,
			colour = "#3388ff",
			...opts
		} = {}
	) {
		super(geom, opts);

		this.#width = width;
		this.#colour = parseColour(colour);

		// A `Callout` is just four vertices in two triangles. One pair of
		// vertices follows the `offset`, while the other doesn't.

		this.attrLength = 4;
		this.idxLength = 6;
	}

	/**
	 * @section Acetate interface
	 * @method _setGlobalStrides(strideExtrusion: StridedTypedArray, strideColour: StridedTypedArray, strideFeather: StridedTypedArray, typedIdxs: TypedArray): undefined
	 * Sets the appropriate values into the strided arrays, based on the
	 * symbol's `attrBase` and `idxBase`.
	 *
	 * Receives the width of the feathering as a parameter, in pixels.
	 */
	_setGlobalStrides(strideExtrusion, strideColour, strideFeather, typedIdxs) {
		const [oX, oY] = this.offset;
		const feather = this._inAcetate.feather;

		// Calculate components of unit vector perpendicular to the offset,
		// create a half-width-length vector from that.
		const l = Math.sqrt(oX * oX + oY * oY);
		const w = (this.#width + feather) / 2;
		const f = w * 256; // Feather max
		const eX = (w * oX) / l;
		const eY = (w * oY) / l;

		// prettier-ignore
		strideExtrusion.set([
			+eY, -eX,
			-eY, eX,
			oX + eY, oY - eX,
			oX - eY, oY + eX
		], this.attrBase);

		const vtx = this.attrBase;
		strideColour?.set(this.#colour, vtx);
		strideFeather?.set([f, f], vtx);
		strideColour?.set(this.#colour, vtx + 1);
		strideFeather?.set([-f, f], vtx + 1);
		strideColour?.set(this.#colour, vtx + 2);
		strideFeather?.set([f, f], vtx + 2);
		strideColour?.set(this.#colour, vtx + 3);
		strideFeather?.set([-f, f], vtx + 3);

		// prettier-ignore
		typedIdxs?.set([
			vtx+ 0, vtx+ 1, vtx+ 2,
			vtx+ 1, vtx+ 2, vtx+ 3
		], this.idxBase);
	}

	_setStridedExtrusion(strideExtrusion) {
		this._setGlobalStrides(
			strideExtrusion,
			undefined,
			undefined,
			undefined,
			this._inAcetate?.feather
		);
	}
}
