import ExtrudedPoint from "../symbols/ExtrudedPoint.mjs";

/**
 * @namespace pulsify
 * @inherits Symbol Decorator
 * @relationship associated ExtrudedPoint
 *
 * Modifies an `ExtrudedPoint` class (e.g. `Sprite`s, `CircleFill`s, etc), so
 * that they grow and lower their opacity with time. The effect is a "pulse",
 * meant to notify of an event happening.
 *
 * The symbols are expanded from zero extrusion size to the *given* size. This
 * means that circles should be given a big radius, and `Sprite`s should use
 * `spriteScale`.
 *
 */

export default function pulsify(base) {
	if (!base instanceof ExtrudedPoint) {
		throw new Error(
			"The 'pulsify' symbol decorator can only be applied to extruded points"
		);
	}

	class PulsifiedAcetate extends base.Acetate {
		constructor(target, opts) {
			super(target, opts);

			this._pulseAttr = new this.glii.InterleavedAttributes(
				{
					usage: this.glii.STATIC_DRAW,
					size: 1,
					growFactor: 1.2,
				},
				[
					{
						// start timestamp (offset), pulse duration
						glslType: "vec2",
						type: Float32Array,
						normalized: false,
					},
					{
						// Start & end opacity factors
						glslType: "vec2",
						type: Float32Array,
						normalized: false,
					},
				]
			);
		}

		glProgramDefinition() {
			const opts = super.glProgramDefinition();

			// Replace `aExtrude` occurrences in the vertex shader by a
			// function that applies the pulse scale factor in the vertex
			// shader
			const extrudeRegExp = /(\W)aExtrude(\W)/;
			function extrudeReplacement(_, p1, p2) {
				return `${p1}pulse(aExtrude, pulsePercentage)${p2}`;
			}

			return {
				...opts,
				attributes: {
					...opts.attributes,
					aPulseTime: this._pulseAttr.getBindableAttribute(0),
					aPulseOpacity: this._pulseAttr.getBindableAttribute(1),
				},
				varyings: {
					vPulseOpacity: "float",
					...opts.varyings,
				},
				uniforms: {
					uNow: "float",
					...opts.uniforms,
				},
				vertexShaderSource:
					`
				vec2 pulse(vec2 extrusion, float pulsePercentage){
					return (extrusion * vec2(pulsePercentage, pulsePercentage));
				}` + opts.vertexShaderSource,
				vertexShaderMain:
					`
					float pulsePercentage = fract((uNow - aPulseTime.x) / aPulseTime.y);
					vPulseOpacity = clamp(mix(aPulseOpacity.x, aPulseOpacity.y, pulsePercentage), 0., 1.);
				` + opts.vertexShaderMain.replace(extrudeRegExp, extrudeReplacement),
				fragmentShaderMain: `${opts.fragmentShaderMain}
				gl_FragColor.a *= vPulseOpacity;`,
			};
		}

		_getStridedArrays(maxVtx, maxIdx) {
			return [
				// Pulse timestamp+duration
				this._pulseAttr.asStridedArray(0, maxVtx),
				// Pulse opacity factors
				this._pulseAttr.asStridedArray(1),
				// Parent strided arrays
				...super._getStridedArrays(maxVtx, maxIdx),
			];
		}

		_commitStridedArrays(baseVtx, vtxCount) {
			this._pulseAttr.commit(baseVtx, vtxCount);
			return super._commitStridedArrays(baseVtx, vtxCount);
		}

		redraw() {
			this._programs.setUniform("uNow", performance.now());
			return super.redraw.apply(this, arguments);
		}

		// An animated Acetate is always dirty, meaning it wants to render at every frame.
		get dirty() {
			return true;
		}
		set dirty(_) {}

		destroy() {
			this._pulseAttr.destroy();
			return this.destroy();
		}
	}

	/**
	 * @miniclass Pulsified GleoSymbol (pulsify)
	 *
	 * A "pulsified" symbol accepts these additional constructor options:
	 */

	/// TODO: number of pulses

	return class PulsifiedSymbol extends base {
		static Acetate = PulsifiedAcetate;

		#pulseDuration;
		#pulseOpacities;
		#pulseCount;

		constructor(
			geom,
			{
				/**
				 * @option pulseDuration: Number = 1000
				 * Duration, in milliseconds, of a pulse animation.
				 *
				 * @option pulseInitialOpacity: Number = 1.5
				 * Opacity factor at the start of the pulse animation. The runtime value
				 * is clamped between 0 and 1, so values larger than 1 mean that the
				 * symbol stays at full opacity during a portion of the pulse animation.
				 *
				 * @option pulseFinalOpacity: Number = 0
				 * Opacity factor at the end of the pulse animation. Defaults to
				 * full transparency.
				 *
				 * @option pulseCount: Number = Infinity
				 * If given a finite integer value, the pulse will be removed after
				 * that many pulse animations.
				 */
				pulseDuration = 1000,
				pulseInitialOpacity = 1.5,
				pulseFinalOpacity = 0,
				pulseCount = Infinity,
				...opts
			}
		) {
			super(geom, opts);
			this.#pulseDuration = pulseDuration;
			this.#pulseOpacities = [pulseInitialOpacity, pulseFinalOpacity];
			this.#pulseCount = pulseCount;
		}

		_setGlobalStrides(pulseTime, pulseOpacity, ...strides) {
			const now = performance.now();
			for (let i = this.attrBase, t = this.attrBase + this.attrLength; i < t; i++) {
				pulseTime.set([now, this.#pulseDuration], i);
				pulseOpacity.set(this.#pulseOpacities, i);
			}

			if (isFinite(this.#pulseCount)) {
				setTimeout(
					this.remove.bind(this),
					this.#pulseCount * this.#pulseDuration
				);
			}

			return super._setGlobalStrides(...strides);
		}
	};
}
