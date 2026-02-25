/// This class doesn't exist.
/// This file exists only to generate documentation about the `BindableAttribute` "interface".

/**
 * @class BindableAttribute
 * Only classes which implement this interface can be passed to the `attributes` option
 * of a `WebGL1Program`.
 *
 * Do not use directly - rather, use subclasses such as `SingleAttribute` or `InterleavedAttributes`.
 */

/**
 * @section
 * @aka BindableAttribute options
 * @aka BindableAttributeOptions
 *
 * Concrete implementations of `BindableAttribute` should expose these options.
 *
 * @option type: prototype
 * A specific subclass of `TypedArray` defining the data format.
 *
 * Valid values for WebGL1 are: `Int8Array`, `Uint8Array`, `Uint8ClampedArray`,
 * `Int16Array` and `Float32Array`.
 *
 *
 * @option glslType: String
 * The GLSL type associated with this attribute. One of `float`, `vec2`, `vec3`, `vec4`.
 *
 * This also defines the number of components for this attribute (1, 2, 3 or 4, respectively).
 *
 *
 * @option normalized: Boolean = false
 * Whether the values in this attribute are normalized into the -1..1 (signed) or
 * 0..1 (unsigned) range when accesed from within GLSL.
 *
 * Only has effect when `type` is an integer array (`Int8Array`, `Uint8Array`, `Int16Array`, `Uint16Array`, `Int32Array` or `Uint32Array`).
 *
 * Note that the normalization of signed integers for the minimum representable integer
 * (-128 = -2⁷ for `Uint8Array`, -32768 = -2¹⁵ for `Uint16Array`,
 * (-and -2³¹ for `Uint32Array`) is clamped to -1.
 *
 *
 * @section Internal methods
 * @uninheritable
 * @method bindWebGL1(location: Number): this
 * Binds the attribute represented by self to the given `location` in the active GLSL program.
 *
 * (`location` is kinda a misnomer, since it's more like an offset in one of the program's
 * symbol table.)
 *
 * This is expected to be called from `WebGL1Program` only.
 *
 *
 * @method getGlslType(): String
 * Returns a `String` with the GLSL type for this attribute.
 *
 * This is expected to be called from `WebGL1Program` only.
 *
 * @section
 *
 * @method debugDump(start?: Number, length?: Number): Array of TypedArray
 * Returns a readable representation of the current attribute values. This is
 * only possible on growable attribute storages (i.e. those defined with
 * a `growFactor` greater than zero).
 *
 * If `start` and `length are defined, then only that range of vertex attributes
 * shall be returned. Otherwise all known data is returned.
 *
 * This is a costly operation, and should be only used for manual debugging purposes.
 */
