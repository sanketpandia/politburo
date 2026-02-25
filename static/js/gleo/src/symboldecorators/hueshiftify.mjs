import Acetate from "../acetates/Acetate.mjs";

/**
 * @namespace hueshiftify
 * @inherits Symbol Decorator
 *
 * Adds a `hueShift` option to a symbol; that's the amount of [hue](https://en.wikipedia.org/wiki/Hue)
 * rotation applied to its pixels.
 */

// From https://stackoverflow.com/questions/15095909/from-rgb-to-hsv-in-opengl-glsl/17897228#17897228 :
const hsv2rgb = `
const vec4 K1 = vec4(1.0, 2.0 / 3.0, 1.0 / 3.0, 3.0);

vec3 hsv2rgb(vec3 c)
{
    vec3 p = abs(fract(c.xxx + K1.xyz) * 6.0 - K1.www);
    return c.z * mix(K1.xxx, clamp(p - K1.xxx, 0.0, 1.0), c.y);
}

vec4 K2 = vec4(0.0, -1.0 / 3.0, 2.0 / 3.0, -1.0);

vec3 rgb2hsv(vec3 c)
{
    vec4 p = mix(vec4(c.bg, K2.wz), vec4(c.gb, K2.xy), step(c.b, c.g));
    vec4 q = mix(vec4(p.xyw, c.r), vec4(c.r, p.yzx), step(p.x, c.r));

    float d = q.x - min(q.w, q.y);
    float e = 1.0e-10;
    return vec3(abs(q.z + (q.w - q.y) / (6.0 * d + e)), d / (q.x + e), q.x);
}
`;

export default function hueshiftify(base) {
	if (base.Acetate.PostAcetate && base.Acetate.PostAcetate !== Acetate) {
		// Throw error if the symbol operates on a scalar/vector field
		throw new Error(
			`The symbol class to be hueshiftified (${base.constructor.name}) doesn't seem to operate with colours`
		);
	}

	class HueshiftifiedAcetate extends base.Acetate {
		constructor(target, opts) {
			super(target, opts);

			this._shiftAttr = new this.glii.SingleAttribute({
				usage: this.glii.STATIC_DRAW,
				size: 1,
				growFactor: 1.2,

				glslType: "float",
				type: Uint8Array,
				normalized: true,
			});
		}

		_getStridedArrays(maxVtx, maxIdx) {
			return [
				// Hue shift
				this._shiftAttr.asStridedArray(maxVtx),

				// Parent strided arrays
				...super._getStridedArrays(maxVtx, maxIdx),
			];
		}

		_commitStridedArrays(baseVtx, vtxCount) {
			this._shiftAttr.commit(baseVtx, vtxCount);
			return super._commitStridedArrays(baseVtx, vtxCount);
		}

		glProgramDefinition() {
			const opts = super.glProgramDefinition();

			return {
				...opts,
				attributes: {
					...opts.attributes,
					aHueShift: this._shiftAttr,
				},
				varyings: {
					...opts.varyings,
					vHueShift: "float",
				},
				vertexShaderMain: opts.vertexShaderMain + "vHueShift = aHueShift;",
				fragmentShaderSource: hsv2rgb + opts.fragmentShaderSource,
				fragmentShaderMain:
					opts.fragmentShaderMain +
					`
				gl_FragColor.rgb = hsv2rgb(rgb2hsv(gl_FragColor.rgb) + vec3(vHueShift, 0., 0.));
				`,
			};
		}
	}

	// Cover the non-raster constructors
	if (base.prototype.constructor.length === 1) {
		/**
		 * @miniclass Hueshiftified Symbol (hueshiftify)
		 *
		 * A "hueshiftified" symbol accepts these additional constructor options:
		 */
		class HueshiftifiedSymbol extends base {
			static Acetate = HueshiftifiedAcetate;

			#hueShift;

			constructor(
				geometry,
				{
					/**
					 * @option hueShift: Number = 0
					 *
					 * The amount of hue shift to apply, in degrees
					 */
					hueShift = 0,

					...opts
				} = {}
			) {
				super(geometry, opts);

				this.#hueShift = hueShift;
			}

			_setGlobalStrides(strideHueShift, ...strides) {
				// Translate from a 0..360 value to a 0..255 value (which will
				// be normalized into a 0..1 value in the attribute buffer)
				const byteHueShift = (modulo(this.#hueShift, 360) * 255) / 360;

				for (
					let i = this.attrBase, t = this.attrBase + this.attrLength;
					i < t;
					i++
				) {
					strideHueShift.set([byteHueShift], i);
				}

				return super._setGlobalStrides(...strides);
			}
		}

		return HueshiftifiedSymbol;
	} else {
		// Cover the raster constructors

		class HueshiftifiedRaster extends base {
			static Acetate = HueshiftifiedAcetate;

			#hueShift;

			constructor(geometry, raster, { hueShift = 0, ...opts } = {}) {
				super(geometry, raster, opts);

				this.#hueShift = hueShift;
			}

			_setGlobalStrides(strideHueShift, ...strides) {
				// Translate from a 0..360 value to a 0..255 value (which will
				// be normalized into a 0..1 value in the attribute buffer)
				const byteHueShift = (modulo(this.#hueShift, 360) * 255) / 360;

				for (
					let i = this.attrBase, t = this.attrBase + this.attrLength;
					i < t;
					i++
				) {
					strideHueShift.set([byteHueShift], i);
				}

				return super._setGlobalStrides(...strides);
			}
		}

		return HueshiftifiedRaster;
	}
}

function modulo(a, n) {
	// As per https://developer.mozilla.org/en-US/docs/Web/JavaScript/Reference/Operators/Remainder

	return ((a % n) + n) % n;
}
