// Akin to typeMap, but in reverse: maps GL constants to TypedArray prototypes intead.

// Includes constants for texture pixel types as well (same kind of mapping, constant
// to corresponding TypedArray prototype able to hold that kind of data).

// prettier-ignore
export default new Map([
	[0x1400,	Int8Array   ], // gl.BYTE
	[0x1401,	Uint8Array  ], // gl.UNSIGNED_BYTE
	[0x1402,	Int16Array  ], // gl.SHORT
	[0x1402,	Int16Array  ], // gl.SHORT
	[0x1403,	Uint16Array ], // gl.UNSIGNED_SHORT
	[0x1404,	Int32Array  ], // gl.INT
	[0x1405,	Uint32Array ], // gl.UNSIGNED_INT
	[0x1406,	Float32Array], // gl.FLOAT

	[0x8033,	Uint16Array], // gl.UNSIGNED_SHORT_4_4_4_4
	[0x8034,	Uint16Array], // gl.UNSIGNED_SHORT_5_5_5_1
	[0x8363,	Uint16Array], // gl.UNSIGNED_SHORT_5_6_5

	[0x84FA,	Uint16Array], // ext.UNSIGNED_INT_24_8_WEBGL from WEBGL_depth_texture

	[0x8D61,	Uint16Array], // ext.HALF_FLOAT_OES from OES_texture_half_float
]);
