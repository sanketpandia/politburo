/**
 * @class AbstractAttributeSet
 *
 * Represents a set of attribute data for vertices (each set being a small slice
 * of contiguous memory), plus some niceties to add/modify records.
 *
 * Internally this represents a `gl.ARRAY_BUFFER` at the WebGL level, or a Vertex Buffer
 * Object (VBO) at the OpenGL level. It also includes the VertexAttrib call(s) needed
 * to use the attribute(s) contained here (since this is WebGL1 and there are no Vertex
 * Array Objects/VAOs, which would cache this).
 *
 * Note that an `AbstractAttributeSet` might correspond to just one attribute (and
 * offer the `BindableAttribute` interface), or several attributes (and do not offer
 * the `BindableAttribute` interface, but rather have properties of setters for such).
 *
 * This is the base abstract class - record size (amount of data per vertex)
 * for this class is zero.
 *
 */
export default class AbstractAttributeSet {
	constructor(gl, options = {}, recordSize = 0) {
		this._gl = gl;

		// Size of each record, in bytes. Must be set internally in each subclass.
		this._recordSize = recordSize;

		// @option size: Number = 255
		// Maximum number of records (vertices) to hold
		this._size = options.size || 255;

		// @option growFactor: Boolean = false
		// Specifies that the size of this attribute buffer is static.
		// @alternative growFactor: Number
		// When `growFactor` is a `Number`, the size of this attribute buffer will
		// grow by that factor (e.g. a factor of 2 means the buffer doubles its size each
		// time the size is insufficient)
		this._growFactor = options.growFactor;

		// @option usage: GLenum = gl.STATIC_DRAW
		// One of `gl.STATIC_DRAW`, `gl.DYNAMIC_DRAW` or `gl.STREAM_DRAW`.
		// See the documentation of the `usage` parameter at
		// [WebGL's `bufferData`](https://developer.mozilla.org/en-US/docs/Web/API/WebGLRenderingContext/bufferData)
		// for more details.
		this._usage = options.usage || gl.STATIC_DRAW;

		// Create a WebGL ARRAY_BUFFER with (record count) * (bytes per record) bytes
		this._buf = gl.createBuffer();
		gl.bindBuffer(gl.ARRAY_BUFFER, this._buf);
		gl.bufferData(gl.ARRAY_BUFFER, this._recordSize * this._size, this._usage);

		if (this._growFactor) {
			// Growable attribute buffers need to store all the data in a
			// readable data structure, in order to call `bufferData` with
			// the new size without destroying data.
			// 			this._arrayBuf = new ArrayBuffer(this._recordSize * this._size);
			this._byteData = new Uint8Array(this._recordSize * this._size);
		}
	}

	// Maps a string containing a GLSL type to its number of components.
	static GLSL_TYPE_COMPONENTS = {
		float: 1,
		vec2: 2,
		vec3: 3,
		vec4: 4,
		// mat2: 4,
		// mat3: 9,
		// mat4: 16,
	};

	/**
	 * @section Batch update methods
	 * @method commit(index, length): this
	 * Dumps the contents of the data in RAM into GPU memory. Will dump a
	 * contiguous section of memory, for a block of vertices starting at `index`
	 * and with the given `length`.
	 *
	 * Will fail if the attribute set has been created with a `growFactor` of zero.
	 */
	commit(index, length) {
		const gl = this._gl;
		const addr = this._recordSize * index;
		const size = this._recordSize * length;
		const data = new Uint8Array(this._byteData.buffer, addr, size);

		if (!isFinite(addr) || !isFinite(size)) {
			throw new Error("Cannot commit attribute data witn non-finite start/length");
		}

		gl.bindBuffer(gl.ARRAY_BUFFER, this._buf);
		gl.bufferSubData(gl.ARRAY_BUFFER, addr, data);

		return this;
	}

	/**
	 * @section Internal methods
	 * @method set(index: Number, offset: Number, data: ArrayBufferView): this
	 *
	 * Uploads the given `data` to GPU memory, given the vertex `index` and the byte
	 * `offset` into that record.
	 *
	 * It can be used to update one record at a time (passing a record-full of `data`),
	 * one attribute field (less than a record-full of data, specifying `offset`), or several
	 * contiguous records.
	 *
	 * The input `data` is *expected* to be in the same byte format than the expected
	 * storage; subclasses do this by coercing `TypeArray`s of specific sizes. Users
	 * wanting to dump binary data using this method are advised to pay attention to
	 * the way the data is packed.
	 *
	 * Will grow self if allowed by `grow` when `index` is larger than the current size.
	 * @alternative
	 * @method set(index: Number, offset: Number, data: ArrayBuffer): this
	 * In addition to `TypedArray`s of any kind and `DataView`s, this method can
	 * also take `ArrayBuffer`s.
	 */
	setBytes(index, offset, data) {
		// if (index >= this._size) {
		const upperIndex =
			index + Math.floor((data.byteLength + offset) / this._recordSize);
		if (upperIndex > this._size) {
			this._grow(upperIndex);
		}

		const gl = this._gl;
		//if (gl.getParameter(gl.ARRAY_BUFFER_BINDING) !== this._buf) {
		gl.bindBuffer(gl.ARRAY_BUFFER, this._buf);
		//}

		const addr = this._recordSize * index + offset;

		gl.bufferSubData(gl.ARRAY_BUFFER, addr, data);

		if (this._byteData) {
			if (data instanceof ArrayBuffer) {
				this._byteData.set(new Uint8Array(data), addr);
			} else {
				this._byteData.set(
					new Uint8Array(data.buffer, data.byteOffset, data.byteLength),
					addr
				);
			}
		}
		return this;
	}

	// Internal use only - grows the size of both the JS RAM `ArrayBuffer` and the
	// in-GPU GL arraybuffer/VBO.
	// Copies everything from the RAM-held copy `this._byteData`, then creates a
	// new, bigger RAM-held copy.
	_grow(minimum) {
		if (!this._growFactor) {
			throw new Error(
				`Non-growable attribute buffer can only hold ${
					this._size
				} records, but tried to set ${minimum + 1}-th record.`
			);
		}
		this._size = Math.max(minimum + 1, Math.ceil(this._size * this._growFactor));

		const newByteData = new Uint8Array(this._recordSize * this._size);
		newByteData.set(this._byteData, 0);
		this._byteData = newByteData;

		const gl = this._gl;
		gl.bindBuffer(gl.ARRAY_BUFFER, this._buf);
		gl.bufferData(gl.ARRAY_BUFFER, this._byteData, this._usage);
	}
}
