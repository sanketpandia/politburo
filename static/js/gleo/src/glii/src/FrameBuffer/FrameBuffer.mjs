import { default as reverseTypeMap } from "../util/reverseTypeMap.mjs";

/**
 * @class FrameBuffer
 * @relationship compositionOf AbstractFrameBufferAttachment, 0..n, 1..n
 *
 * Wraps a [`WebGLFramebuffer`](https://developer.mozilla.org/en-US/docs/Web/API/WebGLFramebuffer)
 * and offers convenience methods.
 *
 * A `FrameBuffer` is a collection of `Texture`s/`RenderBuffer`s: (at least) one for
 * colour (RGBA), an optional one for depth and an optional one for stencil.
 *
 * In GL parlance, each of the `Texture`/`RenderBuffer`s that make up a `FrameBuffer`
 * is called an "attachment". Each attachment must have an `internalFormat` fitting its
 * colour/depth/stencil role.
 *
 * In any operations that allow a `FrameBuffer`, not giving one (or explicitly setting it
 * to `null`) shall work on the "default" framebuffer - in the usual case, this means an
 * internally-created framebuffer with the colour attachment linked to the `<canvas>`
 * that the GL context was created out of.
 *
 * Multiple colour attachments are only possible in WebGL2, or in WebGL1 when the
 * [`WEBGL_draw_buffers`](https://developer.mozilla.org/en-US/docs/Web/API/WEBGL_draw_buffers) extension is available.
 *
 * Note that (in most combinations of drivers&hardware) the colour attachment(s) **must**
 * be textures using the `RGBA`/`UNSIGNED_BYTE` format+type combination.
 *
 * @example
 *
 * ```
 * var fb1 = new gliiFactory.FrameBuffer({
 * 	size: new XY(1024, 1024),
 * 	color: [new gliiFactory.Texture( ... )],
 * 	stencil: new gliiFactory.RenderBuffer( ... ),
 * 	depth: new gliiFactory.RenderBuffer( ... ),
 * });
 *
 * var size = new XY(1024, 1024);
 * var fb2 = new gliiFactory.FrameBuffer({
 * 	size: size,
 * 	color: [
 * 		new gliiFactory.Texture( size: size, ... ),
 * 		new gliiFactory.Texture( size: size, ... )
 * 	],
 * 	stencil: false,
 * 	depth: false,
 * });
 * ```
 */

/// TODO: depth format for textures, only available with extension:
/// https://developer.mozilla.org/en-US/docs/Web/API/WEBGL_depth_texture

import { registerFactory } from "../GliiFactory.mjs";
import Texture from "../Texture.mjs";
import RenderBuffer from "./RenderBuffer.mjs";

export default class FrameBuffer {
	#gl;
	#fb;

	#width;
	#height;

	#colourAttachs;
	#depth;
	#stencil;

	constructor(gl, opts = {}) {
		this.#gl = gl;
		this.#fb = gl.createFramebuffer();

		gl.bindFramebuffer(gl.FRAMEBUFFER, this.#fb);

		// @section
		// @aka FrameBuffer options
		// @option size: Array of Number
		// Width and height of this `FrameBuffer`, in pixels, as a 2-component array.
		// If specified, it overrides the `width` and `height` options.
		if ("size" in opts) {
			if ("width" in opts || "height" in opts) {
				throw new Error(
					'Expected either "size" or "width"/"height", but both were provided'
				);
			}
			this.#width = opts.size[0];
			this.#height = opts.size[1];
		} else {
			// @option width: Number = 256
			// Width of this `RenderBuffer`, in pixels.
			this.#width = opts.width || 256;

			// @option height: Number = 256
			// Height of this `RenderBuffer`, in pixels.
			this.#height = opts.height || 256;
		}

		// @option colour: Array of AbstractFrameBufferAttachment = []
		// An array of colour attachment(s) (either `Texture`s or `RenderBuffer`s)
		this.#colourAttachs = opts.colour || opts.color || [];
		// 		this.colourAttachZero = colourAttachs[0];
		this.#colourAttachs.forEach((att, i) => {
			if (att instanceof Texture) {
				att.getUnit();
				if (!att.isLoaded()) {
					// If the texture is empty (which means, no width/height set),
					// init it to a blank texture the same size as this FB.
					/// TODO: This will fail for pixel types UNSIGNED_SHORT_5_5_5_1 *et al*,
					/// since the size of the arrays and the components per pixel do not match.
					/// There is a need for an additional utility function to instantiate
					/// this kind of typed array.
					att.texArray(
						this.#width,
						this.#height,
						new (reverseTypeMap.get(att.type))(
							this.#width * this.#height * att.getComponentsPerTexel()
						)
					);

					// gl.texImage2D(gl.TEXTURE_2D, 0, this.internalFormat,this.#width, this.#height, 0, this.format, this.type, null);
				}
				gl.framebufferTexture2D(
					gl.FRAMEBUFFER,
					gl.COLOR_ATTACHMENT0 + i,
					gl.TEXTURE_2D,
					att.tex,
					0
				);
			} else if (att instanceof RenderBuffer) {
				gl.framebufferRenderbuffer(
					gl.FRAMEBUFFER,
					gl.COLOR_ATTACHMENT0 + i,
					gl.RENDERBUFFER,
					att.rb
				);
			}
		});

		// @option depth: RenderBuffer = false
		// A `RenderBuffer` for the depth attachment. Must have a depth-compatible `internalformat`.
		if (opts.depth && opts.depth instanceof RenderBuffer) {
			this.#depth = opts.depth;
			gl.framebufferRenderbuffer(
				gl.FRAMEBUFFER,
				gl.DEPTH_ATTACHMENT,
				gl.RENDERBUFFER,
				opts.depth.rb
			);
		}

		// @option stencil: RenderBuffer = false
		// A `RenderBuffer` for the stencil attachment. Must have a stencil-compatible `internalformat`.
		if (opts.stencil && opts.stencil instanceof RenderBuffer) {
			this.#stencil = opts.stencil;
			gl.framebufferRenderbuffer(
				gl.FRAMEBUFFER,
				gl.STENCIL_ATTACHMENT,
				gl.RENDERBUFFER,
				opts.stencil.rb
			);
		}

		this.#checkStatus();
	}

	/**
	 * @property fb: WebGLFramebuffer
	 * The underlying instance of `WebGLFramebuffer`. Read-only.
	 */
	get fb() {
		return this.#fb;
	}

	/**
	 * @property width: Number
	 * The width of the framebuffer (and all its attachments), in pixels. Read-only.
	 */
	get width() {
		return this.#width;
	}

	/**
	 * @property height: Number
	 * The height of the framebuffer (and all its attachments), in pixels. Read-only.
	 */
	get height() {
		return this.#height;
	}

	/**
	 * @method resize(x: Number, y: Number): this
	 * Sets a new size for the framebuffer's attachments (textures/renderbuffers),
	 * destroying their data in the process.
	 */
	resize(x, y) {
		this.#height = y;
		this.#width = x;

		this.#colourAttachs.forEach((att, _) => {
			att.getUnit();
			att.texArray(
				x,
				y,
				new (reverseTypeMap.get(att.type))(
					this.#width * this.#height * att.getComponentsPerTexel()
				)
			);
		});

		if (this.#depth && this.#depth instanceof RenderBuffer) {
			this.#depth.resize(x, y);
		}

		if (this.#stencil && this.#stencil instanceof RenderBuffer) {
			this.#depth.resize(x, y);
		}
		this.#checkStatus();

		return this;
	}

	/**
	 * @method destroy(): this
	 * Tells WebGL to free resources associated with this framebuffer. Use
	 * when the framebuffer won't be used anymore.
	 *
	 * After being destroyed, the framebuffer should not be used in a program.
	 * Does not destroy any associated textures or renderbuffers.
	 */
	destroy() {
		this.#gl.deleteFramebuffer(this.#fb);
	}

	#checkStatus() {
		const gl = this.#gl;
		const status = gl.checkFramebufferStatus(gl.FRAMEBUFFER);
		if (status === gl.FRAMEBUFFER_INCOMPLETE_ATTACHMENT) {
			// One reason for this is colour attachment being textures of a format/type
			// different than RGBA/UNSIGNED_BYTE. See
			// https://www.khronos.org/registry/webgl/extensions/WEBGL_draw_buffers/
			throw new Error(
				`The attachment types are mismatched or not all framebuffer attachment points are framebuffer attachment complete.
For valid format/type combinations of framebuffer attachments, see https://www.khronos.org/registry/webgl/specs/1.0/#6.6 and https://www.khronos.org/registry/webgl/extensions/WEBGL_draw_buffers/`
			);
		} else if (status === gl.FRAMEBUFFER_INCOMPLETE_MISSING_ATTACHMENT) {
			throw new Error("There is no attachment.");
		} else if (status === gl.FRAMEBUFFER_INCOMPLETE_DIMENSIONS) {
			throw new Error("Height and width of the attachment are not the same.");
		} else if (status === gl.FRAMEBUFFER_INCOMPLETE_MISSING_ATTACHMENT) {
			throw new Error("There is no attachment.");
		} else if (status === gl.FRAMEBUFFER_UNSUPPORTED) {
			throw new Error(
				"The format of the attachment is not supported, or depth and stencil attachments are not the same renderbuffer."
			);
		} else if (status !== gl.FRAMEBUFFER_COMPLETE) {
			throw new Error("FrameBuffer invalid " + status);
		}
	}

	/**
	 * @method readPixels: TypedArray
	 * @param x?: Number
	 * @param y?: Number
	 * @param width?: Number
	 * @return TypedArray
	 *
	 * Reads pixels from the colour attachment of the framebuffer, and returns a `TypedArray`
	 * (e.g. a `Uint8Array` for 8-bit RGBA textures) with the data.
	 *
	 * Defaults to reading the entire colour attachment (from `0,0` to its witdh-height),
	 * handles the datatypes, and creates a new `TypedArray` of the appropriate kind.
	 *
	 */
	/// TODO: How are float32 readbacks handled?? It seems that they neccesarily need an extension,
	/// but the documentation is scarce about the issue.

	readPixels(x, y, w, h) {
		const gl = this.#gl;

		x = x || 0;
		y = y || 0;
		w = w || this.#width;
		h = h || this.#height;

		const attach = this.#colourAttachs[0];
		let format, type, arrClass;
		let itemsPerPx = 1;

		if (attach instanceof Texture) {
			if (
				attach.internalFormat === gl.RGBA ||
				attach.internalFormat === gl.RGB ||
				attach.internalFormat === gl.ALPHA
			) {
				format = attach.internalFormat;
				itemsPerPx = attach.getComponentsPerTexel();
			} else if (attach.internalFormat === gl.LUMINANCE) {
				/// Untested!!!
				format = gl.RGB;
				itemsPerPx = 3;
			} else if (attach.internalFormat === gl.R32F) {
				// Needs WebGL2, or float texture extensions
				// This reads 4 floats instead of 1 float per pixel. But works.
				// Using gl.RED fails for whatever reason.
				format = gl.RGBA;
				itemsPerPx = 4;
			} else if (attach.internalFormat === gl.RG32F) {
				// As the R32F case.
				format = gl.RGBA;
				itemsPerPx = 4;
			} else {
				throw new Error(
					"Pixels cannot be read back from texture: texture internal format must be R32F, RGB, RGBA, ALPHA, LUMINANCE or LUMINANCE_ALPHA (all other formats yet unsupported by glii)"
				);
			}

			type = attach.type || gl.UNSIGNED_BYTE;
			arrClass = reverseTypeMap.get(type);

			if (!arrClass) {
				throw new Error("Unknown texture pixel type");
			}
		} else {
			// attach instanceof RenderBuffer
			if (attach.internalFormat === gl.RGBA4) {
				format = gl.RGBA;
				type = gl.UNSIGNED_SHORT_4_4_4_4;
				arrClass = Uint16Array;
			} else if (attach.internalFormat === gl.RGB565) {
				format = gl.RGB565;
				type = gl.UNSIGNED_SHORT_5_6_5;
				arrClass = Uint16Array;
			} else if (attach.internalFormat === gl.RGB5_A1) {
				format = gl.RGB5_A1;
				type = gl.UNSIGNED_SHORT_5_5_5_1;
				arrClass = Uint16Array;
			} else {
				throw new Error(
					"Pixels cannot be read back from renderbuffer: renderbuffer internal format must be RGBA4, RGB565 or RGB5_A1"
				);
			}
		}

		const pixelCount = w * h;

		const out = new arrClass(pixelCount * itemsPerPx);

		gl.bindFramebuffer(gl.FRAMEBUFFER, this.#fb);

		gl.readPixels(x, y, w, h, format, type, out);
		return out;
	}

	/**
	 * @method debugIntoConsole(): this
	 *
	 * Dumps the contents of the (first) colour attachment into the developer
	 * tools' console, with some `<canvas>` and `console.log("%c")` trickery.
	 *
	 * This is an expensive operation and is meant only for debugging purposes.
	 */
	debugIntoConsole() {
		let canvas = document.createElement("canvas");
		const data = this.#colourAttachs[0].asImageData();
		canvas.width = this.width;
		canvas.height = this.height;
		canvas.getContext("2d").putImageData(data, 0, 0);
		let url = canvas.toDataURL();

		console.log(
			"%c+",
			`
		font-size: 1px;
		border: 1px solid black;
		padding: 0px ${Math.floor(this.width / 4)}px;
		line-height: ${this.height / 2}px;
		height: ${this.height}px;
		background: url(${url});
		background-size: ${this.width / 2}px ${this.height / 2}px;
		color: transparent;
		transform: scale(0.3, -0.3);
		`
		);

		return this;
	}
}

/**
 * @factory GliiFactory.FrameBuffer(options: FrameBuffer options)
 * @class Glii
 * @section Class wrappers
 * @property FrameBuffer(options: FrameBuffer options): Prototype of FrameBuffer
 * Wrapped `FrameBuffer` class
 */
registerFactory("FrameBuffer", function (gl) {
	return class WrappedFrameBuffer extends FrameBuffer {
		constructor(opts) {
			super(gl, opts);
		}
	};
});
