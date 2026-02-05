import ExtrudedPoint from "./ExtrudedPoint.mjs";

import imagePromise from "../util/imagePromise.mjs";
import imagifyFetchResponse from "../util/imagifyFetchResponse.mjs";

import AcetateExtrudedPoint from "../acetates/AcetateExtrudedPoint.mjs";
import Acetate from "../acetates/Acetate.mjs";

import ShelfPack from "../3rd-party/shelf-pack.mjs";

const percentageRegexp = /(\d+)%$/;

/**
 * @class AcetateSprite
 * @inherits AcetateExtrudedPoint
 * @relationship compositionOf ShelfPack, 1..1, 1..1
 *
 * An `Acetate` that draws square images anchored on points, at a constant screen
 * ratio. The images are "pinned" or "anchored" to a point `Geometry`.
 *
 * Since this acetate supports `Sprites` with different images, this acetate
 * implements a texture atlas. To build it, this leverages
 * [Bryan Housel's `shelf-pack` library](https://github.com/mapbox/shelf-pack).
 */

/// TODO: Angle relative to "up"?

class AcetateSprite extends AcetateExtrudedPoint {
	constructor(
		target,
		{
			/**
			 * @section AcetateSprite Options
			 * @option interpolate: true
			 * Whether to use bilinear pixel interpolation or not.
			 *
			 * Enabled by default to provide a alightly better display
			 * of sprites with yaw rotation applied. If (and only if) all
			 * sprites will have a yaw rotation of zero, it might be a good
			 * idea to set this to `false` to have pixel-accurate sprites.
			 */
			interpolate = true,
			/**
			 * @option maxTextureSize: Number = 2048
			 * Maximum size of the WebGL texture used to hold tiles. Should be
			 * at least the width/height of the screen. Texture size is ultimately
			 * bound by the WebGL capabilities of the browser/OS/GPU, which usually
			 * can support textures 8192 or 16384 pixels wide/high. Using higher
			 * values might cause the web browser (chromium/chrome in particular)
			 * to take a longer time during texture initialization.
			 */
			maxTextureSize = 2048,
			...opts
		} = {}
	) {
		super(target, { zIndex: 4000, ...opts });

		// Two attributes: vertex extrusion amount and texture UV
		// coordinates (relative to the image atlas for the acetate).
		// Vertex extrusion amount is handled by parent class.
		this._attrs = new this.glii.SingleAttribute({
			usage: this.glii.STATIC_DRAW,
			size: 1,
			growFactor: 1.2,

			// Texture UV coords (relative to acetate image atlas)
			glslType: "vec2",
			type: Float32Array,
			normalized: false,
		});

		// Data structures for the texture atlas
		const texSize = (this._texSize = Math.min(
			this.glii.Texture.getMaxSize(),
			maxTextureSize
		));
		this._packer = new ShelfPack(texSize, texSize);

		const texFilter = !!interpolate
			? this.glii.LINEAR
			: this.glii.NEAREST_MIPMAP_LINEAR;
		this._atlas = new this.glii.Texture({
			minFilter: texFilter,
			magFilter: texFilter,
		});
		this._atlas.texArray(texSize, texSize, new Uint8Array(4 * texSize * texSize));

		// Map of HTMLImageElement/blob to atlas coordinate box
		this._images = new Map();
	}

	glProgramDefinition() {
		const opts = super.glProgramDefinition();
		return {
			...opts,
			attributes: {
				...opts.attributes,
				aUV: this._attrs,
			},
			uniforms: {
				uPixelSize: "vec2",
				...opts.uniforms,
			},
			textures: {
				uAtlas: this._atlas,
				...opts.textures,
			},
			vertexShaderMain: `
				vUV = aUV;
				gl_Position = vec4(
					vec3(aCoords, 1.0) * uTransformMatrix +
					vec3(aExtrude * uPixelSize, 0.0)
					, 1.0);
			`,
			varyings: { vUV: "vec2" },
			fragmentShaderMain: `
				gl_FragColor = texture2D(uAtlas,vUV);
			`,
			// 				if (gl_FragColor.a < 1.0) {
			// 					gl_FragColor = vec4(0.,0.,0.,1.);
			// 				}
		};
	}

	glIdProgramDefinition() {
		const opts = super.glIdProgramDefinition();
		return {
			...opts,
			fragmentShaderMain: `if (texture2D(uAtlas, vUV).a > 0.0) {
				${opts.fragmentShaderMain}
			} else {
				discard;
			}`,
		};
	}

	/**
	 * @section Internal Methods
	 * @uninheritable
	 * @method pack(image: HTMLImageElement): Bin
	 * Given an image, returns a ShelfPack `Bin`, containing
	 * the box coordinates for that image inside the acetate's texture atlas.
	 *
	 * Will upload the image to the texture atlas if it's not in the atlas already;
	 * returns a cached result if the image has already been packed in the atlas.
	 * Anyway, it will increase the usage counter for the used portion of the atlas.
	 * @alternative
	 * @method pack(image: ImageData): Bin
	 * Idem, but takes a `ImageData` instance (from e.g. `TextLabel` symbol)
	 */
	pack(image) {
		const known = this._images.get(image);
		if (known) {
			this._packer.ref(known);
			return known;
		}

		let w, h;
		if (image instanceof ImageData) {
			h = image.height;
			w = image.width;
		} else if (image instanceof HTMLImageElement) {
			h = image.naturalHeight;
			w = image.naturalWidth;
		} else {
			throw new Error(
				"Cannot pack symbol's image: must be either HTMLImageElement or ImageData"
			);
		}
		const bin = this._packer.packOne(w, h);

		if (bin === null) {
			throw new Error("Could not allocate sprite image in atlas");
		}

		this._images.set(image, bin);

		this._atlas.texSubImage2D(image, bin.x, bin.y);

		return bin;
	}

	/**
	 * @section
	 * @method multiAdd(sprites: Array of Sprite): this
	 * Adds the sprites to this acetate (so they're drawn on the next refresh),
	 * using as few WebGL calls as feasible.
	 *
	 * Note this call can be asynchronous - if any of the sprites' images has not loaded
	 * yet, that will delay this whole call until all of the images have loaded.
	 *
	 * TODO: Alternatively, split the sprites: ones with loaded images and one set
	 * per unique image. Load each set as soon as ready.
	 *
	 */
	multiAdd(sprites) {
		// Skip already added symbols
		sprites = sprites.filter((s) => !s._inAcetate);
		if (sprites.length === 0) {
			return;
		}

		sprites.forEach((s) => s.updateRefs(this, undefined, undefined));

		Promise.all(sprites.map((s) => s.image))
			.then((resolved) => resolved.map(imagifyFetchResponse))
			.then((loadedImages) => this._syncMultiAdd(sprites, loadedImages))
			.catch((err) => {
				/**
				 * @event spriteerror: Event
				 * Fired when some of the sprites to be added to this acetate have failed
				 * to load their image.
				 */
				this.fire("spriteerror", err);

				throw err;
			});

		return this;
	}

	// Must return a plain array with all the StridedTypedArrays that a symbol
	// might need, as well as any other (pseudo-)constants that the symbol's
	// `_setGlobalStrides()` method might need.
	// Can be overwritten by subclasses.
	_getStridedArrays(maxVtx, maxIdx) {
		return [
			// UV
			this._attrs.asStridedArray(maxVtx),
			// Extrusion
			this._extrusions.asStridedArray(maxVtx),
			// Index buffer
			this._indices.asTypedArray(maxIdx),
			// Texture size (width and height), in texels
			this._texSize,
		];
	}

	_syncMultiAdd(sprites, loadedImages) {
		// Calculate which symbols did not get removed before their image promise resolved
		const actualSprites = sprites.filter((s) => s._inAcetate === this);
		if (actualSprites.length === 0) {
			return;
		}

		const idxLength = actualSprites.reduce((acc, ext) => acc + ext.idxLength, 0);
		const attrLength = actualSprites.reduce((acc, ext) => acc + ext.attrLength, 0);

		let baseVtx = this._attribAllocator.allocateBlock(attrLength);
		let baseIdx = this._indices.allocateSlots(idxLength);
		let vtxAcc = baseVtx;
		let idxAcc = baseIdx;
		let maxVtx = baseVtx + attrLength;
		let stridedArrays = this._getStridedArrays(maxVtx, baseIdx + idxLength);

		sprites.forEach((s, i) => {
			// Skip those symbols that got removed before their image promise resolved
			// (skipping the loaded image as well; using the `actualSprites` array
			// would use references to stale images corresponding to unloaded sprites)
			if (s._inAcetate !== this) {
				return;
			}

			s.updateRefs(this, vtxAcc, idxAcc);
			this._knownSymbols[vtxAcc] = s;
			vtxAcc += s.attrLength;
			idxAcc += s.idxLength;

			// s.bin = this.pack(s.image);
			const img = loadedImages[i];
			s.bin = this.pack(img);

			s._normalizeAnchor();

			s._setGlobalStrides(...stridedArrays);
		});

		this._commitStridedArrays(baseVtx, attrLength);
		this._indices.commit(baseIdx, idxLength);

		if (this._crs) {
			this.reproject(baseVtx, attrLength, actualSprites);
		}

		// The AcetateInteractive will assign IDs to symbol vertices.
		super.multiAddIds(actualSprites, baseVtx);

		// AcetateExtrudedPoint also implements allocation logic, so its
		// implementation has to be skipped - go directly to base Acetate so
		// it fires the "symbolsadded" event.
		Acetate.prototype.multiAdd.call(this, actualSprites);

		this.dirty = true;
	}

	// The map will call resize() on acetates when needed - besides redoing the
	// framebuffer with the new size, this needs to reset the uniform uPixelSize.
	resize(w, h) {
		super.resize(w, h);
		const dpr2 = (devicePixelRatio ?? 1) * 2;
		this._programs.setUniform("uPixelSize", [dpr2 / w, dpr2 / h]);
	}

	/**
	 * @method remove(sprite: Sprite): this
	 * Removes the sprite from this acetate (so it's *not* drawn on the next refresh).
	 *
	 * Also decreases the usage count of the relevant portion of the atlas.
	 */
	remove(sprite) {
		if (sprite.attrBase === undefined) {
			// It's possible that the sprite's image is not ready yet
			/// TODO: Abort the image request. This means also replacing
			/// the `imagePromise` with an `abortableImagePromise`, but what
			/// about the image cache?
			return this;
		}

		const refcount = this._packer.unref(sprite.bin);

		if (refcount === 0) {
			sprite.image.then((img) => this._images.delete(img));
		}

		super.remove(sprite);

		return this;
	}

	/**
	 * @method debugAtlasIntoCanvas(canvas: HTMLCanvasElement): this
	 *
	 * Dumps the contents of the sprite atlas into the given `<canvas>`.
	 *
	 * This is an expensive operation and is meant only for debugging purposes.
	 */
	debugAtlasIntoCanvas(canvas) {
		const [maxh, maxw] = Object.values(this._packer.bins).reduce(
			([h, w], bin) => [Math.max(h, bin.y + bin.h), Math.max(w, bin.x + bin.w)],
			[0, 0]
		);

		// console.log(maxh, maxw, this._packer);

		if (maxh === 0 && maxw === 0) {
			return;
		}

		const data = this._atlas.asImageData(0, 0, maxw, maxh);
		canvas.width = maxw;
		canvas.height = maxh;
		canvas.getContext("2d").putImageData(data, 0, 0);

		return this;
	}

	/**
	 * @method debugAtlasIntoConsole(): this
	 *
	 * Dumps the contents of the sprite atlas into the developer tools' console,
	 * with some `<canvas>` and `console.log("%c")` trickery.
	 *
	 * This is an expensive operation and is meant only for debugging purposes.
	 */
	debugAtlasIntoConsole() {
		let canvas = document.createElement("canvas");
		this.debugAtlasIntoCanvas(canvas);

		// canvas.toBlob((blob)=>{
		// let url = URL.createObjectURL(blob);
		let url = canvas.toDataURL();

		console.log(
			"%c+",
			`
		font-size: 1px;
		padding: ${Math.floor(canvas.height / 2)}px ${Math.floor(canvas.width / 2)}px;
		line-height: ${canvas.height}px;
		background: url(${url});
		background-size: ${canvas.width}px ${canvas.height}px;
		color: transparent;
		`
		);

		return this;
	}
}

/**
 * @class Sprite
 * @inherits ExtrudedPoint
 * @relationship drawnOn AcetateSprite
 * @relationship compositionOf Bin, 1..1, 0..1
 *
 * A rectangular image, displayed at a constant screen ratio.
 *
 * Synonym of "marker"s and "icon"s.
 *
 * Works with point geometries only.
 *
 * @example
 * ```js
 * new Sprite([0, 0], {
 * 	image: "img/marker.png",
 * 	spriteAnchor: [13, 41]
 * }).addTo(map);
 * ```
 */

export default class Sprite extends ExtrudedPoint {
	/// @section Static properties
	/// @property Acetate: Prototype of AcetateSprite
	// The `Acetate` class that draws this symbol.
	static Acetate = AcetateSprite;

	#spriteStart;
	#spriteScale;
	#yaw;
	#image;

	/**
	 * @constructor Sprite(geom: Geometry, opts?: Sprite Options)
	 * @alternative
	 * @constructor Sprite(geom: Array of Number, opts?: Sprite Options)
	 */
	constructor(
		geom,
		{
			/**
			 * @section
			 * @aka Sprite Options
			 * @option image: HTMLImageElement
			 * The image for the sprite. When given an instance of `HTMLImageElement`, the
			 * image should be completely loaded.
			 * @alternative
			 * @option image: Promise to HTMLImageElement
			 * It might be convenient to instantiate a `Sprite` without knowning if its image
			 * has been loaded. Display of the sprite will be delayed until this `Promise`
			 * resolves.
			 * @alternative
			 * @option image: String
			 * For convenience, a `String` containing a URL can be passed.
			 * @alternative
			 * @option image: URL
			 * For convenience, a `URL` instance can be passed.
			 * @alternative
			 * @option image: Promise to Response
			 * For convenience, the result of a `fetch` call can be passed.
			 */
			image,

			/**
			 * @option spriteAnchor: Array of Number
			 * The coordinates of the pixel which shall display directly on the
			 * sprite's geometry (think "the tip of the pin").
			 *
			 * These coordinates are to be given in `[x, y]` form, in image pixels
			 * relative to the top-left corner of the sprite image.
			 *
			 * Negative values are interpreted as relative to the bottom-right
			 * corner of the image, instead. For this purpose, `+0` and `-0`
			 * are handled as different numbers.
			 * @alternative
			 * @option spriteAnchor: Array of String = ["50%","50%"]
			 * The sprite anchor can be given as two numeric strings ending
			 * with a percent sign (e.g. `["50%", "100%"]`). Anchor percentages
			 * are relative to the size of the image.
			 */
			spriteAnchor = ["50%", "50%"],

			/**
			 * @option spriteStart: Array of Number = [0, 0]
			 * When the image for a `Sprite` is a spritesheet, this is the top-left
			 * offset of the sprite within the spritesheet.
			 *
			 * This is to be given in `[x, y]` form, in image pixels. Defaults to `[0, 0]`,
			 * the top-left corner of the image.
			 */
			spriteStart = [0, 0],

			/**
			 * @option spriteSize: Array of Number = *
			 * The size of the sprite, in `[x, y]` form, in image pixels. Defaults to the
			 * size of the image.
			 *
			 * When the image for a `Sprite` is a spritesheet, this should be set to the
			 * size of the sprite (always smaller than the image itself).
			 */
			spriteSize,

			/**
			 * @option spriteScale: Number = 1
			 * Scale factor between image pixels and CSS pixels. Use `0.5`
			 * (or rather, `1/window.devicePixelRatio`) for "hi-DPI" icons,
			 * or `2` to double the size of the sprite.
			 */
			spriteScale = 1,

			/**
			 * @option yaw: Number = 0
			 * Yaw rotation of the sprite, relative to the "up" direction of the
			 * map's `<canvas>` (**not** relative to the direction of the CRS's `y`
			 * coordinate or "northing"), in clockwise degrees.
			 */
			yaw = 0,
			...opts
		} = {}
	) {
		super(geom, opts);

		/// TODO: Accept HTMLCanvasElement as image
		/// TODO: Accept ImageBitmap as image
		/// See https://developer.mozilla.org/en-US/docs/Web/API/WebGLRenderingContext/texImage2D.html#pixels

		this.#image =
			image instanceof Promise
				? image.then(imagifyFetchResponse)
				: imagePromise(image);

		this._anchor = spriteAnchor;
		this.#spriteStart = spriteStart;
		this._spriteSize = spriteSize;
		this.#spriteScale = spriteScale;
		this.#yaw = yaw;

		// Sprites are *always* two triangles (6 primitive slots) from 4 vertices
		this.attrLength = 4;
		this.idxLength = 6;

		/**
		 * @section Acetate interface
		 * @uninheritable
		 * @property bin: Bin
		 * An array of the form `[x1,y1, x2,y2]`, containing the coordinates
		 * of this sprite's image within the acetate's atlas texture.
		 * @property image: HTMLImageElement
		 * The actual image for this sprite. The acetate will copy it into
		 * a texture atlas.
		 */
		this.bin = undefined;
	}

	/**
	 * @section
	 * @property yaw: Number
	 * Runtime value of the `yaw` option: the yaw rotation of the sprite,
	 * in clockwise degrees. Can be updated.
	 */
	set yaw(yaw) {
		this.#yaw = yaw;
		this._refreshExtrusion();
	}
	get yaw() {
		return this.#yaw;
	}

	/**
	 * @property spriteScale: Number
	 * Runtime value of the `spriteScale` option: image pixel to CSS pixel
	 * ratio. Can be updated.
	 */
	set spriteScale(s) {
		this.#spriteScale = s;
		this._refreshExtrusion();
	}
	get spriteScale() {
		return this.#spriteScale;
	}

	/**
	 * @property image: Promise to HTMLImageElement
	 * A `Promise` to the (possibly not loaded yet) image for this sprite.
	 *
	 * Read-only. Use the `replaceImage` method to change the sprite's image
	 * and associated parameters.
	 */
	get image() {
		return this.#image;
	}

	/**
	 * @method replaceImage(opts: Sprite Options, retain: Boolean = false): Promise of this
	 *
	 * Replaces the sprite's image and associated parameters (sprite size,
	 * anchor, origin, scale). The first parameter is an object containing
	 * constructor options (such as `image`, `spriteAnchor`, etc).
	 *
	 * By default, the old image gets expelled from the acetate's texture atlas.
	 * In order to prevent that, set `retain` to true. This is useful in
	 * scenarios where replacing the images of several `Sprite`s in the same
	 * render frame causes artifacts such as the wrong image being displayed. Avoid
	 * retaining large images.
	 *
	 * Returns a `Promise` that resolves when the image has been loaded.
	 */
	async replaceImage(
		{ image, spriteAnchor, spriteStart, spriteSize, spriteScale, yaw } = {},
		retain = false
	) {
		let oldImage = this.#image;
		let i = (this.#image =
			image instanceof Promise
				? image.then(imagifyFetchResponse)
				: imagePromise(image));

		this._anchor = spriteAnchor ?? this._anchor;
		this.#spriteStart = spriteStart ?? this.#spriteStart;
		this._spriteSize = spriteSize;
		this.#spriteScale = spriteScale ?? this.#spriteScale;
		this.#yaw = yaw ?? this.#yaw;

		const ac = this._inAcetate;
		if (!ac) {
			// If the sprite is not in an acetate, skip replacement.
			return this;
		}

		let [loadedImg, loadedOldImg, [allocAttr, allocIdx]] = await Promise.all([
			this.#image,
			oldImage,
			this.allocation,
		]);

		// console.log(loadedImg, loadedOldImg, _allocation);

		if (i !== this.#image) {
			// Image has already been replaced before it could load
			return;
		}
		if (allocAttr !== this.attrBase || allocIdx !== this.idxBase) {
			throw new Error("Sprite reallocated during image load");
		}

		// ac.debugAtlasIntoConsole();

		if (!retain) {
			// Manually unpack the old image from the acetate's atlas. It's important
			// to **not** call the parent functionality (i.e. do not call the
			// remove interactive symbol code)
			const refcount = ac._packer.unref(this.bin);
			if (refcount === 0) {
				ac._images.delete(loadedOldImg);
			}
		}
		this.bin = ac.pack(loadedImg);
		this._normalizeAnchor();

		let strides = ac._getStridedArrays(
			this.attrBase + this.attrLength,
			this.idxBase + this.idxLength
		);
		this._setGlobalStrides(...strides);
		ac._commitStridedArrays(this.attrBase, this.attrLength);
		ac.dirty = true;

		// ac.debugAtlasIntoConsole();

		return this;
	}

	/**
	 * @section Acetate interface
	 * @uninheritable
	 * @method _setGlobalStrides(strideUV: StridedTypedArray, strideExtrude: StridedTypedArray, texSize: Number): this
	 * Sets the appropriate values in the `StridedTypedArray`s.
	 */
	_setGlobalStrides(strideUV, strideExtrude, typedIdxs, texSize) {
		// Texture bounds, normalized 0..1 texture position
		// ss = SpriteStart
		const ssx = this.bin.x + this.#spriteStart[0];
		const ssy = this.bin.y + this.#spriteStart[1];

		// se = SpriteEnd
		const sex = ssx + this._spriteSize[0];
		const sey = ssy + this._spriteSize[1];

		// Texture coords are normalized into 0..1, relative to the size
		// of the texture atlas.
		const tx1 = ssx / texSize;
		const ty1 = ssy / texSize;
		const tx2 = sex / texSize;
		const ty2 = sey / texSize;

		// `strideUV` comes from a `SingleAttribute`, so values for the four
		// vertices could be concatenated together.
		// prettier-ignore
		strideUV.set([tx1, ty1], this.attrBase);
		strideUV.set([tx1, ty2], this.attrBase + 1);
		strideUV.set([tx2, ty2], this.attrBase + 2);
		strideUV.set([tx2, ty1], this.attrBase + 3);

		// prettier-ignore
		typedIdxs.set([
			this.attrBase, this.attrBase + 1, this.attrBase + 2,
			this.attrBase, this.attrBase + 2, this.attrBase + 3,
		], this.idxBase);

		return this._setStridedExtrusion(strideExtrude);
	}

	_setStridedExtrusion(strideExtrude) {
		const s = this.#spriteScale;
		const x1 = s * -this._anchor[0];
		const y1 = s * this._anchor[1];
		const x2 = x1 + s * this._spriteSize[0];
		const y2 = y1 - s * this._spriteSize[1];
		const [offsetX, offsetY] = this.offset;
		let offsets;

		if (this._yaw === 0) {
			// prettier-ignore
			offsets = [
				offsetX + x1, offsetY + y1,
				offsetX + x1, offsetY + y2,
				offsetX + x2, offsetY + y2,
				offsetX + x2, offsetY + y1,
			];
		} else {
			const yawRadians = (-this.#yaw * Math.PI) / 180;
			const s = Math.sin(yawRadians);
			const c = Math.cos(yawRadians);

			// prettier-ignore
			offsets = [
				offsetX + x1*c-y1*s, offsetY + x1*s+y1*c,
				offsetX + x1*c-y2*s, offsetY + x1*s+y2*c,
				offsetX + x2*c-y2*s, offsetY + x2*s+y2*c,
				offsetX + x2*c-y1*s, offsetY + x2*s+y1*c,
			];
		}
		strideExtrude.set(offsets, this.attrBase);

		return this;
	}

	// Ensures the anchor values are image pixels, in particular when given
	// as percentage strings.
	// Also handles default values for the sprite size.
	_normalizeAnchor() {
		if (!this._spriteSize) {
			this._spriteSize = [this.bin.w, this.bin.h];
		}

		// Sprite anchor expressed as percentage
		if (typeof this._anchor[0] === "string") {
			const anchorXRegexp = this._anchor[0].match(percentageRegexp);
			if (anchorXRegexp) {
				this._anchor[0] = (this.bin.w * anchorXRegexp[1]) / 100;
			} else {
				this._anchor[0] = Number(this._anchor[0]);
			}
		}
		if (typeof this._anchor[1] === "string") {
			const anchorYRegexp = this._anchor[1].match(percentageRegexp);
			if (anchorYRegexp) {
				this._anchor[1] = (this.bin.h * anchorYRegexp[1]) / 100;
			} else {
				this._anchor[1] = Number(this._anchor[1]);
			}
		}

		// Sprite anchor expressed as negative - use bottom-left instead of
		// top-right
		// Note use of `Object.is()` instead of `===` - needed to tell apart
		// `+0` and `-0`.
		if (this._anchor[0] < 0 || Object.is(this._anchor[0], -0)) {
			this._anchor[0] = this.bin.w + this._anchor[0];
		}
		if (this._anchor[1] < 0 || Object.is(this._anchor[1], -0)) {
			this._anchor[1] = this.bin.h + this._anchor[1];
		}
	}

	_refreshExtrusion() {
		if (!this._inAcetate || this.attrBase === undefined) {
			return this;
		}

		let strideExtrude = this._inAcetate._extrusions.asStridedArray();
		this._setStridedExtrusion(strideExtrude);

		this._inAcetate._extrusions.commit(this.attrBase, this.attrLength);
		this._inAcetate.dirty = true;
		return this;
	}
}
