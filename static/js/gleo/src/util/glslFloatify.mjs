/**
 * @namespace Util
 * @function glslFloatify(n: Number): String
 *
 * Turns a `Number` into a `String` which is a valid GLSL float representation
 * for that number.
 */

export default function glslFloatify(number) {
	const str = number.toString();
	if (str.includes(".")) {
		return str;
	} else {
		return `${str}.`;
	}
}
