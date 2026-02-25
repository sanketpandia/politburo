import constantNames from "./constantNames.mjs";

const factories = {};
// Inspired by Leaflet's addInitHook()
// `fact` must be a factory function that expects a `WebGLContext`,
// optionally expects an instances of `GliiFactory`, and
// returns a (wrapped) class constructor.
export function registerFactory(name, fact) {
	factories[name] = fact;
}

/**
 * @class Glii
 * @aka GliiFactory
 * @inherits EventTarget
 * Glii core. Wraps the functionality of a `WebGLRenderingContext`.
 *
 * Contains wrappers for buffer, program, texture classes; also contains
 * a partial set of WebGL constants (only the ones that need to be
 * specified as options/parameters to Glii classes).
 *
 * @example
 * ```
 * // The Glii factory class is the default export of the Glii module;
 * // importing it looks like...
 * import Glii from "path_to_glii/index.mjs";
 *
 * // Create a Glii factory instance from a canvas...
 * const glii = new Glii(document.getElementById("some-canvas"));
 *
 * // ...and use such instance to spawn stuff...
 * let pointIndices = new glii.IndexBuffer({
 * 	// ...using constants available in the Glii factory instance.
 * 	drawMode: glii.POINTS
 * });
 * ```
 *
 * Note that all Glii classes except for `GliiFactory` are meant to be instantiated from
 * the following wrapped classes. In other words: do not try to instantiate e.g.
 * `new IndexBuffer(...)`, but rather create a `GliiFactory` instance
 * (usually named lowercase `glii` in the documentation and examples) and instantiate
 * `new glii.IndexBuffer(...)`.
 *
 * Idem for WebGL constants: most (if not all) the constants needed in class constructors
 * are copied into the namespace of `GliiFactory`, as shown above with `glii.POINTS`.
 *
 */

export default class GliiFactory extends EventTarget {
	/**
	 * @constructor GliiFactory(target: HTMLCanvasElement, contextAttributes?: Object)
	 * Create a GL factory from a `HTMLCanvasElement`, and context attributes as per
	 * [`getContext`](https://developer.mozilla.org/en-US/docs/Web/API/HTMLCanvasElement/getContext)
	 * @alternative
	 * @constructor GliiFactory(target: WebGLRenderingContext)
	 * Create a GL factory from an already instantiated `WebGLRenderingContext`
	 * @alternative
	 * @constructor GliiFactory(target: WebGL2RenderingContext)
	 * Create a GL factory from an already instantiated `WebGL2RenderingContext`
	 */
	/// TODO: Add another alternative, using only context attributes, which shall
	/// implicitly create the canvas.
	constructor(target, contextAttributes) {
		super();

		if (!target || !target.constructor || !target.constructor.name) {
			// Happens on CI environments (gitlab CI)
			throw new Error(
				"Invalid target passed to GliiFactory constructor. Expected either a HTMLCanvasElement or a WebGLRenderingContext but got " +
					typeof target +
					"," +
					JSON.stringify(target) +
					"."
			);
		}
		switch (target.constructor.name) {
			case "HTMLCanvasElement":
				function get(name) {
					try {
						return target.getContext(name, contextAttributes);
					} catch (e) {
						return undefined;
					}
				}

				this.gl =
					get("webgl2") ||
					get("webgl") ||
					get("experimental-webgl") ||
					get("webgl-experimental");

				if (!this.gl) {
					throw new Error("Glii could not create a WebGL context from canvas.");
				}
				break;

			case "WebGLRenderingContext":
			case "WebGL2RenderingContext":
			case "bound WebGLRenderingContext": // Happens on headless using "gl" module
			case "bound WebGL2RenderingContext":
				this.gl = target;
				break;
			default:
				throw new Error(
					"Invalid target passed to GliiFactory constructor. Expected either a HTMLCanvasElement or a WebGLRenderingContext but got an instance of " +
						target.constructor.name +
						"."
				);
		}

		const gl = this.gl;

		this._isWebGL2 =
			gl.constructor.name === "WebGL2RenderingContext" ||
			gl.constructor.name === "bound WebGL2RenderingContext";

		// Call all individual factory functions, assign the class constructors to
		// properties of this instance.
		for (let factName in factories) {
			this[factName] = factories[factName](gl, this);
		}

		// Copy constants from the `WebGLRenderingContext`.
		for (let i in constantNames) {
			const name = constantNames[i];
			this[name] = gl[name];
		}

		if ("canvas" in gl) {
			gl.canvas.addEventListener(
				"webglcontextlost",
				(ev) => {
					console.warn("glii has lost context", ev);
					ev.preventDefault();
				},
				false
			);
			gl.canvas.addEventListener(
				"webglcontextrestored",
				(ev) => {
					console.warn("glii lost context has been restored", ev);
				},
				false
			);

			const resizeObserver = new ResizeObserver(this.#onResize.bind(this));

			resizeObserver.observe(gl.canvas, { box: "content-box" });
		}

		this.refreshDrawingBufferSize();

		this._loadedExtensions = new Map();

		/// TODO: simulate context loss with gl.getExtension('WEBGL_lose_context').loseContext();

		// 		// Fetch some info from the context
		//
		// 		// This kinda assumes that, when given a WebGLRenderingContext/
		// 		// WebGL2RenderingContext, there have been no framebuffer shenanigans.
		// 		this._defaultFramebuffer = gl.getParameter(gl.FRAMEBUFFER_BINDING);
		// 		this._defaultRenderbuffer = gl.getParameter(gl.RENDERBUFFER_BINDING);
		// 		this._glslVersion = gl.getParameter(gl.SHADING_LANGUAGE_VERSION);
		//
		// 		const attachments = [gl.COLOR_ATTACHMENT0, gl.DEPTH_ATTACHMENT, gl.STENCIL_ATTACHMENT];
		// 		const pnames = [
		// 			gl.FRAMEBUFFER_ATTACHMENT_OBJECT_TYPE,
		// 			gl.FRAMEBUFFER_ATTACHMENT_OBJECT_NAME,
		// 			gl.FRAMEBUFFER_ATTACHMENT_TEXTURE_LEVEL,
		// // 			gl.FRAMEBUFFER_ATTACHMENT_TEXTURE_CUBE_MAP_FACE
		// 		];
		//
		// // 			gl.bindFramebuffer(gl.FRAMEBUFFER, null);
		// 		this._defaultAttachments = {};
		// 		for (let att of attachments){
		// 			this._defaultAttachments[att] = {};
		// 			for (let i=0; i<0xFFFF; i++) {
		// // 				for (let pname of pnames){
		// // 					console.log(att, pname);
		// // 					this._defaultAttachments[att][pname] =
		// 				const value =
		// // 						gl.getFramebufferAttachmentParameter(gl.FRAMEBUFFER, att, pname);
		// 					gl.getFramebufferAttachmentParameter(gl.FRAMEBUFFER, att, i);
		// // 						gl.getFramebufferAttachmentParameter(gl.FRAMEBUFFER, att, null);
		// 				if (value) {
		// 					console.log(att, i, value);
		// 				}
		// 			}
		// 		}
		//
		// 		console.log('default framebuffer: ', this._defaultFramebuffer);
		// 		console.log('default renderbuffer: ', this._defaultRenderbuffer);
		// 		console.log('default attachments: ', this._defaultAttachments);
		// 		console.log('GLSL version: ', this._glslVersion);
	}

	/**
	 * @method getSupportedExtensions(): Array of String
	 * Returns the list of GL extensions supported in the running platform, as per
	 * https://developer.mozilla.org/en-US/docs/Web/API/WebGLRenderingContext/getSupportedExtensions.html
	 */
	getSupportedExtensions() {
		if (this._knownExtensions) {
			return this._knownExtensions;
		}
		return (this._knownExtensions = this.gl.getSupportedExtensions());
	}

	/**
	 * @method isExtensionSupported(extName: String): Boolean
	 * Returns whether the given extension is supported in the running platform
	 */
	isExtensionSupported(extName) {
		return this.getSupportedExtensions().includes(extName);
	}

	/**
	 * @method loadExtension(ext: String): Object
	 * Tries to load the given GL extension. Throws an error if the extension is
	 * not supported.
	 *
	 * Returns the extension object, which may vary by extension.
	 */
	loadExtension(extName) {
		let ext = this._loadedExtensions.get(extName);
		if (ext) {
			return ext;
		} else {
			if (!this.isExtensionSupported(extName)) {
				throw new Error(`WebGL extension ${extName} is not supported`);
			}
			ext = this.gl.getExtension(extName);
			this._loadedExtensions.set(extName, ext);
			return ext;
		}
	}

	/**
	 * @method isWebGL2(): Boolean
	 * Returns whether the Glii instance is using a `WebGL2RenderingContext` or
	 * not.
	 */
	isWebGL2() {
		return this._isWebGL2;
	}

	// React to resize observer updates, and cache the dimensions (in device
	// pixels) of the canvas. The canvas is not updated immediately; instead
	// the `refreshDrawingBufferSize()` method should be called prior to a redraw
	#onResize(entries) {
		let entry = entries[0];

		// From https://webglfundamentals.org/webgl/lessons/webgl-resizing-the-canvas.html
		let width_css;
		let height_css;
		let width_device;
		let height_device;
		let dpr = devicePixelRatio ?? 1;

		if (entry.devicePixelContentBoxSize) {
			// NOTE: Only this path gives the correct answer
			// The other paths are imperfect fallbacks
			// for browsers that don't provide anyway to do this
			width_device = entry.devicePixelContentBoxSize[0].inlineSize;
			height_device = entry.devicePixelContentBoxSize[0].blockSize;

			width_css = width_device / dpr;
			height_css = height_device / dpr;
		} else {
			if (entry.contentBoxSize) {
				if (entry.contentBoxSize[0]) {
					width_css = entry.contentBoxSize[0].inlineSize;
					height_css = entry.contentBoxSize[0].blockSize;
				} else {
					width_css = entry.contentBoxSize.inlineSize;
					height_css = entry.contentBoxSize.blockSize;
				}
			} else {
				width_css = entry.contentRect.width;
				height_css = entry.contentRect.height;
			}
			width_device = width_css * dpr;
			height_device = height_css * dpr
		}


		this.#resizedWidth = width_device = Math.round(width_device);
		this.#resizedHeight = height_device = Math.round(height_device);

		this._drawingBufferSizeChanged = true;

		/**
		 * @event resize: CustomEvent
		 * Fired whenever the underlying `<canvas>` changes size. The next
		 * call to `refreshDrawingBufferSize()` will update the output
		 * framebuffer to the updated size (in device pixels).
		 * The `detail` of this event contains the new size, both in CSS pixels
		 * and device pixels.
		 */
		this.dispatchEvent(
			new CustomEvent("resized", {
				detail: {
					x_css: width_css,
					y_css: height_css,
					x_device: this.#resizedWidth,
					y_device: this.#resizedHeight,
					x: this.#resizedWidth,
					y: this.#resizedHeight
				},
			})
		);
	}

	#resizedWidth;
	#resizedHeight;

	/**
	 * @section Internal methods
	 * @method refreshDrawingBufferSize(): Array of Number
	 * Ensure that the size of the <canvas> linked to the `WebGLRenderingContext`
	 * matches the size provided by `getClientRect()`.
	 *
	 * Meant to be called from a `WebGL1Program` right before fetching the drawing buffer
	 * size. This technique should lower blinking when the `<canvas>` is resized.
	 *
	 * Returns the current canvas dimensions in `[width, height]` form.
	 */
	refreshDrawingBufferSize() {
		if (this._drawingBufferSizeChanged) {
			const canvas = this.gl.canvas;
			if (this.#resizedWidth) {
				this._width = canvas.width = this.#resizedWidth;
				this._height = canvas.height = this.#resizedHeight;
			} else {
				let dpr = devicePixelRatio ?? 1;
				let rect = canvas.getClientRects && canvas.getClientRects()[0];
				let width, height;

				if (rect) {
					// Canvas is in the DOM, possibly with applied CSS
					width = rect.width;
					height = rect.height;
				} else if (canvas.width) {
					// Canvas is *not* in the DOM, so trust its width/height
					/// FIXME: What if canvas is a WebGLRenderingContext?
					width = canvas.width;
					height = canvas.height;
				} else if (canvas.drawingBufferWidth) {
					width = canvas.drawingBufferWidth;
					height = canvas.drawingBufferHeight;
				}

				this._width = canvas.width = width * dpr;
				this._height = canvas.height = height * dpr;
			}
			this._drawingBufferSizeChanged = false;
		}
		return [this._width, this._height];
	}

	/// TODO: lightweight event handler for resizing; uniforms might need to be re-set.
}
