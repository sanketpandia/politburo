import AcetateRotatingExtrusion from "../acetates/AcetateRotatingExtrusion.mjs";
// import AcetateSolidExtrusion from "../acetates/AcetateSolidExtrusion.mjs";
import CircleFill from "./CircleFill.mjs";
import parseColour from "../3rd-party/css-colour-parser.mjs";

/**
 * @class RadarSweep
 * @inherits CircleFill
 * @relationship drawnOn AcetateRotatingExtrusion
 *
 * Decorative animated rotating radar sweep.
 *
 * Behaves similar to a `CircleFill`, but the fill colour varies radially,
 * losing opacity. The end result is the movie-like sweep effect of a old-timey
 * CRT radar.
 */

export default class RadarSweep extends CircleFill {
	/// @section Static properties
	/// @property Acetate: Prototype of AcetateRotatingExtrusion
	// The `Acetate` class that draws this symbol.
	static Acetate = AcetateRotatingExtrusion;

	#radius;
	#colour;
	#speed;

	/**
	 * @constructor RadarSweep(geom: Geometry, opts?: RadarSweep Options)
	 */
	constructor(
		geom,
		{
			/**
			 * @section
			 * @aka RadarSweep Options
			 * @option speed: Number = 0.5
			 * Rotation speed, in revolutions per second.
			 */
			radius = 20,

			colour = "#3388ffff",

			speed = 0.5,

			...opts
		} = {}
	) {
		super(geom, opts);

		this.#speed = speed;
		this.#radius = radius;
		this.#colour = parseColour(colour);

		this.attrLength = this.steps * 2 + 3;
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
		strideRotateExtrusion,
		strideColour,
		strideFeather,
		strideSpeed,
		typedIdxs
	) {
		const feather = this._inAcetate.feather;
		// Radian increment per step
		const ɛ = (Math.PI * 2) / this.steps;

		const ρ = this.#radius + feather / 2;
		const f = ρ * 256; // Feather max
		const [offsetX, offsetY] = this.offset;
		const colour = this.#colour.slice(0);

		let vtx = this.attrBase;
		let idx = this.idxBase;

		// Attributes start with the center point
		strideExtrusion.set([offsetX, offsetY], vtx);
		strideRotateExtrusion?.set([0, 0], vtx);
		strideColour?.set(this.#colour, vtx);
		strideFeather?.set([0, f], vtx);
		strideSpeed?.set([this.#speed], vtx);

		let θ = 0;
		for (let i = 0; i < this.steps; i++) {
			// Alpha for current two vertices
			colour[3] = this.#colour[3] * (1 - i / this.steps);

			// Edge point
			strideExtrusion.set([offsetX, offsetY], vtx);
			strideRotateExtrusion?.set([Math.sin(θ) * ρ, Math.cos(θ) * ρ], vtx);
			strideColour?.set(colour, vtx);
			strideFeather?.set([f, f], vtx);
			strideSpeed?.set([this.#speed], vtx);
			vtx++;

			// Centre point
			strideExtrusion.set([offsetX, offsetY], vtx);
			strideRotateExtrusion?.set([0, 0], vtx);
			strideColour?.set(colour, vtx);
			strideFeather?.set([0, f], vtx);
			strideSpeed?.set([this.#speed], vtx);
			vtx++;

			typedIdxs?.set([vtx, vtx + 1, vtx + 2], idx);
			idx += 3;

			θ += ɛ;
		}

		// Final edge point, transparent
		colour[3] = 0;
		strideExtrusion.set([offsetX, offsetY], vtx);
		strideRotateExtrusion?.set([0, ρ], vtx);
		strideColour?.set(colour, vtx);
		strideFeather?.set([f, f], vtx);
		strideSpeed?.set([this.#speed], vtx);
		vtx++;
	}
}
