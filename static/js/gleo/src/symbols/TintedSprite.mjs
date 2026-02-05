import Sprite from "./Sprite.mjs";
import parseColour from "../3rd-party/css-colour-parser.mjs";

/**
 * @class AcetateTintedSprite
 * @inherits AcetateSprite
 *
 * As `AcetateSprite`, but aditionally the shader applies a tint to the image.
 *
 * Meant to be used with `TintedSprite` symbols.
 *
 */
class AcetateTintedSprite extends Sprite.Acetate {
	constructor(target, opts) {
		super(target, { zIndex: 4000, ...opts });

		// this._indices = new glii.WireframeTriangleIndices({ type: glii.UNSIGNED_INT });

		this._attrs = new this.glii.InterleavedAttributes(
			{
				usage: this.glii.STATIC_DRAW,
				size: 1,
				growFactor: 1.2,
			},
			[
				{
					// Texture UV coords (relative to acetate image atlas)
					glslType: "vec2",
					type: Float32Array,
					normalized: false,
				},
				{
					// Tint colour
					glslType: "vec4",
					type: Uint8Array,
					normalized: true,
				},
			]
		);
	}

	_getStridedArrays(maxVtx, maxIdx) {
		return [
			// Tint
			this._attrs.asStridedArray(1, maxVtx),
			// UV
			this._attrs.asStridedArray(0),
			// Extrusion
			this._extrusions.asStridedArray(maxVtx),
			// Index buffer
			this._indices.asTypedArray(maxIdx),
			// Texture size (width and height), in texels
			this._texSize,
		];
	}

	glProgramDefinition() {
		const opts = super.glProgramDefinition();
		return {
			...opts,
			attributes: {
				...opts.attributes,
				aUV: this._attrs.getBindableAttribute(0),
				aTint: this._attrs.getBindableAttribute(1),
			},
			vertexShaderMain: `
				vUV = aUV;
				vTint = aTint;
				gl_Position = vec4(
					vec3(aCoords, 1.0) * uTransformMatrix +
					vec3(aExtrude * uPixelSize, 0.0)
					, 1.0);
			`,
			varyings: { vUV: "vec2", vTint: "vec4" },
			fragmentShaderMain: `gl_FragColor = texture2D(uAtlas,vUV) * vTint;`,
		};
	}
}

/**
 * @class TintedSprite
 * @inherits Sprite
 * @relationship drawnOn AcetateTintedSprite
 *
 * As `Sprite`, but with a colour tint applied.
 *
 * @example
 * ```js
 * new Sprite([0, 0], {
 * 	image: "img/whitemarker.png",
 * 	spriteAnchor: [13, 41]
 * 	tint: "red",
 * }).addTo(map);
 * ```
 */

// export default class Sprite extends GleoSymbol {
export default class TintedSprite extends Sprite {
	/// @section Static properties
	/// @property Acetate: Prototype of AcetateTintedSprite
	// The `Acetate` class that draws this symbol.
	static Acetate = AcetateTintedSprite;
	#tintColour;

	/**
	 * @constructor TintedSprite(geom: Geometry, opts?: TintedSprite Options)
	 * @alternative
	 * @constructor TintedSprite(geom: Array of Number, opts?: TintedSprite Options)
	 */
	constructor(
		geom,
		{
			/**
			 * @section
			 * @aka TintedSprite Options
			 * @option tint: Colour = [255,255,255,255]
			 * The tint colour for the sprite. The result colour is
			 * the [colour multiplication](https://en.wikipedia.org/wiki/Blend_modes#Multiply)
			 * of the sprite's pixels and the tint.
			 */

			tint = [255, 255, 255, 255],
			...opts
		} = {}
	) {
		super(geom, opts);

		this.#tintColour = parseColour(tint);
	}

	_setGlobalStrides(strideTint, ...strides) {
		strideTint.set(this.#tintColour, this.attrBase + 0);
		strideTint.set(this.#tintColour, this.attrBase + 1);
		strideTint.set(this.#tintColour, this.attrBase + 2);
		strideTint.set(this.#tintColour, this.attrBase + 3);
		return super._setGlobalStrides(...strides);
	}

	/**
	 * @property tint: Colour
	 * Gets or sets the tint colour for this sprite
	 */
	get tint() {
		return this.#tintColour;
	}
	set tint(t) {
		this.#tintColour = parseColour(t);

		if (!this._inAcetate || this.attrBase === undefined) {
			return this;
		}

		let strideTint = this._inAcetate._attrs.asStridedArray(1);

		strideTint.set(this.#tintColour, this.attrBase + 0);
		strideTint.set(this.#tintColour, this.attrBase + 1);
		strideTint.set(this.#tintColour, this.attrBase + 2);
		strideTint.set(this.#tintColour, this.attrBase + 3);
		this._inAcetate._attrs.commit(this.attrBase, this.attrLength);
		this._inAcetate.dirty = true;
	}
}
