import ScalarFieldAnimated from "./ScalarFieldAnimated.mjs";
import parseColour from "../3rd-party/css-colour-parser.mjs";

/**
 * @class TwinkleField
 * @inherits ScalarFieldAnimated
 *
 * An animated `ScalarField` that twinkles. The probability of any pixel twinkling
 * is proportional to the value of the underlying scalar field.
 *
 * Works similar to `HeatMap`, in the sense that this needs symbols
 * that add intensity to its scalar field.
 *
 * @example
 *
 * ```js
 * const twinkler = new TwinkleField(map, {
 * 		colour: 'red'
 * 		maxIntensity: 16000,
 * 	},
 * });
 *
 * new HeatPoint(geom, {radius: 80, intensity: 500}).addTo(twinkler);
 * ```
 */

export default class TwinkleField extends ScalarFieldAnimated {
	#colour;
	#maxIntensity;
	#noiseTexture;
	#noiseTextureSize;

	/**
	 * @constructor TwinkleField(target: GliiFactory)
	 *
	 */
	constructor(
		target,
		{
			/**
			 * @option colour: Colour = [0, 0, 0, 255]
			 * The colour of the twinkling pixels.
			 */
			colour = [0, 0, 0, 255],

			/**
			 * @option maxIntensity: Number = 65536
			 * Valur of the field that equals a probability of 1 of a pixel
			 * having a solid colour. In other words, the probability of
			 * any given pixel twinkling at any given time is its intensity
			 * divided by the maximum intensity of the twinkle field.
			 */
			maxIntensity = 65536,

			/**
			 * @option noiseTextureSize: Number = 512
			 * Width and height of the internal noise texture used for
			 * pseudo-RNG. This repeats, so small values might result in
			 * visible non-random patterns.
			 */
			noiseTextureSize = 512,

			...opts
		} = {}
	) {
		super(target, opts);

		this.#colour = parseColour(colour);
		this.#maxIntensity = maxIntensity;

		const n = (this.#noiseTextureSize = noiseTextureSize);
		this.#noiseTexture = new this.glii.Texture({
			format: this.glii.gl.RED,
			internalFormat: this.glii.gl.R32F,
			type: this.glii.FLOAT,
			wrapS: this.glii.REPEAT,
			wrapT: this.glii.REPEAT,
		}).texArray(
			n,
			n,
			Float32Array.from(new Array(n * n), () => Math.random())
		);
	}

	// Returns the definition for the GL program that turns the float32 texture
	// into a RGBA8 texture
	glProgramDefinition() {
		const opts = super.glProgramDefinition();
		return {
			...opts,
			textures: {
				uTwinkleRNG: this.#noiseTexture,
				...opts.textures,
			},
			uniforms: {
				uMaxIntensity: "float",
				uRNGoffset: "vec2",
				uRNGsize: "vec2",
				uColour: "vec4",
				...opts.uniforms,
			},
			fragmentShaderMain: `
				float value = texture2D(uField, vUV).x;
				float prob = texture2D(uTwinkleRNG, (vUV + uRNGoffset) / uRNGsize ).x;

				if (value / uMaxIntensity > prob) {
					gl_FragColor = uColour;
				} else {
					discard;
				}
			`,
		};
	}

	resize(w, h) {
		super.resize(w, h);
		this._programs.setUniform("uMaxIntensity", this.#maxIntensity);
		this._programs.setUniform("uColour", this.#colour);
		this._programs.setUniform("uRNGsize", [
			this.#noiseTextureSize / w,
			this.#noiseTextureSize / h,
		]);
	}

	redraw() {
		this._programs.setUniform("uRNGoffset", [Math.random(), Math.random()]);
		return super.redraw.apply(this, arguments);
	}
}
