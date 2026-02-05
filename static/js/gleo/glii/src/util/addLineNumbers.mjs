/**
 * Internal helper function.
 *
 * Adds line numbers to a \n-delimited string. Just for printing out
 * the line numbers of GLSL shaders.
 */
export default function addLineNumbers(str) {
	return str
		.split("\n")
		.map((l, i) => `${i}: ${l}`)
		.join("\n");
}
