import AcetateSolidExtrusion from "../acetates/AcetateSolidExtrusion.mjs";
import ExtrudedPoint from "./ExtrudedPoint.mjs";
import parseColour from "../3rd-party/css-colour-parser.mjs";

/**
 * @class CircleGauge
 * @inherits CircleStroke
 * @relationship drawnOn AcetateSolidExtrusion
 *
 * A circular percentage gauge. Looks like a `CircleStroke`, but covers less
 * than 360°.
 *
 */

export default class CircleGauge extends ExtrudedPoint {
	/// @section Static properties
	/// @property Acetate: Prototype of AcetateSolidExtrusion
	// The `Acetate` class that draws this symbol.
	static Acetate = AcetateSolidExtrusion;

	#radius;
	#colour;
	#width;
	#percentage;

	/**
	 * @constructor CircleGauge(geom: Geometry, opts?: CircleGauge Options)
	 */
	constructor(
		geom,
		{
			/**
			 * @section
			 * @aka CircleGauge Options
			 * @option radius: Number = 20; Radius of the circle, in CSS pixels
			 * @option colour: Colour = '#3388ff'; The stroke colour
			 * @option width: Number = 2; The width of the stroke, in CSS pixels
			 * @option percentage: Number = 1
			 * The percentage to show, must be a number between 0 (0%) and 1 (100%)
			 */
			radius = 20,
			colour = "#3388ff",
			width = 2,
			percentage = 1,

			...opts
		} = {}
	) {
		super(geom, opts);

		this.#radius = radius;
		this.#colour = this.constructor._parseColour(colour);
		this.#width = width;
		this.#percentage = Math.min(1, Math.max(0, percentage));

		// Length of circumference
		const length = Math.PI * 2 * this.#radius;
		// Divide in triangles so there's a triangle per...
		// 6 pixels of circumference length. That should be enough.
		this.steps = Math.max(7, Math.ceil(length / 6));

		this.opaqueSteps = Math.ceil(this.steps * this.#percentage);

		this.attrLength = (this.opaqueSteps + 1) * 2;
		this.idxLength = this.opaqueSteps * 6;
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
		let vtx = this.attrBase;
		let idx = this.idxBase;
		for (let i = 0; i < this.opaqueSteps; i++) {
			const sinθ = Math.sin(θ);
			const cosθ = Math.cos(θ);

			// Two vertices per step: inner and outer
			// prettier-ignore
			strideExtrusion.set(
				[	sinθ * (ρ - w) + Δx, cosθ * (ρ - w) + Δy,
					sinθ * (ρ + w) + Δx, cosθ * (ρ + w) + Δy, ],
				vtx
			);

			strideColour?.set(this.#colour, vtx);
			strideFeather?.set([-f, f], vtx);
			strideColour?.set(this.#colour, vtx + 1);
			strideFeather?.set([+f, f], vtx + 1);

			// Two triangles per step, forming a quad to the vertices of the
			// next step.
			// prettier-ignore
			typedIdxs?.set([
				vtx+0, vtx+1, vtx+2,
				vtx+2, vtx+1, vtx+3
			], idx);

			θ += ɛ;
			vtx += 2;
			idx += 6;
		}

		// Last pair of vertices
		θ = Math.PI * 2 * this.#percentage;
		const sinθ = Math.sin(θ);
		const cosθ = Math.cos(θ);

		// prettier-ignore
		strideExtrusion.set(
			[	sinθ * (ρ - w) + Δx, cosθ * (ρ - w) + Δy,
				sinθ * (ρ + w) + Δx, cosθ * (ρ + w) + Δy, ],
			vtx
		);

		strideColour?.set(this.#colour, vtx);
		strideFeather?.set([-f, f], vtx);
		strideColour?.set(this.#colour, vtx + 1);
		strideFeather?.set([+f, f], vtx + 1);
	}
	_setStridedExtrusion(strideExtrusion) {
		this._setGlobalStrides(strideExtrusion);
	}

	// Can be overriden by subclasses or the `intensify` decorator
	static _parseColour = parseColour;
}
