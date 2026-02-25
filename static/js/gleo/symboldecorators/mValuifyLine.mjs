/**
 * @namespace mValuifyLine
 * @inherits Symbol Decorator
 *
 * Common abstraction for trailify and dashGrowify (useless when used on its own).
 */

export default function mValuifyLine(base) {
	let valid = false;
	let proto = base;
	while (!valid) {
		if (!proto) {
			throw new Error(
				"This symbol decorator can only be applied to Stroke, Chain, or Hair"
			);
		}

		if (proto.name === "mValuifiedSymbol") {
			return proto;
		}

		valid |=
			proto.name === "Stroke" || proto.name === "Chain" || proto.name === "Hair";
		proto = proto.__proto__;
	}

	/**
	 * @miniclass M-Valuified Acetate (mValuifyLine)
	 *
	 * A "trailified" symbol accepts this additional constructor options:
	 */
	class mValuifiedAcetate extends base.Acetate {
		#minMValue;
		#maxMValue;

		constructor(target, opts) {
			super(target, opts);

			this._mcoords = new this.glii.SingleAttribute({
				size: 1,
				growFactor: 1.2,
				usage: this.glii.DYNAMIC_DRAW,
				glslType: "float",
				type: Float32Array,
			});
		}

		glProgramDefinition() {
			const opts = super.glProgramDefinition();

			return {
				...opts,
				attributes: {
					...opts.attributes,
					aMCoord: this._mcoords,
				},
				uniforms: {
					...opts.uniforms,
					uMThreshold: "vec2", // Lower threshold, higher-lower delta
				},
				varyings: {
					...opts.varyings,
					vMCoord: "float",
				},
				vertexShaderMain: `${opts.vertexShaderMain}
					vMCoord = aMCoord;
				`,

				/// Typical fragment shader for subclasses:
				// fragmentShaderMain: `${opts.fragmentShaderMain}
				// 	if (vMCoord > (uMThreshold.x + uMThreshold.y)) {
				// 		discard;
				// 	} else if (vMCoord < uMThreshold.x) {
				// 		discard;
				// 	} else {
				// 		// Do something
				// 	}
				// `,
			};
		}

		/**
		 * @property minMValue
		 * The lower threshold for the m-values of symbols. Can be updated
		 * at runtime.
		 * @property maxMValue
		 * The higher threshold for the m-values of symbols. Can be updated
		 * at runtime.
		 */
		get minMValue() {
			return this.#minMValue;
		}
		get maxMValue() {
			return this.#maxMValue;
		}
		set minMValue(m) {
			this.#minMValue = m;
			this.#updateMValues();
		}
		set maxMValue(m) {
			this.#maxMValue = m;
			this.#updateMValues();
		}

		#updateMValues() {
			this._programs.setUniform("uMThreshold", [
				this.#minMValue,
				this.#maxMValue - this.#minMValue,
			]);
			this.dirty = true;
		}

		_getPerPointStridedArrays(maxVtx, maxIdx) {
			return [
				// M-value
				this._mcoords.asStridedArray(maxVtx),

				// Parent strided arrays
				...super._getPerPointStridedArrays(maxVtx, maxIdx),
			];
		}

		_commitPerPointStridedArrays(baseVtx, vtxCount) {
			this._mcoords.commit(baseVtx, vtxCount);
			return super._commitPerPointStridedArrays(baseVtx, vtxCount);
		}
	}

	/**
	 * @miniclass M-Valuified GleoSymbol (mValuifyLine)
	 *
	 * A "trailified" symbol accepts this additional constructor options:
	 */
	return class mValuifiedSymbol extends base {
		static Acetate = mValuifiedAcetate;

		#mCoords;

		constructor(
			geom,
			{
				/**
				 * @option mcoords: Array of Number = []
				 *
				 * The values for the M-coordinates of the geometry. This **must** have one
				 * M-coordinate per point in the symbol's geometry.
				 *
				 */
				mcoords = [],

				...opts
			} = {}
		) {
			super(geom, opts);
			this.#mCoords = mcoords;
		}

		_setPerPointStrides(n, pointType, vtx, vtxCount, strideMcoords, ...strides) {
			super._setPerPointStrides(n, pointType, vtx, vtxCount, ...strides);

			for (let i = 0; i < vtxCount; i++) {
				strideMcoords.set([this.#mCoords[n]], vtx + i);
			}
		}
	};
}
