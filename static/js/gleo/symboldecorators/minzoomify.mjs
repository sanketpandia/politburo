/**
 * @namespace minzoomify
 * @inherits Symbol Decorator
 *
 * Makes the symbol visible only between a range of zoom levels (i.e. between
 * a set of scale factors). Akin to adding Leaflet's `minzoom` and `maxzoom`
 * options to a symbol.
 *
 * @example
 *
 * ```
 * import minzoomify from 'gleo/symboldecorators/minzoomify.mjs';
 *
 * const ZoomySprite = minzoomify(Sprite);

 * const sprite1 = new ZoomySprite(geom, {
 * 	minScale: 40000,	// Around Leaflet zoom level 3
 * 	minOpaqueScale: 30000,
 * 	maxOpaqueScale: 300,// Around Leaflet zoom level 10
 * 	maxScale: 200,
 * 	...spriteOptions
 * };
 * ```
 */

export default function minzoomify(base) {
	class MinZoomifiedAcetate extends base.Acetate {
		constructor(target, opts) {
			super(target, opts);

			// Holds, in this order:
			// min scale,
			// max scale,
			// min opaque scale delta,
			// max opaque scale delta
			this._minMaxScaleAttr = new this.glii.SingleAttribute({
				usage: this.glii.STATIC_DRAW,
				size: 1,
				growFactor: 1.2,
				glslType: "vec4",
				type: Float32Array,
				normalized: false,
			});
		}

		glProgramDefinition() {
			const opts = super.glProgramDefinition();

			// const extrudeRegExp = /(\W)aExtrude(\W)/;
			// function extrudeReplacement(_, p1, p2) {
			// 	return `${p1}bounce(aExtrude)${p2}`;
			// }
			const scaleGlslFilter = `

				float minZoomOpacity = (aMinMaxScale.x - uScale ) / aMinMaxScale.z;
				float maxZoomOpacity = (uScale - aMinMaxScale.y) / aMinMaxScale.w;

				vScaleOpacity = min( 1.0, min(minZoomOpacity, maxZoomOpacity));

				if (vScaleOpacity <= 0.0) {
					gl_Position.w = 0.0;	// Drop the vertex
				}
				`;

			const scaleGlslOpacity = `gl_FragColor.a *= vScaleOpacity;`;

			return {
				...opts,
				attributes: {
					// aBounceSquish: this._bounceAttr.getBindableAttribute(0),
					// aBounceHeight: this._bounceAttr.getBindableAttribute(1),
					aMinMaxScale: this._minMaxScaleAttr,
					...opts.attributes,
				},
				uniforms: {
					uScale: "float",
					...opts.uniforms,
				},
				vertexShaderMain: opts.vertexShaderMain + scaleGlslFilter,
				varyings: {
					...opts.varyings,
					vScaleOpacity: "float",
				},
				fragmentShaderMain: opts.fragmentShaderMain + scaleGlslOpacity,
			};
		}

		_getStridedArrays(maxVtx, maxIdx) {
			return [
				// min/max scale factors
				this._minMaxScaleAttr.asStridedArray(maxVtx),

				// Parent strided arrays
				...super._getStridedArrays(maxVtx, maxIdx),
			];
		}

		_commitStridedArrays(baseVtx, vtxCount) {
			this._minMaxScaleAttr.commit(baseVtx, vtxCount);
			return super._commitStridedArrays(baseVtx, vtxCount);
		}

		runProgram() {
			this._programs.setUniform("uScale", this.platina.scale);
			super.runProgram();
		}
	}

	return class MinZoomifiedSymbol extends base {
		static Acetate = MinZoomifiedAcetate;

		#minScale;
		#maxScale;
		#minOpaqueScale;
		#maxOpaqueScale;

		constructor(
			geom,
			{ minScale, maxScale, minOpaqueScale, maxOpaqueScale, ...opts }
		) {
			super(geom, opts);

			// When minScale / maxScale are not defined, use values next to the
			// largest/smallest numbers representable in float32.
			// See https://stackoverflow.com/questions/16069959/glsl-how-to-ensure-largest-possible-float-value-without-overflow

			this.#minScale = minScale ?? 1e38;
			this.#maxScale = maxScale ?? 1e-38;
			this.#minOpaqueScale = minOpaqueScale ?? this.#minScale;
			this.#maxOpaqueScale = maxOpaqueScale ?? this.#maxScale;
		}

		_setGlobalStrides(strideMinMaxScales, ...strides) {
			for (let i = this.attrBase, t = this.attrBase + this.attrLength; i < t; i++) {
				strideMinMaxScales.set(
					[
						this.#minScale,
						this.#maxScale,
						this.#minScale - this.#minOpaqueScale,
						this.#maxOpaqueScale - this.#maxScale,
					],
					i
				);
			}

			return super._setGlobalStrides(...strides);
		}
	};
}
