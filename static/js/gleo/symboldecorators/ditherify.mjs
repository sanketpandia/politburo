/**
 * @namespace ditherify
 * @inherits Symbol Decorator
 *
 * Applies a [dithering](https://en.wikipedia.org/wiki/Dither)-like filter to
 * any symbol: some pixels will be transparent based on a per-symbol threshold
 * and a acetate-wide noise texture.
 *
 * @example
 * ```
 * const DitherFill = ditherify(Fill);
 *
 * const ditherPolygon = new DitherFill(polygonGeometry, { dither: 0.9 }).addTo(map);
 * ```
 *
 */

export default function ditherify(base) {
	if (base.name === "HeatPoint") {
		throw new Error("The ditherify decorator cannot be applied to HeatPoints.");
	}

	class DitherifiedAcetate extends base.Acetate {
		#noiseTexture;

		constructor(glii, opts) {
			super(glii, opts);

			this._ditherAttr = new this.glii.SingleAttribute({
				usage: glii.STATIC_DRAW,
				size: 1,
				growFactor: 1.2,

				glslType: "float",
				type: Float32Array,
				normalized: true,
			});

			/// TODO: Sanity checks for F32 textures. Reuse from HeatMap.
			this.#noiseTexture = new this.glii.Texture({
				format: this.glii.gl.RED,
				internalFormat: this.glii.gl.R32F,
				type: this.glii.FLOAT,
				wrapS: this.glii.REPEAT,
				wrapT: this.glii.REPEAT,
			}).texArray(
				512,
				512,
				Float32Array.from(new Array(512 * 512), () => Math.random())
			);
		}

		glProgramDefinition() {
			const opts = super.glProgramDefinition();

			return {
				...opts,
				attributes: {
					aDither: this._ditherAttr,
					...opts.attributes,
				},
				textures: {
					uDitherNoise: this.#noiseTexture,
					...opts.textures,
				},
				varyings: {
					vDither: "float",
					...opts.varyings,
				},
				vertexShaderMain: `vDither = aDither;\n` + opts.vertexShaderMain,
				fragmentShaderMain:
					`
				float ditherThreshold = texture2D(uDitherNoise, gl_FragCoord.xy / 512.0).x;
				if (ditherThreshold > vDither) {
					discard;
				}
				` + opts.fragmentShaderMain,
			};
		}

		_getStridedArrays(maxVtx, maxIdx) {
			return [
				// Dither
				this._ditherAttr.asStridedArray(maxVtx),

				...super._getStridedArrays(maxVtx, maxIdx),
			];
		}

		_commitStridedArrays(baseVtx, vtxCount, baseIdx, idxCount) {
			this._ditherAttr.commit(baseVtx, vtxCount);
			return super._commitStridedArrays(baseVtx, vtxCount, baseIdx, idxCount);
		}
	}

	/**
	 * @miniclass Ditherified Symbol (ditherify)
	 *
	 * A "ditherified" symbol accepts these additional constructor options:
	 */
	class DitherifiedSymbol extends base {
		static Acetate = DitherifiedAcetate;
		constructor(
			geom,
			{
				/**
				 * @option dither: Number = 0.5
				 * The (expected) ratio of pixels to be seen, between `0` (none)
				 * and `1` (all)
				 */
				dither = 0.5,
				...opts
			} = {}
		) {
			super(geom, opts);
			this.dither = dither;
		}

		_setGlobalStrides(strideDither, ...strides) {
			for (let i = this.attrBase, t = this.attrBase + this.attrLength; i < t; i++) {
				strideDither.set([this.dither], i);
			}

			return super._setGlobalStrides(...strides);
		}
	}

	return DitherifiedSymbol;
}
