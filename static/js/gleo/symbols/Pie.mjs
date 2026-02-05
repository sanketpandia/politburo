import AcetateSolidExtrusion from "../acetates/AcetateSolidExtrusion.mjs";
import ExtrudedPoint from "./ExtrudedPoint.mjs";
import parseColour from "../3rd-party/css-colour-parser.mjs";

/**
 * @class Pie
 * @inherits ExtrudedPoint
 * @relationship drawnOn AcetateSolidExtrusion
 *
 * A circular pie chart, displayed at a constant screen ratio.
 *
 * @example
 * ```
 * let pie = new Pie(geometry, {
 * 	radius: 40,
 * 	slices: {
 * 		red: 10,
 * 		green: 15,
 * 		blue: 8,
 * 		pink: 11,
 * 		cyan: 12,
 * 		black: 13,
 * 	},
 * }
 * ```
 */

const TAU = Math.PI * 2;

export default class Pie extends ExtrudedPoint {
	/// @section Static properties
	/// @property Acetate: Prototype of AcetateSolidExtrusion
	// The `Acetate` class that draws this symbol.
	static Acetate = AcetateSolidExtrusion;

	/**
	 * @constructor Pie(geom: Geometry, opts?: Pie Options)
	 */
	constructor(geom, { radius = 20, slices, resolution = 0.4, ...opts } = {}) {
		super(geom, opts);

		/**
		 * @section
		 * @aka Pie Options
		 *
		 * @option radius: Number = 20
		 * Radius of the pie chart, in CSS pixels
		 *
		 * @option slices: Object of Colour to Number
		 * The data for the pie chart slices. Keys must be `Colour`s, values must be
		 * `Number`s.The size of the pie chart's slices will be directly proportional
		 * to the data value.
		 */
		this.#radius = radius;
		this.#slices = slices;
		const sliceCount = Object.keys(slices).length;

		this.#valueSum = Object.values(slices).reduce((acc, curr) => acc + curr, 0);

		// Preliminary steps calculation, just to estimate the amount of
		// vertices/triangles needed.
		const length = TAU * this.#radius;
		const steps = Math.max(6, Math.ceil(length / 6));
		this.#epsilonValue = this.#valueSum / steps;

		// NOTE: This is an upper bound - assuming all and each slice needs three,
		// and not two, extra vertices. That's center, first vertex of arc, and
		// extra arc vertex due to `ceil()`ing steps calculations.
		this.attrLength = steps + sliceCount * 3;
		this.idxLength = (steps + sliceCount) * 3;
	}
	#radius;
	#slices;
	#valueSum;
	#epsilonValue;

	/**
	 * @section Acetate interface
	 * @method _setGlobalStrides(strideExtrusion: StridedTypedArray, strideColour: StridedTypedArray, strideFeather: StridedTypedArray, typedIdxs: TypedArray): undefined
	 * Sets the appropriate values into the strided arrays, based on the
	 * symbol's `attrBase` and `idxBase`.
	 *
	 * Receives the width of the feathering as a parameter, in pixels.
	 */
	_setGlobalStrides(strideExtrusion, strideColour, strideFeather, typedIdxs) {
		const feather = this._inAcetate.feather;
		const ρ = this.#radius + feather / 2;
		const f = ρ * 256; // Feather max
		const [offsetX, offsetY] = this.offset;

		let acc = 0;
		let radiansPerUnit = TAU / this.#valueSum;
		let vtx = this.attrBase;
		let idx = this.idxBase;

		Object.entries(this.#slices).forEach(([rawColour, amount]) => {
			// Number of triangle divisions in this slice
			const steps = Math.ceil(amount / this.#epsilonValue);

			// Increment of value each triangle division
			const ɛ = amount / steps;

			const colour = parseColour(rawColour);

			// Start and end angles of the slice arc
			let θ = acc * radiansPerUnit;

			// Center vertex
			strideExtrusion.set([offsetX, offsetY], vtx);
			strideColour?.set(colour, vtx);
			strideFeather?.set([0, f], vtx);
			const centerVtx = vtx;
			vtx++;

			// First vertex of the arc
			strideExtrusion.set(
				[Math.sin(θ) * ρ + offsetX, Math.cos(θ) * ρ + offsetY],
				vtx
			);
			strideColour?.set(colour, vtx);
			strideFeather?.set([f, f], vtx);
			vtx++;

			for (let i = 0; i < steps; i++) {
				// Rest of vertices of the arc

				typedIdxs?.set([centerVtx, vtx - 1, vtx], idx);

				acc += ɛ;
				θ = acc * radiansPerUnit;

				strideExtrusion.set(
					[Math.sin(θ) * ρ + offsetX, Math.cos(θ) * ρ + offsetY],
					vtx
				);
				strideColour?.set(colour, vtx);
				strideFeather?.set([f, f], vtx);
				vtx++;
				idx += 3;
			}
		});

		// Since this.#idxLength is a upper bound of the needed amount of
		// triangle vertices, there might be an unused gap in the indexBuffer.
		// This is set to NaN to avoid stale references from being used.
		let gap = this.idxBase + this.idxLength - idx;
		if (gap > 0) {
			typedIdxs?.set(new Array(gap).fill(0), idx);
			// typedIdxs?.set(new Array(gap).fill(NaN), idx);
		}
	}

	_setStridedExtrusion(strideExtrusion) {
		this._setGlobalStrides(strideExtrusion);
	}
}
