/**
 * @class RenderBuffer
 * @inherits AbstractFrameBufferAttachment
 *
 * Wraps a [`WebGLRenderbuffer`](https://developer.mozilla.org/en-US/docs/Web/API/WebGLRenderbuffer)
 * and offers convenience methods.
 *
 * A `RenderBuffer` is most akin to an image: a rectangular collection of
 * pixels with `width`, `height` and a `internalFormat`. The main difference
 * between a `RenderBuffer` and a 2D `Texture` is the different `internalFormat`s.
 */

import { registerFactory } from "../GliiFactory.mjs";

export default class RenderBuffer {
	#width;
	#height;
	#gl;
	#rb;
	#internalFormat;
	#multisample;

	constructor(gl, opts = {}) {
		this.#gl = gl;
		this.#rb = gl.createRenderbuffer();

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

		// @option internalFormat: Renderbuffer format constant = gl.RGBA4
		// Internal format of this `RenderBuffer`, as per [`renderBufferStorage`](https://developer.mozilla.org/en-US/docs/Web/API/WebGLRenderingContext/renderbufferStorage).
		this.#internalFormat = opts.internalFormat || gl.RGBA4;

		// @option multisample: Number = 0
		// Number of samples to be used in the `RenderBuffer`. Set to higher values
		// for smoother antialiasing. Only has effect if the Glii context is WebGL2.
		this.#multisample = opts.multisample || 0;

		this.resize(this.#width, this.#height);
	}

	/**
	 * @property rb: WebGLRenderbuffer
	 * The underlying instance of `WebGLRenderBuffer`. Read-only.
	 */
	get rb() {
		if (!this.#rb) {
			throw new Error("RenderBuffer has been destroyed and cannot be used");
		}
		return this.#rb;
	}

	/**
	 * @method resize(x: Number, y: Number): this
	 * Sets a new size for the renderbuffer (destroying its data in the process).
	 */
	resize(x, y) {
		this.#width = x;
		this.#height = y;
		const gl = this.#gl;

		gl.bindRenderbuffer(gl.RENDERBUFFER, this.#rb);

		if ("renderbufferStorageMultisample" in gl && this.#multisample > 1) {
			gl.renderbufferStorageMultisample(
				gl.RENDERBUFFER,
				this.#multisample,
				this.#internalFormat,
				this.#width,
				this.#height
			);
		} else {
			gl.renderbufferStorage(
				gl.RENDERBUFFER,
				this.#internalFormat,
				this.#width,
				this.#height
			);
		}
		return this;
	}

	/**
	 * @method destroy(): this
	 * Tells WebGL to free resources associated with this `RenderBuffer`. Use
	 * when the `RenderBuffer` won't be used anymore.
	 *
	 * After being destroyed, WebGL programs should not use any `FrameBuffer` which
	 * points to the destroyed `RenderBuffer`.
	 */
	destroy() {
		this.#gl.deleteRenderbuffer(this.#rb);
		this.#rb = undefined;
	}
}

/**
 * @factory GliiFactory.RenderBuffer(options: RenderBuffer options)
 * @class Glii
 * @section Class wrappers
 * @property RenderBuffer(options: RenderBuffer options): Prototype of RenderBuffer
 * Wrapped `RenderBuffer` class
 */
registerFactory("RenderBuffer", function (gl) {
	return class WrappedRenderBuffer extends RenderBuffer {
		constructor(opts) {
			super(gl, opts);
		}
	};
});
