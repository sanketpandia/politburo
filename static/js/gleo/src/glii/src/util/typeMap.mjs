// Maps each kind of TypedArray to the GL constant for the corresponding type.
// Only includes kinds of JS TypedArray that have a counterpart in WebGL.

// prettier-ignore
export default new Map([
	[Int8Array,         0x1400], // gl.BYTE
	[Uint8Array,        0x1401], // gl.UNSIGNED_BYTE
	[Uint8ClampedArray, 0x1401], // gl.UNSIGNED_BYTE
	[Int16Array,        0x1402], // gl.SHORT
	[Uint16Array,       0x1403], // gl.UNSIGNED_SHORT
	[Int32Array,        0x1404], // gl.INT
	[Uint32Array,       0x1405], // gl.UNSIGNED_INT
	[Float32Array,      0x1406], // gl.FLOAT
]);

/*
 * Notes:
 *
 * - For attributes, gl.INT and gl.UNSIGNED_INT can only
 * be used in WebGL2 (vertexAttrib*I*Pointer)
 *
 * - For index buffers, only unsigned types are valid.
 *
 * - While the rest of the code copies the GL constants from the WebGLRenderingContext,
 * this *assumes* the type constants are indeed constants. I always have this
 * fear that the actual integer values for these constants might change without notice.
 *
 */
