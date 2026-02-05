import AcetateSolidExtrusion from "../acetates/AcetateSolidExtrusion.mjs";
import ExtrudedPoint from "./ExtrudedPoint.mjs";
import parseColour from "../3rd-party/css-colour-parser.mjs";

/**
 * @class CircleStroke
 * @inherits ExtrudedPoint
 * @relationship drawnOn AcetateSolidExtrusion
 *
 * The "stroke" part of a circle symbol - a line of constant width
 * (in CSS pixels), going around the circumference of a circle with its center in
 * the given `Geometry`.
 *
 * @example
 * ```js
 * new CircleStroke([0, 0], {
 * 	colour: "red",
 * 	radius: 40,
 * 	width: 3
 * }).addTo(map);
 * ```
 */

export default class CircleStroke extends ExtrudedPoint {
	/// @section Static properties
	/// @property Acetate: Prototype of AcetateSolidExtrusion
	// The `Acetate` class that draws this symbol.
	static Acetate = AcetateSolidExtrusion;

	#radius;
	#colour;
	#width;

	/**
	 * @constructor CircleStroke(geom: Geometry, opts?: CircleStroke Options)
	 */
	constructor(
		geom,
		{
			/**
			 * @section
			 * @aka CircleStroke Options
			 * @option radius: Number = 20; Radius of the circle, in CSS pixels
			 * @option colour: Colour = '#3388ff'; The stroke colour
			 * @option width: Number = 2; The width of the stroke, in CSS pixels
			 */
			radius = 20,
			colour = "#3388ff",
			width = 2,

			...opts
		} = {}
	) {
		super(geom, opts);

		this.#radius = radius;
		this.#colour = this.constructor._parseColour(colour);
		this.#width = width;

		// Length of circumference
		const length = Math.PI * 2 * this.#radius;
		// Divide in triangles so there's a triangle per...
		// 6 pixels of circumference length. That should be enough.
		this.steps = Math.max(7, Math.ceil(length / 6));

		this.attrLength = this.steps * 2;
		this.idxLength = this.steps * 6;
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
		// Radian increment per step
		const ɛ = (Math.PI * 2) / this.steps;

		const ρ = this.#radius;
		const w = (this.#width + this._inAcetate.feather) / 2;
		const f = w * 256; // Feather max
		const [Δx, Δy] = this.offset;

		let θ = 0;
		const steps2 = this.steps * 2;
		let vtx = this.attrBase;
		let idx = this.idxBase;
		for (let i = 0; i < steps2; i += 2) {
			const sinθ = Math.sin(θ);
			const cosθ = Math.cos(θ);

			// Two vertices per step: inner and outer
			strideExtrusion.set(
				[
					sinθ * (ρ - w) + Δx,
					cosθ * (ρ - w) + Δy,
					sinθ * (ρ + w) + Δx,
					cosθ * (ρ + w) + Δy,
				],
				vtx
			);

			strideColour?.set(this.#colour, vtx);
			strideFeather?.set([-f, f], vtx);
			strideColour?.set(this.#colour, vtx + 1);
			strideFeather?.set([+f, f], vtx + 1);

			// Two triangles per step, forming a quad to the vertices of the
			// next step.
			if (i !== steps2 - 2) {
				// prettier-ignore
				typedIdxs?.set([
					vtx+0, vtx+1, vtx+2,
					vtx+2, vtx+1, vtx+3
				], idx);
			} else {
				// prettier-ignore
				typedIdxs?.set([
					vtx, vtx+1, this.attrBase,
					this.attrBase, vtx+1, this.attrBase + 1
				], idx);
			}

			θ += ɛ;
			vtx += 2;
			idx += 6;
		}
	}
	_setStridedExtrusion(strideExtrusion) {
		this._setGlobalStrides(strideExtrusion);
	}

	// Can be overriden by subclasses or the `intensify` decorator
	static _parseColour = parseColour;
}
