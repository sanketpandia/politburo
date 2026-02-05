import { registerFactory } from "../GliiFactory.mjs";

/**
 * @class SequentialIndices
 *
 * Represents a set of sequential vertex indices. The order cannot
 * be changed.
 *
 * A `SequentialIndices` of size `n` behaves (in practice) the same as a
 * `IndexBuffer` of size `n` with sequential indices starting at zero.
 *
 * In other words, the following two are equivalent in practice:
 *
 * ```
 * let seqIdx = new glii.SequentialIndices({ size: 9 });
 *
 * let idxBuf = new glii.IndexBuffer({ size: 9 });
 * idxBuf.set(0, [0,1,2,3,4,5,6,7,8]);
 * ```
 *
 * Internally, this performs a `gl.drawArrays` calls instead of `gl.drawElements`.
 *
 */

export default class SequentialIndices {
	constructor(gl, options = {}) {
		this._gl = gl;

		// @section
		// @aka SequentialIndices options
		// @option size: Number = 3
		// Exact number of indices to refer to.
		this._size = options.size || 3;

		// @option drawMode: Draw mode constant = glii.TRIANGLES
		// Determines what kind of primitive is represented by the vertices
		// pointed by the indices.
		this._drawMode = options.drawMode === undefined ? gl.TRIANGLES : options.drawMode;
	}

	/**
	 * @section Internal methods
	 * @uninheritable
	 * @method drawMe(): undefined
	 * Internal use only. Does the `WebGLRenderingContext.drawArrays()` calls, but
	 * assumes that everyhing else (bound program, textures, attribute name-locations,
	 * uniform name-locations-values) has been set up already.
	 *
	 * This is expected to be called from `WebGL1Program` only.
	 */
	drawMe() {
		this._gl.drawArrays(this._drawMode, 0, this._size);
	}

	/**
	 * @method drawMePartial(start: Number, count: Number): undefined
	 * Internal use only. As `drawMe()`, but lets the programmer explicitly
	 * set the range of vertex slots to be drawn.
	 *
	 * This is expected to be called from `WebGL1Program` only.
	 */
	drawMePartial(start, count) {
		this._gl.drawArrays(this._drawMode, start, count);
	}

	/**
	 * @section
	 * @method destroy(): this
	 * Stub method.
	 *
	 * Subclasses that allocate WebGL resources (i.e. element buffers) shall
	 * reimplement this method and free them.
	 */
	destroy() {
		return this;
	}
}

/**
 * @section
 * @factory GliiFactory.SequentialIndices(options: SequentialIndices options)
 * @class Glii
 * @section Class wrappers
 * @property SequentialIndices(options: SequentialIndices options): Prototype of SequentialIndices
 * Wrapped `SequentialIndices` class
 */
registerFactory("SequentialIndices", function (gl) {
	return class WrappedSequentialIndices extends SequentialIndices {
		constructor(options) {
			super(gl, options);
		}
	};
});
