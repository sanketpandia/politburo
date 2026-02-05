import glslFloatify from "./glslFloatify.mjs";

/**
 * @namespace Util
 * @function glslVecNify(a: Array of Number): String
 *
 * Turns an `Array` of two/three/four `Number`s into a `String` which
 * is a valid `vec2`/`vec3`/`vec4` GLSL representation for that array.
 */

export default function glslVecNify(arr) {
	const l = arr.length;
	if (l < 2 || l > 4) {
		throw new Error(
			"Cannot turn array into vec2/vec3/vec4 representation: wrong length"
		);
	}
	return `vec${l}( ${arr.map(glslFloatify).join(",")} )`;
}
