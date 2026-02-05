import AcetateSolidExtrusion from "../acetates/AcetateSolidExtrusion.mjs";
import ExtrudedPoint from "./ExtrudedPoint.mjs";
import parseColour from "../3rd-party/css-colour-parser.mjs";

/**
 * @class CircleFill
 * @inherits ExtrudedPoint
 * @relationship drawnOn AcetateSolidExtrusion
 *
 * The "fill" part of a circle symbol - a circle of constant radius
 * (measured in CSS pixels), spawning from a point `Geometry` in the circle center.
 *
 * @example
 * ```js
 * new CircleFill([0, 0], {
 * 	colour: "red",
 * 	radius: 40
 * }).addTo(map);
 * ```
 */

export default class CircleFill extends ExtrudedPoint {
	/// @section Static properties
	/// @property Acetate: Prototype of AcetateSolidExtrusion
	// The `Acetate` class that draws this symbol.
	static Acetate = AcetateSolidExtrusion;

	#radius;
	#colour;

	/**
	 * @constructor CircleFill(geom: Geometry, opts?: CircleFill Options)
	 */
	constructor(
		geom,
		{
			/**
			 * @section
			 * @aka CircleFill Options
			 * @option radius: Number = 20; Radius of the circle, in CSS pixels
			 * @option colour: Colour = '#3388ff33'; The fill colour
			 */
			radius = 20,

			colour = "#3388ff33",

			...opts
		} = {}
	) {
		super(geom, opts);

		this.#radius = radius;
		this.#colour = this.constructor._parseColour(colour);

		// Length of circumference
		const length = Math.PI * 2 * this.#radius;
		// Divide in triangles so there's a triangle per...
		// 6 pixels of circumference length. That should be enough.
		this.steps = Math.max(7, Math.ceil(length / 6));

		this.attrLength = this.steps + 1;
		this.idxLength = this.steps * 3;
	}

	/**
	 * @property colour
	 * The colour of this `CircleFill`. Can be updated.
	 */
	get colour() {
		return this.#colour;
	}

	set colour(c) {
		this.#colour = this.constructor._parseColour(c);
		if (!this._inAcetate) {
			return;
		}

		const stridedArrays = this._inAcetate._getStridedArrays(
			this.attrBase + this.attrLength,
			this.idxBase + this.idxLength
		);
		this._setGlobalStrides(...stridedArrays);
		this._inAcetate._commitStridedArrays(
			this.attrBase,
			this.attrLength,
			this.idxBase,
			this.idxLength
		);
		this._inAcetate.dirty = true;
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
		const feather = this._inAcetate.feather;

		// Radian increment per step
		const ɛ = (Math.PI * 2) / this.steps;

		const ρ = this.#radius + feather / 2;
		const f = ρ * 256; // Feather max
		const [Δx, Δy] = this.offset;

		// Attributes start with the center point
		strideExtrusion.set([Δx, Δy], this.attrBase);
		strideColour?.set(this.#colour, this.attrBase);
		strideFeather?.set([0, f], this.attrBase);

		let θ = 0;
		let vtx = this.attrBase + 1;
		let idx = this.idxBase;
		for (let i = 0; i < this.steps; i++) {
			strideExtrusion.set([Math.sin(θ) * ρ + Δx, Math.cos(θ) * ρ + Δy], vtx);
			strideColour?.set(this.#colour, vtx);
			strideFeather?.set([f, f], vtx);

			// Vertices of the i-th triangle are: center, current, next
			if (i !== this.steps - 1) {
				typedIdxs?.set([this.attrBase, vtx, vtx + 1], idx);
			} else {
				typedIdxs?.set([this.attrBase, vtx, this.attrBase + 1], idx);
			}

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
