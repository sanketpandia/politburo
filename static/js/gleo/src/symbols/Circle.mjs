import ExtrudedPoint from "./ExtrudedPoint.mjs";
import parseColour from "../3rd-party/css-colour-parser.mjs";
import AcetateSolidBorder from "../acetates/AcetateSolidBorder.mjs";

/**
 * @class Circle
 * @inherits ExtrudedPoint
 * @relationship drawnOn AcetateSolidBorder
 *
 * A circle with both fill and stroke (i.e. perimeter line).
 *
 * Renders with a different `Acetate` than the simpler `CircleFill` and
 * `CircleStroke`.
 *
 * @example
 * ```js
 * new Circle([0, 0], {
 * 	fillColour: "red",
 * 	strokeColour: "black",
 * 	width: 3,
 * 	radius: 40
 * }).addTo(map);
 * ```
 */

export default class Circle extends ExtrudedPoint {
	/// @section Static properties
	/// @property Acetate: Prototype of AcetateSolidExtrusion
	// The `Acetate` class that draws this symbol.
	static Acetate = AcetateSolidBorder;

	#radius;
	#width;
	#fillColour;
	#strokeColour;
	#feather;

	/**
	 * @constructor CircleFill(geom: Geometry, opts?: CircleFill Options)
	 */
	constructor(
		geom,
		{
			/**
			 * @section
			 * @aka Circle Options
			 * @option radius: Number = 20; Radius of the circle, in CSS pixels
			 * @option width: Number = 4; Width of the border, in CSS pixels
			 * @option fillColour: Colour = '#3388ff33'; The fill colour
			 * @option strokeColour: Colour = '#3388ff33'; The border stroke colour
			 */
			radius = 20,
			width = 4,
			fillColour = "#3388ff33",
			strokeColour = "#3388ff",
			/**
			 * @option feather: Number = 0.5
			 * The width of the antialiasing feather, in CSS pixels.
			 */
			feather = 0.5,

			...opts
		} = {}
	) {
		super(geom, opts);

		this.#radius = radius;
		this.#width = width * 2;
		this.#fillColour = this.constructor._parseColour(fillColour);
		this.#strokeColour = this.constructor._parseColour(strokeColour);
		this.#feather = feather;

		// Length of circumference
		const length = Math.PI * 2 * this.#radius;
		// Divide in triangles so there's a triangle per...
		// 6 pixels of circumference length. That should be enough.
		this.steps = Math.max(7, Math.ceil(length / 6));
		// this.steps = 4;

		this.attrLength = this.steps + 1;
		this.idxLength = this.steps * 3;
	}

	/**
	 * @section Acetate interface
	 * @method _setGlobalStrides(strideExtrusion: StridedTypedArray, strideColour: StridedTypedArray, strideFeather: StridedTypedArray, typedIdxs: TypedArray): undefined
	 * Sets the appropriate values into the strided arrays, based on the
	 * symbol's `attrBase` and `idxBase`.
	 *
	 * Receives the width of the feathering as a parameter, in pixels.
	 */
	_setGlobalStrides(
		strideExtrusion,
		strideFillColour,
		strideBorderColour,
		strideBorder,
		strideEdgeDistance,
		typedIdxs
	) {
		// const feather = this._inAcetate.feather;

		// Radian increment per step
		const ɛ = (Math.PI * 2) / this.steps;

		const ρ = this.#radius + this.#feather / 2;
		const [Δx, Δy] = this.offset;

		// Attributes start with the center point
		strideExtrusion.set([Δx, Δy], this.attrBase);
		strideFillColour?.set(this.#fillColour, this.attrBase);
		strideBorderColour?.set(this.#strokeColour, this.attrBase);
		strideEdgeDistance.set([this.#radius, this.#radius, this.#radius], this.attrBase);

		let θ = 0;
		let vtx = this.attrBase + 1;
		let idx = this.idxBase;
		for (let i = 0; i < this.steps; i++) {
			strideExtrusion.set([Math.sin(θ) * ρ + Δx, Math.cos(θ) * ρ + Δy], vtx);
			strideFillColour?.set(this.#fillColour, vtx);
			strideBorderColour?.set(this.#strokeColour, vtx);

			// Vertices of the i-th triangle are: center, current, next
			if (i !== this.steps - 1) {
				typedIdxs?.set([this.attrBase, vtx, vtx + 1], idx);
			} else {
				typedIdxs?.set([this.attrBase, vtx, this.attrBase + 1], idx);
			}

			strideEdgeDistance.set([0, 0, 0], vtx);
			strideBorder.set([this.#width, this.#feather], vtx);

			θ += ɛ;
			vtx++;
			idx += 3;
		}
	}

	_setStridedExtrusion(strideExtrusion) {
		this._setGlobalStrides(strideExtrusion);
	}

	// Can be overriden by subclasses or the `intensify` decorator
	static _parseColour = parseColour;
}
