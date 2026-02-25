import { default as typeMap } from "../util/typeMap.mjs";

function stridify(arrayType) {
	/**
	 * @class StridedTypedArray
	 *
	 * This is not a standalone class, but a decorated `TypedArray`, with
	 * the ability to store items based on a *record index* instead of an
	 * index relative to the number of elements in the array.
	 *
	 * @example
	 * ```
	 * // Get a strided array representation of the 3rd field in an interleaved
	 * // attribute set (position `2` since it's zero-indexed), capable of
	 * // holding at least 1000 vertices
	 * let strided = myInterlavedAttributes.asStridedArray(2, 1000);
	 *
	 * // Assuming the field is a `vec3`, this will store data for the 42th vertex
	 * // (`41` because, again, it's zero-indexed)
	 * strided.set([x, y, z], 41);
	 * ```
	 */
	return class StridedTypeArray extends arrayType {
		#stride;
		#offset;

		constructor(buffer, stride, offset) {
			super(buffer);

			this.#stride = stride;
			this.#offset = offset;
		}

		/**
		 * @method set(values: Array of Number, index: Number = 0): undefined
		 * Sets the given values (as per
		 * [`TypedArray.set()`](https://developer.mozilla.org/en-US/docs/Web/JavaScript/Reference/Global_Objects/TypedArray/set)),
		 * but taking an `index` (relative to the number of records) instead of
		 * an `offset` (relative to the number of elements in the array).
		 *
		 * No sanity checks are performed on the length on the input data. (In
		 * other words: if a `StridedTypeArray` represents a `vec2` attribute,
		 * then `values` should have a length of `2`, idem for `vec2` and `vec4`).
		 */
		set(values, index) {
			super.set(values, index * this.#stride + this.#offset);
		}
	};
}

const stridedArrays = new Map(Array.from(typeMap.keys()).map((t) => [t, stridify(t)]));

export default stridedArrays;
