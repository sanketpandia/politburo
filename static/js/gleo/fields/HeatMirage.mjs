import ScalarFieldAnimated from "./ScalarFieldAnimated.mjs";

/**
 * @class HeatMirage
 * @inherits ScalarFieldAnimated
 * @relationship compositionOf Acetate, 1..1, 1..1
 *
 * Warps another acetate based on the intensity of the scalar field, in
 * an animated way. The amplitude of the warp is directly proportional to the
 * value of the scalar field.
 *
 * @example
 *
 * An `HeatMirage` needs another `Acetate` or `Loader` to be added to
 * itself, like:
 *
 * ```
 * const mirage = new HeatMirage( ... );
 * const tiles = new MercatorTiles(tilesUrl).addTo(mirage);
 *
 * new HeatPoint(...).addTo(mirage);
 * ```
 *
 */

export default class HeatMirage extends ScalarFieldAnimated {
	#warpedAcetate;
	#amplitudeRatio;
	#maxAmplitude;
	#speed;
	#phase;

	constructor(
		target,
		{
			/**
			 * @option amplitudeRatio: Number = 1
			 * The ratio between the amplitude of the warp and the intensity of the
			 * scalar field, in (horizontal) CSS pixels per unit of amplitude.
			 */
			amplitudeRatio = 1,

			/**
			 * @option maxAmplitude: Number = 32
			 * The maximum amplitude, in CSS pixels.
			 */
			maxAmplitude = 32,

			/**
			 * @option phase: Number = 32
			 * The phase of the wave, in CSS pixels. This can be thought as the
			 * amplitude of the "vertical" waves seen.
			 */
			phase = 32,

			/**
			 * @option speed: Number = .25
			 * Frequency of the wave, in hertz
			 */
			speed = 0.25,

			...opts
		} = {}
	) {
		super(target, opts);

		this.#amplitudeRatio = amplitudeRatio;
		this.#maxAmplitude = maxAmplitude;
		this.#speed = speed;
		this.#phase = phase;
	}

	glProgramDefinition() {
		const opts = super.glProgramDefinition();

		return {
			...opts,
			textures: {
				uBase: undefined,
				...opts.textures,
			},
			uniforms: {
				uAmplitude: "float",
				uMaxAmplitude: "float",
				uCycle: "float",
				uPhaseMult: "float",
				// uSpeed: "float",
				...opts.uniforms,
			},
			// vertexShaderSource: `void main() {
			// 	gl_Position = vec4(aPos, 0., 1.);
			// 	vUV = aUV;
			// }`,
			// varyings: { vUV: "vec2" },
			fragmentShaderMain: `
				float value = texture2D(uField, vUV).x;

				float offset = min(
					// (cos(uCycle + sin((vUV.y +uCycle) * 5.)))* value * uAmplitude,
					(cos(uCycle - vUV.y * uPhaseMult)) * value * uAmplitude,
					uMaxAmplitude
				);
				// offset /= 10.;

				gl_FragColor = texture2D(uBase, vUV + vec2(offset, 0.));

				// gl_FragColor.r = offset;
			`,
		};
	}

	resize(w, h) {
		super.resize(w, h);
		const dpr = devicePixelRatio ?? 1;
		const dpr2 = dpr * 2;
		// this._programs.setUniform("uAmplitudeRatio", this.#amplitudeRatio);
		// this._programs.setUniform("uPixelSize", [2 / w, 2 / h]);
		this._programs.setUniform("uAmplitude", (this.#amplitudeRatio * dpr2) / w);
		this._programs.setUniform("uMaxAmplitude", (this.#maxAmplitude * dpr2) / w);

		// Phase-based vertical multiplier
		// this._programs.setUniform("uPhase", (2 / h) * this.#phase );
		this._programs.setUniform("uPhaseMult", (dpr * h) / this.#phase);

		if (this.#warpedAcetate) {
			this.#warpedAcetate.resize(w, h);
			this._programs.setTexture("uBase", this.#warpedAcetate.asTexture());
		}
	}

	addAcetate(ac) {
		if (ac.constructor.PostAcetate) {
			super.addAcetate(ac);
		} else {
			if (this.#warpedAcetate) {
				throw new Error("Heat mirage already has a subordinate RGBA acetate");
			}
			this.#warpedAcetate = ac;
			this._programs.setTexture("uBase", ac.asTexture());
		}
	}

	redraw() {
		if (!this.dirty) {
			return;
		}
		this._programs.setUniform(
			"uCycle",
			(performance.now() % (1000 / this.#speed)) / (500 / 3.14159 / this.#speed)
		);
		if (this.#warpedAcetate) {
			this.#warpedAcetate.redraw(...arguments);
		}
		return super.redraw.apply(this, arguments);
	}

	destroy() {
		super.destroy();
		this.#warpedAcetate.destroy();
	}

	has(ac) {
		return ac === this.#warpedAcetate;
		/// TODO: implement has() for all acetates (and call super.has() here)
	}

	set dirty(d) {
		super.dirty = d;
		if (this.#warpedAcetate) {
			this.#warpedAcetate.dirty = d;
		}
	}
	get dirty() {
		return super.dirty;
	}
}
