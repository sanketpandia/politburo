import ExtrudedPoint from "../symbols/ExtrudedPoint.mjs";

/**
 * @namespace bouncify
 * @inherits Symbol Decorator
 * @relationship associated ExtrudedPoint
 *
 * Modifies an `ExtrudedPoint` class (e.g. `Sprite`s, `CircleFill`s, etc) so that
 * they bounce in a time-based animation.
 *
 * @example
 *
 * A typical use case is to create bouncing sprites (akin to "bouncing markers"
 * in Leaflet):
 *
 * ```
 * import bouncify from 'gleo/src/symbols/Bouncify.mjs';
 *
 * const BouncingSprite = bouncify(Sprite);	// "Bouncified" `Sprite` class
 * const BouncingSpriteAcetate = BouncingSprite.Acetate;	// "Bouncified" `AcetateSprite` class
 * ```
 *
 * Declare bouncing sprites, as desired (including the bounce-related constructor
 * options):
 *
 * ```
 * const bouncySprite = new BouncingSprite(geom, {
 * 	bounceHeight: 10,
 * 	bounceSquish: [0.8, 1.2],
 * 	...spriteOptions
 * };
 * ```
 *
 */

export default function bouncify(base) {
	if (!base instanceof ExtrudedPoint) {
		throw new Error(
			"The 'bounficy' symbol decorator can only be applied to extruded points"
		);
	}

	class BouncifiedAcetate extends base.Acetate {
		constructor(glii, opts) {
			super(glii, opts);

			this._bounceAttr = new this.glii.InterleavedAttributes(
				{
					usage: this.glii.STATIC_DRAW,
					size: 1,
					growFactor: 1.2,
				},
				[
					{
						// squish X,Y + squash X,Y
						glslType: "vec4",
						type: Float32Array,
						normalized: false,
					},
					{
						// bounce height
						glslType: "float",
						type: Float32Array,
						normalized: false,
					},
					{
						// per-symbol offset
						glslType: "vec2",
						type: Float32Array,
						normalized: false,
					},
				]
			);
		}

		glProgramDefinition() {
			const opts = super.glProgramDefinition();

			const extrudeRegExp = /(\W)aExtrude(\W)/;
			function extrudeReplacement(_, p1, p2) {
				return `${p1}bounce(aExtrude)${p2}`;
			}
			const bounceGlslFunction = `
				vec2 bounce(vec2 extrusion){
					vec2 squish = vec2(
						mix(aBounceSquish.z, aBounceSquish.x, uBounceCycle),
						mix(aBounceSquish.w, aBounceSquish.y, uBounceCycle)
					);
					return ((extrusion - aBounceBaseOffset) * squish) +
					        aBounceBaseOffset +
					        vec2(0., aBounceHeight * uBounceCycle );
				}`;

			return {
				...opts,
				attributes: {
					aBounceSquish: this._bounceAttr.getBindableAttribute(0),
					aBounceHeight: this._bounceAttr.getBindableAttribute(1),
					aBounceBaseOffset: this._bounceAttr.getBindableAttribute(2),
					...opts.attributes,
				},
				uniforms: {
					// uNow: "float",
					uBounceCycle: "float",
					...opts.uniforms,
				},
				vertexShaderSource: opts.vertexShaderSource + bounceGlslFunction,
				vertexShaderMain: opts.vertexShaderMain.replace(
					extrudeRegExp,
					extrudeReplacement
				),
			};
		}

		_getStridedArrays(maxVtx, maxIdx) {
			return [
				// Bounce squish
				this._bounceAttr.asStridedArray(0, maxVtx),
				// Bounce height
				this._bounceAttr.asStridedArray(1),
				// Bounce base offset
				this._bounceAttr.asStridedArray(2),
				// Parent strided arrays
				...super._getStridedArrays(maxVtx, maxIdx),
			];
		}

		_commitStridedArrays(baseVtx, vtxCount) {
			this._bounceAttr.commit(baseVtx, vtxCount);
			return super._commitStridedArrays(baseVtx, vtxCount);
		}

		redraw() {
			// this._programs.setUniform("uNow", performance.now());
			this._programs.setUniform(
				"uBounceCycle",
				Math.abs(Math.sin(performance.now() / (1000 / Math.PI)))
			);
			return super.redraw.apply(this, arguments);
		}

		// An animated Acetate is always dirty as long as it has any symbols,
		// meaning it wants to render at every frame.
		get dirty() {
			return super.dirty || this._knownSymbols.length > 0;
		}
		set dirty(d) {
			return (super.dirty = d);
		}

		destroy() {
			this._bounceAttr.destroy();
			return super.destroy();
		}
	}

	/**
	 * @miniclass Bouncified GleoSymbol (bouncify)
	 *
	 * A "bouncified" symbol accepts these additional constructor options:
	 *
	 * @option bounceHeight: Number = 10
	 * The height of the boucing, in CSS pixels. This will always be vertical
	 * relative to the viewport.
	 *
	 * @option bounceSquish: Array of Number = [0.9, 1.1]
	 * The symbol will "squish" when at the apex of the bounce, effectively being
	 * scaled horizontally/vertically; these values control the scale factors.
	 * e.g. the default of `[0.9, 1.1]` means that the symbol will be 90% wide and
	 * 110% high at the apex, compared to its at-rest size.
	 *
	 * @option bounceSquash: Array of Number = [1.1, 0.7]
	 * Akin to `bounceSquish`, but for the bottom instant of the bounce.
	 */

	/// TODO: Bounce speed/time

	return class BouncifiedSymbol extends base {
		static Acetate = BouncifiedAcetate;

		#bounceSquish;
		#bounceHeight;

		constructor(
			geom,
			{
				bounceSquish = [0.9, 1.1],
				bounceSquash = [1.1, 0.7],
				bounceHeight = 10,
				...opts
			}
		) {
			super(geom, opts);
			this.#bounceSquish = [
				bounceSquish[0],
				bounceSquish[1],
				bounceSquash[0],
				bounceSquash[1],
			];
			this.#bounceHeight = bounceHeight;
		}

		_setGlobalStrides(
			strideSquish,
			strideBounceHeight,
			strideBounceBaseOffset,
			...strides
		) {
			for (let i = this.attrBase, t = this.attrBase + this.attrLength; i < t; i++) {
				strideSquish.set(this.#bounceSquish, i);
				strideBounceHeight.set([this.#bounceHeight], i);
				strideBounceBaseOffset.set(this.offset, i);
			}

			return super._setGlobalStrides(...strides);
		}

		_setStrideExtrusion(strideExtrusion) {
			const strideBaseOffset = this._inAcetate._bounceAttr.asStridedArray(2);
			for (let i = this.attrBase, t = this.attrBase + this.attrLength; i < t; i++) {
				strideBaseOffset.set(this.offset, i);
			}
			this._inAcetate._bounceAttr.commit(this.attrBase, this.attrLength);

			return super._setStrideExtrusion(strideExtrusion);
		}
	};
}
