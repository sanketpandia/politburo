import { ScalarField } from "./Field.mjs";

/**
 * @class BlurField
 * @inherits ScalarField
 * @relationship compositionOf Acetate, 1..1, 1..1
 *
 * Blurs another acetate based on the intensity of the scalar field.
 *
 * @example
 *
 * An `BlurField` needs another `Acetate` or `Loader` to be added to
 * itself, like:
 *
 * ```
 * const blur = new BlurField(...);
 * const tiles = new MercatorTiles(tilesUrl).addTo(blur);
 *
 * new HeatPoint(...).addTo(blur);
 * ```
 *
 */

export default class BlurField extends ScalarField {
	#blurredAcetate;
	#minIntensity;
	#maxIntensity;
	#maxBlur;

	constructor(
		target,
		{
			/**
			 * @option minIntensity: Number = 0
			 * The intensity of the field corresponding to no blur at all.
			 * Any intensity at or below this value will produce no blur.
			 */
			minIntensity = 0,

			/**
			 * @option maxIntensity: Number = 1
			 * The intensity of the field corresponding to full blur.
			 * Any intensity at or above this value will produce maximum blur.
			 */
			maxIntensity = 1,

			/**
			 * @option maxBlur: Number = 4
			 * The maximum amplitude of the blur, in CSS pixels.
			 */
			maxBlur = 2,

			...opts
		} = {}
	) {
		super(target, opts);

		this.#minIntensity = minIntensity;
		this.#maxIntensity = maxIntensity;
		this.#maxBlur = maxBlur;
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
				uIntensityRange: "vec3",
				uMaxBlur: "vec2",
				...opts.uniforms,
			},
			// vertexShaderSource: `void main() {
			// 	gl_Position = vec4(aPos, 0., 1.);
			// 	vUV = aUV;
			// }`,
			// varyings: { vUV: "vec2" },
			fragmentShaderMain: `
				float value = texture2D(uField, vUV).x;

				vec2 blur = uMaxBlur * clamp(
					(value - uIntensityRange.x) / uIntensityRange.z
					, 0.0, 1.0);

				vec4 sample0 = texture2D(uBase, vUV);

				vec4 sample1 = texture2D(uBase, vUV + vec2(blur.x, 0.));
				vec4 sample2 = texture2D(uBase, vUV + vec2(0., blur.y));
				vec4 sample3 = texture2D(uBase, vUV - vec2(blur.x, 0.));
				vec4 sample4 = texture2D(uBase, vUV - vec2(0., blur.y));

				vec4 sample5 = texture2D(uBase, vUV + vec2(+blur.x, +blur.y));
				vec4 sample6 = texture2D(uBase, vUV + vec2(+blur.x, -blur.y));
				vec4 sample7 = texture2D(uBase, vUV + vec2(-blur.x, -blur.y));
				vec4 sample8 = texture2D(uBase, vUV + vec2(-blur.x, +blur.y));

				gl_FragColor = (sample0 +
					            sample1 + sample2 + sample3 + sample4 +
				                sample5 + sample6 + sample7 + sample8) / 9.;
			`,
		};
	}

	resize(w, h) {
		super.resize(w, h);
		// this._programs.setUniform("uAmplitudeRatio", this.#amplitudeRatio);
		// this._programs.setUniform("uPixelSize", [2 / w, 2 / h]);
		this._programs.setUniform("uIntensityRange", [
			this.#minIntensity,
			this.#maxIntensity,
			this.#maxIntensity - this.#minIntensity,
		]);
		this._programs.setUniform("uMaxBlur", [
			(this.#maxBlur * 2) / w,
			(this.#maxBlur * 2) / h,
		]);

		if (this.#blurredAcetate) {
			this.#blurredAcetate.resize(w, h);
			this._programs.setTexture("uBase", this.#blurredAcetate.asTexture());
		}
	}

	addAcetate(ac) {
		if (ac.constructor.PostAcetate) {
			super.addAcetate(ac);
		} else {
			if (this.#blurredAcetate) {
				throw new Error("Blur already has a subordinate RGBA acetate");
			}
			this.#blurredAcetate = ac;
			this._programs.setTexture("uBase", ac.asTexture());
		}
	}

	redraw() {
		if (!this.dirty) {
			return;
		}
		if (this.#blurredAcetate) {
			this.#blurredAcetate.redraw(...arguments);
		}
		return super.redraw.apply(this, arguments);
	}

	destroy() {
		super.destroy();
		this.#blurredAcetate.destroy();
	}

	set dirty(d) {
		super.dirty = d;
		if (this.#blurredAcetate) {
			this.#blurredAcetate.dirty = d;
		}
	}
	get dirty() {
		return super.dirty || this.#blurredAcetate.dirty;
	}
}
