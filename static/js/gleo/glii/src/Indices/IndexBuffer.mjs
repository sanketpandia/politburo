import { registerFactory } from "../GliiFactory.mjs";
import { default as SequentialIndices } from "./SequentialIndices.mjs";

/**
 * @class IndexBuffer
 * @inherits SequentialIndices
 *
 * Represents a set of configurable vertex indices. This will tell the
 * program which vertices to draw, in which mode (points/lines/triangles),
 * and in which order. Includes a couple of niceties like setters and
 * handling of the data types.
 *
 * The implementation of `IndexBuffer` is low-ish level. Using the
 * `TriangleIndices` subclass might feel easier.
 *
 * Internally this represents a `gl.ELEMENT_ARRAY_BUFFER` at the WebGL level, or an
 * Element Array Buffer (EAB) at the OpenGL level.
 */

export default class IndexBuffer extends SequentialIndices {
	constructor(gl, gliiFactory, options = {}) {
		super(gl, options);

		// @section
		// @aka IndexBuffer options
		// @option size: Number = 255
		// Maximum number of indices to hold
		this._size = options.size || 255;

		// @option growFactor: Boolean = false
		// Specifies that the size of this indices buffer is static.
		// @alternative
		// @option growFactor: Number
		// Allows this buffer to automatically grow when `set()` is out of bounds.
		//
		// Each time that happens, the `size` of this indices buffer will
		// grow by that factor (e.g. a `growFactor` of 2 means the buffer doubles its size each
		// time the size is insufficient). `growFactor` should be greater than `1`.
		this._growFactor = options.growFactor || 1.2;

		// @option usage: Buffer usage constant = glii.STATIC_DRAW
		// One of `gl.STATIC_DRAW`, `gl.DYNAMIC_DRAW` or `gl.STREAM_DRAW`.
		// See the documentation of the `usage` parameter at
		// [`bufferData`](https://developer.mozilla.org/en-US/docs/Web/API/WebGLRenderingContext/bufferData)
		// for more details.
		this._usage = options.usage || gl.STATIC_DRAW;

		// @option type: Data type constant = glii.UNSIGNED_SHORT
		// One of `glii.UNSIGNED_BYTE`, `glii.UNSIGNED_SHORT` or `glii.UNSIGNED_INT`.
		// This sets the maximum index that can be referenced by this `IndexBuffer`
		// (but not how many indices this `IndexBuffer` can hold):
		// 2^8, 2^16 or 2^32 respectively.
		// See the documentation of the `usage` parameter at
		// [`gl.drawElements`](https://developer.mozilla.org/en-US/docs/Web/API/WebGLRenderingContext/drawElements)
		// for more details.
		this._type = options.type || gl.UNSIGNED_SHORT;

		if (this._type === gl.UNSIGNED_BYTE) {
			this._bytesPerSlot = 1;
			this._typedArray = Uint8Array;
			this._maxValue = 1 << 8;
		} else if (this._type === gl.UNSIGNED_SHORT) {
			this._bytesPerSlot = 2;
			this._typedArray = Uint16Array;
			this._maxValue = 1 << 16;
		} else if (this._type === gl.UNSIGNED_INT) {
			/// Manually load the relevant GL extension, only needed in WebGL1
			/// contexts.
			gliiFactory.isWebGL2() || gliiFactory.loadExtension("OES_element_index_uint");

			this._bytesPerSlot = 4;
			this._typedArray = Uint32Array;
			this._maxValue = (1 << 16) * (1 << 16); // The JS << operator is clamped to 2^32 :-/
		} else {
			throw new Error(
				"Invalid type for IndexBuffer. Must be one of `gl.UNSIGNED_BYTE`, `gl.UNSIGNED_SHORT` or `gl.UNSIGNED_INT`."
			);
		}
		this._buf = gl.createBuffer();
		gl.bindBuffer(gl.ELEMENT_ARRAY_BUFFER, this._buf);
		gl.bufferData(
			gl.ELEMENT_ARRAY_BUFFER,
			this._size * this._bytesPerSlot,
			this._usage
		);

		// Upper bound of many indices in this IndexBuffer are pointing to valid vertices.
		// AKA "highest set slot"
		this._activeIndices = 0;

		if (this._growFactor) {
			// Growable index buffers need to store all the data in a
			// readable data structure, in order to call `bufferData` with
			// the new size without destroying data.
			this._ramData = new this._typedArray(this._size);
		}
	}

	/**
	 * @method set(n: Number, indices: Array of Number): this
	 * Stores the given indices as 1-, 2- or 4-byte integers, starting at the `n`-th
	 * position in this `IndexBuffer` (zero-indexed; the first index slot is at `n`=0).
	 *
	 * The indices passed must exist (and likely, have values) in any
	 * `AttributeBuffer`s being used together with this `IndexBuffer` in a
	 * GL program.
	 */
	set(n, indices) {
		if (indices.length === 0) {
			return this;
		}

		const gl = this._gl;
		this.grow(n + indices.length);

		gl.bufferSubData(
			gl.ELEMENT_ARRAY_BUFFER,
			n * this._bytesPerSlot,
			this._typedArray.from(indices)
		);

		this._setActiveIndices(n + indices.length);

		if (this._ramData) {
			this._ramData.set(indices, n);
		}

		return this;
	}

	/**
	 * @method truncate(n: Number): this
	 * Shrinks the amount of indices to be drawn, so that only the first `n`
	 * indices are considered.
	 */
	truncate(n) {
		this._activeIndices = Math.min(this._activeIndices, n);
		return this;
	}

	/**
	 * @section Internal methods
	 * @uninheritable
	 * @method grow(minimum): undefined
	 * Internal usage only. Grows the size of both the internal buffer, so
	 * it can contain at least `minimum` index slots.
	 */
	grow(minimum) {
		this.bindMe();
		if (this._size >= minimum) {
			return;
		}
		if (!this._growFactor) {
			throw new Error(
				`Tried to set index out of bounds of non-growable IndexBuffer (requested ${minimum} vs size ${this._size})`
			);
		} else {
			this._size = Math.max(minimum + 1, Math.ceil(this._size * this._growFactor));

			const newRamData = new this._typedArray(this._size);
			newRamData.set(this._ramData, 0);
			this._ramData = newRamData;

			const gl = this._gl;
			gl.bufferData(gl.ELEMENT_ARRAY_BUFFER, this._ramData, this._usage);
		}
	}

	/**
	 * @method bindMe(): undefined
	 * Internal use only. (Re-)binds itself as the `ELEMENT_ARRAY_BUFFER` of the
	 * `WebGLRenderingContext`.
	 *
	 * In practice, this should happen every time the program changes in the
	 * current context.
	 *
	 * This is expected to be called from `WebGL1Program` only.
	 */
	bindMe() {
		const gl = this._gl;
		//if (gl.getParameter(gl.ELEMENT_ARRAY_BUFFER_BINDING) !== this._buf) {
		gl.bindBuffer(gl.ELEMENT_ARRAY_BUFFER, this._buf);
		//}
	}

	/**
	 * @method drawMe(): undefined
	 * Internal use only. Does the `WebGLRenderingContext.drawElement()` calls, but
	 * assumes that everyhing else (bound program, textures, attribute name-locations,
	 * uniform name-locations-values) has been set up already.
	 *
	 * This is expected to be called from `WebGL1Program` only.
	 */
	drawMe() {
		this.bindMe();
		this._gl.drawElements(this._drawMode, this._activeIndices, this._type, 0);
	}

	/**
	 * @method drawMePartial(start: Number, count: Number): undefined
	 * Internal use only. As `drawMe()`, but lets the programmer explicitly
	 * set the range of vertex slots to be drawn.
	 *
	 * This is expected to be called from `WebGL1Program` only.
	 */
	drawMePartial(start, count) {
		this.bindMe();
		this._gl.drawElements(
			this._drawMode,
			count,
			this._type,
			start * this._bytesPerSlot
		);
	}

	// Set the number of active indices to either the given number or the number of active indices,
	// whatever is greater.
	_setActiveIndices(n) {
		this._activeIndices = Math.max(this._activeIndices, n);
	}

	/**
	 * @section
	 * @method destroy(): this
	 * Tells WebGL to free resources associated with this `IndexBuffer`. Use
	 * when the `IndexBuffer` won't be used anymore.
	 *
	 * After being destroyed, WebGL programs should not use the destroyed `IndexBuffer`.
	 */
	destroy() {
		this._gl.deleteBuffer(this._buf);
		return this;
	}

	/**
	 * @section Batch update methods
	 *
	 * These methods are a less convenient, but more performant, way of updating
	 * indices data.
	 *
	 * For a `IndexBuffer`, the workflow is:
	 * - Call `asTypedArray()` once
	 * - Update the values in the returned typed array (using typed array offsets,
	 *   avoiding array concatenations)
	 * - Call `commit()`
	 *
	 * These methods need the index buffer to have been created with a `growFactor`
	 * larger than zero.
	 *
	 * @method asTypedarray(minSize?: Number): TypedArray
	 * Returns a view of the internal in-RAM buffer for the index data, able to
	 * contain at least `minSize` indices.
	 */
	asTypedArray(minSize) {
		if (isFinite(minSize)) {
			this.grow(minSize);
		}
		return this._ramData;
	}

	/**
	 * @method commit(start, length): this
	 * Dumps the contents of the data in RAM into GPU memory. Will dump
	 * a contiguous section, for a block of indices starting at `start` and
	 * with the given `length`.
	 */
	commit(start, length) {
		this.bindMe();

		const addr = this._bytesPerSlot * start;
		const data = new this._typedArray(this._ramData.buffer, addr, length);
		const gl = this._gl;

		gl.bufferSubData(gl.ELEMENT_ARRAY_BUFFER, addr, data);

		this._setActiveIndices(start + length);
	}
}

/**
 * @factory GliiFactory.IndexBuffer(options: IndexBuffer options)
 * @class Glii
 * @section Class wrappers
 * @property IndexBuffer(options: IndexBuffer options): Prototype of IndexBuffer
 * Wrapped `IndexBuffer` class
 */
registerFactory("IndexBuffer", function (gl, gliiFactory) {
	return class WrappedIndexBuffer extends IndexBuffer {
		constructor(options) {
			super(gl, gliiFactory, options);
		}
	};
});
