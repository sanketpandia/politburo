import { default as typeMap } from "./util/typeMap.mjs";
import { default as reverseTypeMap } from "./util/reverseTypeMap.mjs";
import FrameBuffer from "./FrameBuffer/FrameBuffer.mjs";

/// TODO: Subclass? from HTMLImageElement
/// TODO: Subclass? from HTMLVideoElement

/// TODO: pixelStorei, from
/// https://developer.mozilla.org/en-US/docs/Web/API/WebGLRenderingContext/pixelStorei
/// (e.g. flip Y coordinate)

/**
 * @class Texture
 * @inherits AbstractFrameBufferAttachment
 * Wraps a [`WebGLTexture`](https://developer.mozilla.org/docs/Web/API/WebGLTexture)
 * and offers convenience methods.
 *
 * Contrary to what one might think, a `Texture` has no implicit size. Its size
 * is (re-)defined on any full update (`texImage2D` or `linkRenderBuffer`).
 */

import { registerFactory } from "./GliiFactory.mjs";

export default class Texture {
	#gl;
	#tex;

	constructor(gl, opts = {}) {
		this.#gl = gl;
		this.#tex = gl.createTexture();

		/**
		 * @section
		 * @aka Texture options
		 * @option minFilter: Texture interpolation constant = gl.NEAREST
		 * Initial value of the `minFilter` property
		 * @option magFilter: Texture interpolation constant = gl.NEAREST
		 * Initial value of the `magFilter` property
		 * @option wrapS: Texture wrapping constant = gl.CLAMP_TO_EDGE
		 * Initial value for the `wrapS` property
		 * @option wrapT: Texture wrapping constant = gl.CLAMP_TO_EDGE
		 * Initial value for the `wrapS` property
		 * @option internalFormat: Texture format constant = gl.RGBA
		 * Initial value for the `internalFormat` property
		 * @option format: Texture format constant = gl.RGBA
		 * Initial value for the `format` property
		 * @option type: Texture type constant = gl.UNSIGNED_BYTE
		 * Initial value for the `type` property
		 */

		// Helper for caching bound textures and their active texture unit
		this._unit = undefined;

		// Helper for LRU-ing texture units. Shall be (re-)set every time
		// this texture is promoted to an available unit
		this._lastActive = performance.now();

		// @property minFilter: Texture interpolation constant = glii.NEAREST
		// Texture minification filter (or "what to do when pixels in the texture
		// are smaller than pixels in the output image")
		this.minFilter = opts.minFilter || gl.NEAREST;

		// @property magFilter: Texture interpolation constant = glii.NEAREST
		// Texture magification filter (or "what to do when pixels in the texture
		// are bigger than pixels in the output image"). Cannot use mipmaps (as
		// mipmaps are always smaller than the texture).
		this.magFilter = opts.magFilter || gl.NEAREST;

		// @property wrapS: Texture wrapping constant
		// Value for the `TEXTURE_WRAP_S` [texture parameter](https://developer.mozilla.org/en-US/docs/Web/API/WebGLRenderingContext/texParameter) for any subsequent texture full updates
		this.wrapS = opts.wrapS || gl.CLAMP_TO_EDGE;

		// @property wrapT: Texture wrapping constant
		// Value for the `TEXTURE_WRAP_T` [texture parameter](https://developer.mozilla.org/en-US/docs/Web/API/WebGLRenderingContext/texParameter) for any subsequent texture full updates
		this.wrapT = opts.wrapT || gl.CLAMP_TO_EDGE;

		// @property internalFormat: Texture format constant
		// Value for the `internalFormat` parameter for `texImage2D` calls.
		this.internalFormat = opts.internalFormat || gl.RGBA;

		// @property format: Texture format constant
		// Value for the `format` parameter for `texImage2D` calls. in WebGL1, this must
		// be equal to `internalFormat`. For WebGL2, see
		// https://registry.khronos.org/webgl/specs/latest/2.0/#TEXTURE_TYPES_FORMATS_FROM_DOM_ELEMENTS_TABLE
		this.format = opts.format || gl.RGBA;

		// @property type: Texture pixel type constant
		// Value for the `type` parameter for `texImage2D` calls.
		this.type = opts.type || gl.UNSIGNED_BYTE;

		// Loaded flag, to let `FrameBuffer` know when the texture has to be init'd
		// with a specific width/height
		this._isLoaded = false;

		this.width = undefined;
		this.height = undefined;
	}

	// Each element of this `WeakMap` is keyed by a `WebGLRenderingContext`, and its value
	// is a plain `Array` of `Texture`s.
	static _boundUnits = new WeakMap();

	/**
	 * @property tex: WebGLTexture
	 * The underlying instance of `WebGLTexture`. Read-only.
	 */
	get tex() {
		if (!this.#tex) {
			throw new Error("Texture has been destroyed and cannot be used");
		}
		return this.#tex;
	}

	/**
	 * @section Internal methods
	 * @method getUnit(): Number
	 * Returns a the texture unit index (or "name" in GL parlance) that this texture
	 * is bound to.
	 *
	 * Calling this method guarantees that the texture is bound into a valid unit,
	 * and that that unit is the active one (until a number of other `Texture`s
	 * call `getUnit()`, at least `MAX_COMBINED_TEXTURE_IMAGE_UNITS`)
	 *
	 * This might expel (unbind) the texture which was used the longest ago.
	 */
	getUnit() {
		this._lastActive = performance.now();
		const gl = this.#gl;
		if (this._unit !== undefined) {
			gl.activeTexture(gl.TEXTURE0 + this._unit);
			// 			console.log("Texture already bound to unit", this._unit);
			return this._unit;
		}

		if (!Texture._boundUnits.has(this.#gl)) {
			const maxUnits = this.#gl.getParameter(
				this.#gl.MAX_COMBINED_TEXTURE_IMAGE_UNITS
			);
			Texture._boundUnits.set(this.#gl, new Array(maxUnits));
		}
		const units = Texture._boundUnits.get(this.#gl);

		let oldestUnit = -1;
		let oldestTime = Infinity;
		for (let i = 0, l = units.length; i < l; i++) {
			if (units[i] === undefined) {
				// 				console.log("Texture newly bound to unit", i);
				gl.activeTexture(gl.TEXTURE0 + i);
				gl.bindTexture(gl.TEXTURE_2D, this.#tex);
				units[i] = this;
				return (this._unit = i);
			}
			if (units[i]._lastActive < oldestTime) {
				oldestUnit = i;
				oldestTime = units[i]._lastActive;
			}
		}
		// 		console.log("Expelled texture to bound to unit", oldestUnit);
		gl.activeTexture(gl.TEXTURE0 + oldestUnit);
		gl.bindTexture(gl.TEXTURE_2D, this.#tex);
		units[oldestUnit]._unit = undefined;
		units[oldestUnit] = this;
		return (this._unit = oldestUnit);
	}

	/**
	 * @method unbind(): this
	 * Forcefully unbinds the texture
	 */
	unbind() {
		if (this._unit === undefined) {
			return;
		}
		const units = Texture._boundUnits.get(this.#gl);
		units[this._unit] = undefined;
		this.#gl.activeTexture(this.#gl.TEXTURE0 + this._unit);
		this.#gl.bindTexture(this.#gl.TEXTURE_2D, undefined);
		this._unit = undefined;
		return this;
	}

	_resetParameters() {
		const gl = this.#gl;

		gl.bindTexture(gl.TEXTURE_2D, this.#tex);

		gl.texParameteri(gl.TEXTURE_2D, gl.TEXTURE_MIN_FILTER, this.minFilter);
		gl.texParameteri(gl.TEXTURE_2D, gl.TEXTURE_MAG_FILTER, this.magFilter);
		gl.texParameteri(gl.TEXTURE_2D, gl.TEXTURE_WRAP_S, this.wrapS);
		gl.texParameteri(gl.TEXTURE_2D, gl.TEXTURE_WRAP_T, this.wrapT);
	}

	/**
	 * @section
	 * @method setParameters: this
	 * @param minFilter?: Texture interpolation constant
	 * @param maxFilter?: Texture interpolation constant
	 * @param wrapS?: Texture wrapping constant
	 * @param wrapT?: Texture wrapping constant
	 * Resets the interpolation and wrapping parameters to the given ones.
	 * Any value can be `undefined` (and if so, won't be updated)
	 */
	setParameters(minFilter, maxFilter, wrapS, wrapT) {
		this.minFilter = minFilter ?? this.minFilter;
		this.maxFilter = maxFilter ?? this.maxFilter;
		this.wrapS = wrapS ?? this.wrapS;
		this.wrapT = wrapT ?? this.wrapT;

		this._resetParameters();
		return this;
	}

	/**
	 * @section
	 * @method texImage2D(img: HTMLImageElement): this
	 * (Re-)sets the texture contents to a copy of the given image. This is considered a "full update" of the texture.
	 *
	 * If the texture's format is other than `RGBA`, some data might be dropped (e.g.
	 * putting an RGBA `HTMLImageElement` with an alpha channel into a texture with `RGB`
	 * format shall drop the alpha channel).
	 * @alternative
	 * @method texImage2D(img: HTMLCanvasElement): this
	 * @alternative
	 * @method texImage2D(img: ImageData): this
	 * @alternative
	 * @method texImage2D(img: HTMLVideoElement): this
	 * @alternative
	 * @method texImage2D(img: ImageBitmap): this
	 */
	texImage2D(img) {
		/// TODO: set as dirty (?)
		/// TODO: notify programs using this texture that they're dirty now
		/// TODO: Read width/height from the image?? Then set as .width / .height
		const gl = this.#gl;
		this._isLoaded = true;
		this.width = img.width;
		this.height = img.height;
		this.getUnit();
		gl.texImage2D(gl.TEXTURE_2D, 0, this.internalFormat, this.format, this.type, img);
		this._resetParameters();

		this.#generateMipmap();
		return this;
	}

	/**
	 * @section
	 * @method texSubImage2D(img: HTMLImageElement, x: Number, y: Number): this
	 * (Re-)sets a portion of the texture contents to a copy of the given image. The portion
	 * starts at the given `x` and `y` coordinates, and is as big as the image.
	 *
	 * Otherwise, same as `texImage2D`.
	 * @alternative
	 * @method texSubImage2D(img: HTMLCanvasElement, x: Number, y: Number): this
	 * @alternative
	 * @method texSubImage2D(img: ImageData, x: Number, y: Number): this
	 * @alternative
	 * @method texSubImage2D(img: HTMLVideoElement, x: Number, y: Number): this
	 * @alternative
	 * @method texSubImage2D(img: ImageBitmap, x: Number, y: Number): this
	 */
	texSubImage2D(img, x, y) {
		const gl = this.#gl;
		this._isLoaded = true;

		this.getUnit();
		gl.texSubImage2D(gl.TEXTURE_2D, 0, x, y, this.internalFormat, this.type, img);
		this._resetParameters();

		this.#generateMipmap();
		return this;
	}

	/**
	 * @method texArray(w: Number, h: Number, arr: ArrayBufferView): this
	 * (Re-)sets the texture contents to a copy of the given `ArrayBufferView`
	 * (typically a `TypedArray` fitting this texture's `type`/`format`).
	 * Must be given width and height as well.
	 * @alternative
	 * @method texArray(w: Number, h: Number, arr: null): this
	 * Zeroes out the texture.
	 */
	texArray(w, h, arr) {
		this.getUnit();
		const gl = this.#gl;

		this._isLoaded = true;
		this.width = w;
		this.height = h;

		// 		gl.texImage2D(target, level, internalformat, width, height, border, format, type, ArrayBufferView? pixels);

		if (arr === null || reverseTypeMap.get(this.type) !== arr.constructor) {
			throw new Error("Passed TypedArray doesn't match the texture's pixel type ");
		}

		gl.texImage2D(
			gl.TEXTURE_2D,
			0,	// level
			this.internalFormat,
			w,
			h,
			0,	// border
			this.format,
			this.type,
			arr
		);
		this._resetParameters();

		this.#generateMipmap();
		return this;
	}


	/**
	 * @method texSubArray(w: Number, h: Number, arr: ArrayBufferView, x: Number, y:Number): this
	 *
	 * (Re-)sets a portion of the texture contents to a copy of the given
	 * `ArrayBufferView` (typically a `TypedArray` fitting this texture's
	 * `type`/`format`).
	 *
	 * Must be given width and height as well.
	 */
	texSubArray(w, h, arr, x, y) {
		this.getUnit();
		const gl = this.#gl;

		this._isLoaded = true;

		if (arr === null || reverseTypeMap.get(this.type) !== arr.constructor) {
			throw new Error("Passed TypedArray doesn't match the texture's pixel type ");
		}

		gl.texSubImage2D(
			gl.TEXTURE_2D,
			0,	// level
			x,
			y,
			w,
			h,
			this.format,
			this.type,
			arr
		);
		this._resetParameters();

		this.#generateMipmap();
		return this;
	}





	#generateMipmap() {
		const gl = this.#gl;
		if (
			(this.minFilter === gl.NEAREST || this.minFilter === gl.LINEAR) &&
			(this.magFilter === gl.NEAREST || this.magFilter === gl.LINEAR)
		) {
			return;
		}
		gl.generateMipmap(gl.TEXTURE_2D);
	}

	/**
	 * @method isLoaded(): Boolean
	 * Returns whether the texture has been initialized with any data at all. `true` after `texImage2D()` and the like.
	 */
	isLoaded() {
		return this._isLoaded;
	}

	/**
	 * @method getComponentsPerTexel(): Number
	 * Returns the number of components per texel, based on the `format` property.
	 * (e.g. 3 for `RGB`, 4 for `RGBA`, etc).
	 */
	getComponentsPerTexel() {
		const gl = this.#gl;
		switch (this.format) {
			case gl.RGBA:
			case gl.RGBA_INTEGER:
				return 4;
			case gl.RGB:
			case gl.RGB_INTEGER:
				return 3;
			case gl.LUMINANCE_ALPHA:
			case gl.RG:
			case gl.RG_INTEGER:
				return 2;
			case gl.LUMINANCE:
			case gl.ALPHA:
			case gl.RED:
			case gl.RED_INTEGER:
				return 1;
			default:
				throw new Error("Unknown texel data format");
		}
	}

	/**
	 * @method asImageData(x:Number, y:Number, w:Number, h:Number): ImageData
	 * Returns an instance of `ImageData` with a copy of the current contents
	 * of the texture.
	 *
	 * Note this is **not** a performant method call (it creates and destroys
	 * an interim `FrameBuffer`) and is meant for debugging purposes only (i.e.
	 * dumping a texture to a `HTMLCanvasElement` via [`putImageData()`](https://developer.mozilla.org/en-US/docs/Web/API/CanvasRenderingContext2D/putImageData.html)).
	 *
	 * `x` and `y` specify the start of the dump (in texels); `w` and `h` specify
	 * the width and height of the dump. When not specified, the whole texture is
	 * dumped.
	 */
	asImageData(x, y, w, h) {
		if (!this._isLoaded) {
			throw new Error(
				"Must load something into the Texture before calling asImageData()"
			);
		}
		const gl = this.#gl;
		if (this.format !== gl.RGBA && this.internalFormat !== gl.R32F) {
			throw new Error(
				"asImageData() only available for textures with RGBA8 or R32F format"
			);
		}

		const fb = new FrameBuffer(gl, {
			width: this.width,
			height: this.height,
			colour: [this],
		});

		let pixels = fb.readPixels(x, y, w, h);

		if (this.internalFormat === gl.R32F) {
			// Due to WebGL shenanigans, reading a R32F texture has to create
			// a RGBA32F texture. Filter the GBA components to leave just the R
			// component.
			const size = this.width * this.height;
			const redComponent = new Float32Array(size);
			for (let i = 0; i < size; i++) {
				redComponent[i] = pixels[i * 4];
			}
			pixels = redComponent;
		}

		const imagedata = new ImageData(
			new Uint8ClampedArray(pixels.buffer),
			w ?? this.width,
			h ?? this.height
		);
		fb.destroy();

		return imagedata;
	}

	/**
	 * @method debugIntoCanvas(canvas: HTMLCanvasElement): this
	 *
	 * Convenience wrapper around `asImageData()`. Automates fetching a 2D context
	 * from the given `HTMLCanvasElement`, resizing it, and running `putImageData()`.
	 *
	 * The texture might be inverted in the Y-axis (see `UNPACK_FLIP_Y_WEBGL`), it is
	 * suggested to flip the destination canvas as well with a `transform:scaleY(-1)`
	 * CSS rule.
	 */
	debugIntoCanvas(canvas) {
		const data = this.asImageData();
		canvas.width = this.width;
		canvas.height = this.height;
		canvas.getContext("2d").putImageData(data, 0, 0);
		return this;
	}

	/**
	 * @method destroy(): this
	 * Tells WebGL to free resources associated with this `Texture`. Use
	 * when the `Texture` won't be used anymore.
	 *
	 * After being destroyed, WebGL programs should not use the destroyed `Texture`,
	 * not any `FrameBuffer` pointing to the destroyed texture.
	 */
	destroy() {
		this.#gl.deleteTexture(this.#tex);
		this.#tex = undefined;
	}
}

/**
 * @factory GliiFactory.Texture(options: Texture options)
 *
 * @class Glii
 * @section Class wrappers
 * @property Texture(options: Texture options): Prototype of Texture
 * Wrapped `Texture` class
 */
registerFactory("Texture", function (gl, glii) {

	/**
	 * @class Glii
	 * @section
	 * @method flushTextureUnits(): this
	 * Removes the cached data about texture units, forcing all textures to
	 * re-bind their units.
	 *
	 * Useful when the WebGL context is shared with some other library that *may*
	 * move texture units around.
	 */
	glii.flushTextureUnits = function forgetTextureUnits() {
		// Texture._boundUnits.delete(gl);
		if (!Texture._boundUnits.has(this.gl)) {
			return;
		}

		let units = Texture._boundUnits.get(this.gl);
		units.forEach((tex, unit)=>{
			tex?.unbind();
		});
		return this;
	}

	return class WrappedTexture extends Texture {
		constructor(opts) {
			super(gl, opts);
		}

		/**
		 * @class Texture
		 * @function getMaxSize(): Number
		 * Returns the maximum width/height that a `Texture` instance can have.
		 */
		static getMaxSize() {
			return gl.getParameter(gl.MAX_TEXTURE_SIZE);
		}
	};
});
