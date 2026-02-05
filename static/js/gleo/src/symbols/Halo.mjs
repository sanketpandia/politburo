import AcetateSolidExtrusion from "../acetates/AcetateSolidExtrusion.mjs";
import ExtrudedPoint from "./ExtrudedPoint.mjs";
import parseColour from "../3rd-party/css-colour-parser.mjs";

/**
 * @class Halo
 * @inherits ExtrudedPoint
 * @relationship drawnOn AcetateSolidExtrusion
 *
 * A `Halo` is a circular symbol with multiple colour stops in a radial gradient,
 * each colour stop having a different radius (specified in CSS pixels).
 *
 * @example
 * ```
 * let halo = new Halo(geometry, {
 *   stops: {
 *     90: [255,0,0,0],
 *     100: [255,0,0,255],
 *     110: [255,0,0,0]
 *   }
 * });
 * ```
 *
 */

export default class Halo extends ExtrudedPoint {
	/// @section Static properties
	/// @property Acetate: Prototype of AcetateSolidExtrusion
	// The `Acetate` class that draws this symbol.
	static Acetate = AcetateSolidExtrusion;

	#radii = [];
	#colours = [];
	#stopsCount = 0;
	#width;

	/**
	 * @constructor Halo(geom: Geometry, opts?: Halo Options)
	 */
	constructor(
		geom,
		{
			/**
			 * @section
			 * @aka Halo Options
			 * @option stops: Object of Number to Colour = {}
			 * A key-value map of radii to `Colour`s
			 */
			stops = {},
			...opts
		} = {}
	) {
		super(geom, opts);

		let maxRadius = 0;
		let minRadius = Infinity;
		for (let [radius, colour] of Object.entries(stops)) {
			const r = Number(radius);
			this.#radii.push(r);
			this.#colours.push(this.constructor._parseColour(colour));
			maxRadius = Math.max(maxRadius, r);
			minRadius = Math.min(minRadius, r);
			this.#stopsCount++;
		}

		if (this.#stopsCount < 2) {
			throw new Error(`A Halo needs at least two colour stops`);
		}

		this.#width = maxRadius - minRadius;

		// Length of circumference
		const length = Math.PI * 2 * maxRadius;
		// Divide in triangles so there's a triangle per...
		// 6 pixels of circumference length. That should be enough.
		this.steps = Math.max(6, Math.ceil(length / 6));

		this.attrLength = this.steps * this.#stopsCount;
		this.idxLength = this.steps * 6 * (this.#stopsCount - 1);
	}

	// Can be overriden by subclasses or the `intensify` decorator
	static _parseColour = parseColour;

	/**
	 * @section Acetate interface
	 * @method _setGlobalStrides(strideExtrusion: StridedTypedArray, strideColour: StridedTypedArray, strideFeather: StridedTypedArray, typedIdxs: TypedArray): undefined
	 * Sets the appropriate values into the strided arrays, based on the
	 * symbol's `attrBase` and `idxBase`.
	 *
	 * Receives the width of the feathering as a parameter, in pixels.
	 */
	_setGlobalStrides(strideExtrusion, strideColour, strideFeather, typedIdxs) {
		const ɛ = (Math.PI * 2) / this.steps;

		const ρmid = this.#radii[0] + this.#width / 2;
		const w = this.#width / 2;
		const f = w * 256; // Feather max
		const [Δx, Δy] = this.offset;

		let θ = 0;
		const c = this.#stopsCount;
		const steps2 = this.steps * c;
		let vtx = this.attrBase;
		let idx = this.idxBase;
		for (let i = 0; i < steps2; i += c) {
			const sinθ = Math.sin(θ);
			const cosθ = Math.cos(θ);

			for (let j = 1; j < c; j++) {
				if (i !== steps2 - c) {
					// prettier-ignore
					typedIdxs?.set([
						vtx+j-1, vtx+j, vtx+j+c-1,
						vtx+j, vtx+j+c-1, vtx+j+c,
					], idx);
				} else {
					// prettier-ignore
					typedIdxs?.set(
						[
							vtx + j - 1, vtx + j, this.attrBase + j - 1,
							vtx + j, this.attrBase + j - 1, this.attrBase + j,
						],
						idx
					);
				}
				idx += 6;
			}

			for (let j = 0; j < this.#stopsCount; j++) {
				const ρ = this.#radii[j];
				strideExtrusion.set([sinθ * ρ + Δx, cosθ * ρ + Δy], vtx);
				strideColour?.set(this.#colours[j], vtx);
				strideFeather?.set([(ρ - ρmid) / w, f], vtx);
				vtx++;
			}

			θ += ɛ;
		}
	}

	_setStridedExtrusion(strideExtrusion) {
		this._setGlobalStrides(strideExtrusion);
	}
}
