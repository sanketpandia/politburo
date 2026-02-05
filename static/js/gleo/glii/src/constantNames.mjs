export default [
	/**
	 * @class Glii
	 * @section Buffer usage constants
	 * @aka Buffer usage constant
	 *
	 * Used in the `usage` option of `IndexBuffer`s and `AbstractAttributeSet`s,
	 * these allegedly tell the hardware which region of GPU memory the data should
	 * be into.
	 *
	 * @property STATIC_DRAW: Number
	 * Hints the hardware that the contents of the buffer are likely to be used often
	 * and not change often.
	 * @property DYNAMIC_DRAW: Number
	 * Hints the hardware that the contents of the buffer are likely to be used often
	 * and change often.
	 * @property STREAM_DRAW: Number
	 * Hints the hardware that the contents of the buffer are likely to not be used often.
	 */
	"STATIC_DRAW",
	"DYNAMIC_DRAW",
	"STREAM_DRAW",

	/**
	 * @section Data type constants
	 * @aka Data type constant
	 *
	 * Used in the `type` option of `IndexBuffer`s.
	 *
	 * Note that `BindableAttribute`s infer the data type from the subclass of `TypedArray`.
	 *
	 * @property BYTE: Number; 8-bit integer, complement-2 signed
	 * @property UNSIGNED_BYTE: Number; 8-bit integer, unsigned
	 * @property SHORT: Number; 16-bit integer, complement-2 signed
	 * @property UNSIGNED_SHORT: Number; 16-bit integer, unsigned
	 * @property INT: Number; 32-bit integer, complement-2 signed
	 * @property UNSIGNED_INT: Number; 32-bit integer, unsigned
	 * @property FLOAT: Number; 32-bit IEEE754 floating point
	 */
	"BYTE",
	"UNSIGNED_BYTE",
	"SHORT",
	"UNSIGNED_SHORT",
	"INT",
	"UNSIGNED_INT",
	"FLOAT",

	/**
	 * @section Texture pixel type constants
	 * @aka Texture pixel type constant
	 *
	 * Used in the `type` option of `Texture`s, for the `type` parameter of `texImage2D` calls.
	 *
	 * Note that, in WebGL1, some values are only valid when an extension is loaded. See
	 * [`WEBGL_depth_texture`](https://developer.mozilla.org/en-US/docs/Web/API/WEBGL_depth_texture.html),
	 * [`OES_texture_float`](https://developer.mozilla.org/en-US/docs/Web/API/OES_texture_float.html), and
	 * [`OES_texture_half_float`](https://developer.mozilla.org/en-US/docs/Web/API/OES_texture_half_float.html).
	 *
	 * @property UNSIGNED_BYTE: Number; 8-bit integer, unsigned
	 * @property UNSIGNED_SHORT_5_6_5: Number; 5 red bits, 6 green bits, 5 blue bits.
	 * @property UNSIGNED_SHORT_4_4_4_4: Number; 4 red bits, 4 green bits, 4 blue bits, 4 alpha bits.
	 * @property UNSIGNED_SHORT_5_5_5_1: Number; 5 red bits, 5 green bits, 5 blue bits, 1 alpha bit.
	 * @property UNSIGNED_SHORT: Number; 16-bit integer, unsigned
	 * @property UNSIGNED_INT: Number; 32-bit integer, unsigned
	 * @property FLOAT: Number; 32-bit IEEE754 floating point
	 */

	//"UNSIGNED_BYTE",
	"UNSIGNED_SHORT_5_6_5",
	"UNSIGNED_SHORT_4_4_4_4",
	"UNSIGNED_SHORT_5_5_5_1",
	//"UNSIGNED_SHORT",
	//"UNSIGNED_INT",
	//"FLOAT",

	/**
	 * @section Draw mode constants
	 * @aka Draw mode constant
	 *
	 * Used in the `drawMode` option of `SequentialIndices`, `IndexBuffer` and
	 * `SparseIndices`. Determines how vertices (pointed by their indices) form draw
	 * primitives.
	 *
	 * See [primitives in the OpenGL wiki](https://www.khronos.org/opengl/wiki/Primitive)
	 * and [`drawElements` in Mozilla dev network](https://developer.mozilla.org/en-US/docs/Web/API/WebGLRenderingContext/drawElements).
	 *
	 * @property POINTS: Number; Each vertex is drawn as a single point.
	 * @property LINES: Number; Each set of two vertices is drawn as a line segment.
	 * @property LINE_LOOP: Number
	 * Each vertex connects to the next with a line segment. The last vertex connects to
	 * the first.
	 * @property LINE_STRIP: Number
	 * Draw a line segment from the first vertex to each of the other vertices
	 * @property TRIANGLES: Number
	 * Each set of three vertices is drawn as a triangle (0-1-2, then 3-4-5, 6-7-8, etc)
	 * @property TRIANGLE_STRIP: Number
	 * Each group of three adjacent vertices is drawn as a triangle (0-1-2, then 2-3-4,
	 * 3-4-5, etc). See [triangle strip on wikipedia](https://en.wikipedia.org/wiki/Triangle_strip)
	 * @property TRIANGLE_FAN: Number
	 * The first vertex plus each group of two adjacent vertices is drawn as a triangle.
	 * See [triangle fan on wikipedia](https://en.wikipedia.org/wiki/Triangle_fan)
	 */
	"POINTS",
	"LINES",
	"LINE_LOOP",
	"LINE_STRIP",
	"TRIANGLES",
	"TRIANGLE_STRIP",
	"TRIANGLE_FAN",

	/**
	 * @section Texture format constants
	 * @aka Texture format constant
	 *
	 * Determines the [image format](https://www.khronos.org/opengl/wiki/Image_Format)
	 * of a `Texture`. Used in a `Texture`'s `format`&`internalFormat` options&properties.
	 *
	 * Some of these are only available when using a WebGL2 context. In some
	 * cases, a few of the WebGL2-only formats are available when using a WebGL1
	 * extension such as `OES_texture_float`.
	 *
	 * See https://registry.khronos.org/webgl/specs/latest/2.0/#TEXTURE_TYPES_FORMATS_FROM_DOM_ELEMENTS_TABLE
	 *
	 * @property RGB: Number; Texture holds red, green and blue components.
	 * @property RGBA: Number; Texture holds red, green, blue and alpha components.
	 * @property ALPHA: Number; Texture holds only an alpha component
	 * @property LUMINANCE: Number
	 * Texture holds only a luminance component. This effectively makes the texture greyscale.
	 * @property LUMINANCE_ALPHA: Number
	 * Texture holds luminance and alpha. This effectively makes the texture grayscale with
	 * transparency.
	 * @property RED: Number; WebGL2 only. Texture holds red component only.
	 * @property RG: Number
	 * WebGL2 only. Texture holds red and green components only.
	 * @property RED_INTEGER
	 * WebGL2 only. Texture holds integers in its red component.
	 * @property RG_INTEGER
	 * WebGL2 only. Texture holds integer in its red and green components.
	 * @property RGB_INTEGER
	 * WebGL2 only. Texture holds integer in its red, green and blue components.
	 * @property RGBA_INTEGER
	 * WebGL2 only. Texture holds integer in its red, green, blue and alpha components.
	 *
	 */
	"ALPHA",
	"RGB",
	"RGBA",
	"LUMINANCE",
	"LUMINANCE_ALPHA",

	"RED",
	"RG",
	"RED_INTEGER",
	"RG_INTEGER",
	"RGB_INTEGER",
	"RGBA_INTEGER",

	/**
	 * @section Texture interpolation constants
	 * @aka Texture interpolation constant
	 *
	 * Determines the behaviour of texel interpolation (when a fragment shader requests
	 * a texel coordinate which falls between several texels). This is used in the
	 * `minFilter` and `maxFilter` options&properties of `Texture`s.
	 *
	 * See [sampler filtering on the OpenGL wiki](https://www.khronos.org/opengl/wiki/Sampler_Object#Filtering)
	 *
	 * @property NEAREST: Number; Nearest-texel interpolation
	 * @property LINEAR: Number; Linear interpolation between texels
	 * @property NEAREST_MIPMAP_NEAREST: Number
	 * Nearest-texel interpolation, in the nearest mipmap
	 * @property LINEAR_MIPMAP_NEAREST: Number
	 * Linear interpolation between texels, in the nearest mipmap
	 * @property NEAREST_MIPMAP_LINEAR: Number
	 * Nearest-texel interpolation, in a linearly-interpolatex mipmap
	 * @property LINEAR_MIPMAP_LINEAR: Number
	 * Linear interpolation between texels, in a linearly-interpolated mipmap
	 */
	"NEAREST",
	"LINEAR",
	"NEAREST_MIPMAP_NEAREST",
	"LINEAR_MIPMAP_NEAREST",
	"NEAREST_MIPMAP_LINEAR",
	"LINEAR_MIPMAP_LINEAR",

	/**
	 * @section Texture wrapping constants
	 * @aka Texture wrapping constant
	 *
	 * Used in the `wrapS`/`wrapT` options of a `Texture`.
	 *
	 * Determines the behaviour of texel sampling when the requested texel is outside
	 * the bounds of the `Texture` (i.e. when the texel coordinate is outside the
	 * [0..1] range).
	 *
	 * See [https://learnopengl.com/Getting-started/Textures](https://learnopengl.com/Getting-started/Textures)
	 * for an illustrative example.
	 *
	 * @property REPEAT: Number; Texture repeats.
	 * @property CLAMP_TO_EDGE: Number
	 * Texels from the edge of the texture are used outside.
	 * @property MIRRORED_REPEAT: Number
	 * Texture repeats but is mirrored on every odd occurence.
	 */
	"REPEAT",
	"CLAMP_TO_EDGE",
	"MIRRORED_REPEAT",

	/**
	 * @section Renderbuffer format constants
	 * @aka Renderbuffer format constant
	 *
	 * Determines the internal format of a `RenderBuffer` (to be attached as
	 * either/both the depth component and/or the stencil component of a
	 * framebuffer).
	 *
	 * See [`renderBufferStorage`](https://developer.mozilla.org/en-US/docs/Web/API/WebGLRenderingContext/renderbufferStorage)
	 *
	 * Some of these are only available when using a WebGL2 context. (In some
	 * cases, a few of the WebGL2-only formats are available when using a WebGL1
	 * extension such as `WEBGL_depth_texture`).
	 *
	 * @property RGBA4: Number;  4 red bits, 4 green bits, 4 blue bits 4 alpha bits.
	 * @property RGB565: Number;  5 red bits, 6 green bits, 5 blue bits.
	 * @property RGB5_A1: Number;  5 red bits, 5 green bits, 5 blue bits, 1 alpha bit.
	 * @property DEPTH_COMPONENT16: Number; Renderbuffer holds 16 bits of depth
	 * @property STENCIL_INDEX8: Number; Renderbuffer holds 8 bits of stencil
	 * @property DEPTH_STENCIL: Number; Renderbuffer holds both depth and stencil
	 * (implementation-dependant; can be assumed to hold *at least* 16 bits of depth
	 * and 8 bits of stencil on WebGL1; in WebGL2 it should behave as `DEPTH24_STENCIL8`)
	 * @property DEPTH_COMPONENT24: Number; Renderbuffer holds 24 bits of depth
	 * (WebGL2 only).
	 * @property DEPTH_COMPONENT32F: Number; Renderbuffer holds depth as 32-bit
	 * floating point (WebGL2 only).
	 * @property DEPTH24_STENCIL8: Number; Renderbuffer holds 24 bits of depth
	 * and 8 bits of stencil (WebGL2 only).
	 * @property DEPTH32F_STENCIL8: Number; Renderbuffer holds depth as 32-bit
	 * @property R32I: Number; Renderbuffer holds depth as 32-bit signed
	 * integer.
	 */
	"RGBA4",
	"RGB565",
	"RGB5_A1",

	"DEPTH_COMPONENT16",	//0x81A5
	"STENCIL_INDEX8",	//0x8D48
	"DEPTH_STENCIL",	//0x84F9

	"DEPTH_COMPONENT24",
	"DEPTH_COMPONENT32F",
	"DEPTH24_STENCIL8",
	"DEPTH32F_STENCIL8",

	"R32I",

	/**
	 * @section Comparison constants
	 * @aka Comparison constant
	 *
	 * Used in the `depth` option of `WebGL1Program`.
	 *
	 * Use `glii.ALWAYS` to disable depth testing. Otherwise, the most usual
	 * value is `glii.LEQUAL` or `glii.LESS`, to render fragments with a lower
	 * `z` component of their `gl_Position` ("closer to the camera") over fragments
	 * with a higher `z`.
	 *
	 * See [depth testing in learnopengl.com](https://learnopengl.com/Advanced-OpenGL/Depth-testing).
	 *
	 * @property NEVER: Number; Always fails (i.e. shall drop all fragments).
	 * @property ALWAYS: Number; Disables depth testing.
	 * @property LESS: Number
	 * Render fragments that have a lower `z` ("closer to the camera") over others.
	 * @property LEQUAL: Number
	 * As `LESS`, but also renders fragments with the same `z`.
	 * @property GREATER: Number
	 * Render fragments that have a higher `z` ("further away from the camera") over others.
	 * @property GEQUAL: Number
	 * As `GREATER`, but also renders fragments with the same `z`.
	 * @property EQUAL: Number
	 * Only render fragments with the same `z` as the depth buffer value.
	 * @property NOTEQUAL: Number; Opposite of `EQUAL`.
	 */
	"NEVER",
	"ALWAYS",
	"LESS",
	"LEQUAL",
	"GREATER",
	"GEQUAL",
	"EQUAL",
	"NOTEQUAL",

	/**
	 * @section Blend equation constants
	 * @aka Blend equation constant
	 *
	 * Used in the `blend` option of `WebGL1Program`.
	 *
	 * Defines which kind of arithmetic operation is applied to the RGB and Alpha
	 * channels of fragments when they need to be blended together (i.e. when
	 * two or more fragments from several triangles have the same `x,y` position
	 * in the output framebuffer)
	 *
	 * See `WebGLRenderingContext`'s [`blendEquationSeparate`](https://developer.mozilla.org/en-US/docs/Web/API/WebGLRenderingContext/blendEquationSeparate)
	 *
	 * @property FUNC_ADD: Number; source + destination
	 * @property FUNC_SUBTRACT: Number; source - destination
	 * @property FUNC_REVERSE_SUBTRACT: Number; destination - source
	 * @property MIN: Number; Minimum of source and destination
	 * @property MAX: Number; Maximum of source and destination
	 */
	"FUNC_ADD",
	"FUNC_SUBTRACT",
	"FUNC_REVERSE_SUBTRACT",
	"MIN",
	"MAX",

	/**
	 * @section Blend factor constants
	 * @aka Blend factor constant
	 *
	 * Used in the `blend` option of `WebGL1Program`.
	 *
	 * Defines what factors shall multiply the RGB and Alpha components of
	 * overlapping fragments just prior to applying the "blend equation" operation.
	 *
	 * See `WebGLRenderingContext`'s [`blendFuncSeparate`](https://developer.mozilla.org/en-US/docs/Web/API/WebGLRenderingContext/blendFuncSeparate)
	 * @property ZERO: Number; Multiplies all colors by 0.
	 * @property ONE: Number; Multiplies all colors by 1.
	 * @property SRC_COLOR: Number; Multiplies all colors by the source colors.
	 * @property ONE_MINUS_SRC_COLOR: Number; Multiplies all colors by 1 minus each source color.
	 * @property DST_COLOR: Number; Multiplies all colors by the destination color.
	 * @property ONE_MINUS_DST_COLOR: Number; Multiplies all colors by 1 minus each destination color.
	 * @property SRC_ALPHA: Number; Multiplies all colors by the source alpha color.
	 * @property ONE_MINUS_SRC_ALPHA: Number; Multiplies all colors by 1 minus the source alpha color.
	 * @property DST_ALPHA: Number; Multiplies all colors by the destination alpha color.
	 * @property ONE_MINUS_DST_ALPHA: Number; Multiplies all colors by 1 minus the destination alpha color.
	 * @property CONSTANT_COLOR: Number; Multiplies all colors by a constant color.
	 * @property ONE_MINUS_CONSTANT_COLOR: Number; Multiplies all colors by 1 minus a constant color.
	 * @property CONSTANT_ALPHA: Number; Multiplies all colors by a constant alpha value.
	 * @property ONE_MINUS_CONSTANT_ALPHA: Number; Multiplies all colors by 1 minus a constant alpha value.
	 * @property SRC_ALPHA_SATURATE: Number; Multiplies the RGB colors by the smaller of either the source alpha color or the value of 1 minus the destination alpha color. The alpha value is multiplied by 1.
	 *
	 */
	"ZERO",
	"ONE",
	"SRC_COLOR",
	"ONE_MINUS_SRC_COLOR",
	"DST_COLOR",
	"ONE_MINUS_DST_COLOR",
	"SRC_ALPHA",
	"ONE_MINUS_SRC_ALPHA",
	"DST_ALPHA",
	"ONE_MINUS_DST_ALPHA",
	"CONSTANT_COLOR",
	"ONE_MINUS_CONSTANT_COLOR",
	"CONSTANT_ALPHA",
	"ONE_MINUS_CONSTANT_ALPHA",
	"SRC_ALPHA_SATURATE",
];
