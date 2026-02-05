import { default as AbstractAttributeSet } from "./AbstractAttributeSet.mjs";
import { registerFactory } from "../GliiFactory.mjs";
import { default as typeMap } from "../util/typeMap.mjs";
import { parseGlslAttribType } from "../util/parseGlslType.mjs";
import stridedArrays from "./StridedTypedArrays.mjs";

/**
 * @class SingleAttribute
 * @inherits AbstractAttributeSet
 * @inherits BindableAttribute
 *
 * Represents a `gl.ARRAY_BUFFER` holding data for a single attribute.
 *
 * @example
 *
 * ```
 * const posInPlane = new glii.SingleAttribute({
 * 	type: Float32Array
 * 	glslType: 'vec2'
 * });
 *
 * const rgbaColour = new glii.SingleAttribute({
 * 	type: Uint8Array,
 * 	normalized: true,
 * 	glslType: 'vec4'
 * });
 * ```
 */

/// TODO: Somehow implement integer GLSL types for WebGL2.

export default class SingleAttribute extends AbstractAttributeSet {
	constructor(gl, options) {
		/**
		 * @section
		 * @aka SingleAttribute options
		 * @option type: prototype = Float32Array
		 * A specific subclass of `TypedArray` defining the data format
		 */
		const type = options.type || Float32Array;
		const bytesPerElement = type.BYTES_PER_ELEMENT;

		/**
		 * @option glslType: String = 'float'
		 * The GLSL type associated with this attribute. One of `float`, `vec2`, `vec3`, `vec4`, with an optional precision qualifier after it (`lowp`, `mediump` or
		 * `highp`, e.g. `"mediump vec3"`).
		 *
		 * This also defines the number of components for this attribute (1, 2, 3 or 4, respectively).
		 *
		 * `matN` attributes are not supported (yet), see https://gitlab.com/IvanSanchez/glii/-/issues/18
		 */
		const fullGlslType = options.glslType || "float";
		const [glslPrecision, glslType] = parseGlslAttribType(fullGlslType);
		if (!(glslType in AbstractAttributeSet.GLSL_TYPE_COMPONENTS)) {
			throw new Error(
				"Invalid value for the `glslType` option; must be `float`, `vec2`, `vec3`, or `vec4`."
			);
		}
		const componentCount = AbstractAttributeSet.GLSL_TYPE_COMPONENTS[glslType];

		super(gl, options, bytesPerElement * componentCount);

		this._glslType = fullGlslType;
		this._componentCount = componentCount;
		this._glType = typeMap.get(type);

		this._normalized = options.normalized;

		/**
		 * @method set(index: Number, value: Number): this
		 * Alias of `setNumber`, available when `glslType` is `float`.
		 * @alternative
		 * @method set(index: Number, values: [Number]): this
		 * Alias of `setArray`, available when `glslType` is `vec2`, `vec3` or `vec4`. `values`
		 * must be an array of length 2, 3 or 4 (respectively).
		 */
		if (options.glslType === "float") {
			this.set = this.setNumber;
		} else {
			this.set = this.setArray;
		}

		this._recordBuf = new type(componentCount);
		this._arrayType = type;
	}

	/**
	 * @method setNumber(index: Number, value: Number): this
	 * Sets the value for the `index`th vertex. Valid when `glslType` is `float`.
	 */
	setNumber(index, value) {
		this._recordBuf[0] = value;
		super.setBytes(index, 0, this._recordBuf);
		return this;
	}

	/**
	 * @method setArray(index: Number, values: Array of Number): this
	 * Sets the values for the `index`th vertex. Valid when `glslType` is `vec2`, `vec3` or `vec4`. `val` must be an array of length 2, 3 or 4 (respectively).
	 */
	setArray(index, values) {
		if (values.length !== this._componentCount) {
			throw new Error(
				`Expected ${this._componentCount} values but got ${values.length}.`
			);
		}
		this._recordBuf.set(values);
		super.setBytes(index, 0, this._recordBuf);
		return this;
	}

	/**
	 * @section Batch update methods
	 *
	 * These methods are a less convenient, but more performant, way of updating
	 * attribute data.
	 *
	 * For a `SingleAttribute`, the workflow is:
	 * - Call `asTypedArray()`
	 * - Update the values in the returned typed array (using typed array offsets,
	 *   avoiding array concatenations)
	 * - Call `commit()`
	 *
	 * These methods need the attribute set to have been created with a `growFactor`
	 * larger than zero.
	 *
	 * @method asStridedArray(minSize: Number): StridedTypedArray
	 * Returns a view of the internal in-RAM data buffer, as a `TypedArray` of
	 * the appropriate type.
	 */
	asStridedArray(minSize) {
		if (minSize > this._size) {
			this._grow(minSize);
		}
		return new (stridedArrays.get(this._arrayType))(
			this._byteData.buffer,
			this._componentCount,
			0
		);
	}

	/**
	 * @method multiSet(index: Number, values: Array of Number): this
	 *
	 * Batch version of `setArray()`.
	 *
	 * Sets values for several contiguous values at once, starting with the `index`th.
	 *
	 * The length of `values` must be a multiple of 2, 3 or 4 when `glslType` is `vec2`,
	 * `vec3` or `vec4` (respectively). `values` must be a flat array (i.e. run
	 * `.flat()` if needed).
	 */
	multiSet(index, values) {
		if (values.length % this._componentCount) {
			throw new Error(
				`Expected values to be a multiple of ${this._componentCount} but got ${values.length}.`
			);
		}
		super.setBytes(index, 0, this._arrayType.from(values));
		return this;
	}

	// Method implementing `BindableAttribute` interface.
	bindWebGL1(location) {
		const gl = this._gl;
		gl.bindBuffer(gl.ARRAY_BUFFER, this._buf);
		gl.enableVertexAttribArray(location);
		gl.vertexAttribPointer(
			location,
			this._componentCount,
			this._glType,
			this._normalized,
			this._recordSize, // stride
			0 // offset
		);
	}

	// Method implementing `BindableAttribute` interface.
	getGlslType() {
		return this._glslType;
	}

	/**
	 * @method destroy(): this
	 * Tells WebGL to free resources associated with this `SingleAttribute`. Use
	 * when the `SingleAttribute` won't be used anymore.
	 *
	 * After being destroyed, WebGL programs should not use the destroyed `SingleAttribute`.
	 */
	destroy() {
		this._gl.deleteBuffer(this._buf);
	}

	debugDump(start, length) {
		start ??= 0;
		length ??= this._size;
		const end = start + length;


		const view = new this._arrayType(this._byteData.buffer);

		// return Array.from(new Array(this._size), (_, i) => {
		const result = new Array(this._size);

		for (let i=start; i<end; i++) {
			const j = i * this._componentCount;
			result[i] = view.subarray(j, j + this._componentCount);
		};
		return result;
	}
}

/**
 * @factory GliiFactory.SingleAttribute(options: SingleAttribute options)
 * @class Glii
 * @section Class wrappers
 * @property SingleAttribute(options: SingleAttribute options): Prototype of SingleAttribute
 * Wrapped `SingleAttribute` class
 */
registerFactory("SingleAttribute", function (gl) {
	return class WrappedSingleAttribute extends SingleAttribute {
		constructor(options) {
			super(gl, options);
		}
	};
});
