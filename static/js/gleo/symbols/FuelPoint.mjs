import AcetateFuelPoint from "../acetates/AcetateFuelPoint.mjs";
import GleoSymbol from "./Symbol.mjs";

/**
 * @class FuelPoint
 * @inherits GleoSymbol
 * @relationship drawnOn AcetateFuelPoint
 *
 * A point for a scalar field, similar to `HeatPoint`. Unlike `HeatPoint`,
 * `FuelPoint`s do not have a radius in pixels and only have an intensity. The
 * intensity of all `FuelPoint`s in the same `AcetateFuelPoint`
 * increases/decreases at the same rate.
 */

export default class FuelPoint extends GleoSymbol {
	/// @section Static properties
	/// @property Acetate: Prototype of AcetateFuelPoint
	// The `Acetate` class that draws this symbol.
	static Acetate = AcetateFuelPoint;

	#intensity;

	/**
	 * @constructor HeatPoint(geom: Geometry, opts?: HeatPoint Options)
	 */
	constructor(
		geom,
		{
			/**
			 * @section
			 * @aka HeatMap Options
			 * @option intensity: Number = 10
			 * Intensity of the point, at its center. The intensity fades linearly
			 * outwards, at the same rate for all `FuelPoint`s.
			 */
			intensity = 10,

			interactive = true,

			...opts
		} = {}
	) {
		super(geom, { ...opts, interactive });
		this.#intensity = intensity;

		// Assume a constant number of circle subdivisions for all FuelPoints.
		this.steps = 16;

		this.attrLength = this.steps + 1;
		this.idxLength = this.steps * 3;
	}

	/**
	 * @property intensity
	 * The value of the `intensity` option at instantiation time. Read-only.
	 */
	get intensity() {
		return this.#intensity;
	}

	_setGlobalStrides(strideIntensity, strideDistance, typedIdxs, radius) {
		// Radian increment per step
		// const ɛ = (Math.PI * 2) / this.steps;

		/// TODO: Scale radius - the offset should be geodetic instead of CRS-planar
		// const ρ = radius;

		// const [Δx, Δy] = this.offset;

		// Attributes start with the center point
		// strideExtrusion.set([Δx, Δy], this.attrBase);
		strideIntensity?.set([this.#intensity], this.attrBase);
		strideDistance?.set([0], this.attrBase);

		// let θ = 0;
		let vtx = this.attrBase + 1;
		let idx = this.idxBase;

		// Intensity is a single attribute, and all values but the first must be set to zero
		// strideIntensity?.set(new Array(this.attrLength - 1).fill(0), vtx);

		// Intensity is the same for all vertices
		// strideIntensity?.set(new Array(this.attrLength).fill(this.#intensity), this.attrBase);

		for (let i = 0; i < this.steps; i++) {
			// strideExtrusion.set([Math.sin(θ) * ρ + Δx, Math.cos(θ) * ρ + Δy], vtx);
			strideDistance?.set([radius], vtx);
			strideIntensity?.set([this.#intensity], vtx);

			// Vertices of the i-th triangle are: center, current, next
			if (i !== this.steps - 1) {
				typedIdxs?.set([this.attrBase, vtx, vtx + 1], idx);
			} else {
				typedIdxs?.set([this.attrBase, vtx, this.attrBase + 1], idx);
			}

			// θ += ɛ;
			vtx++;
			idx += 3;
		}
	}
}
