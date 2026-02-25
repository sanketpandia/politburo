// Partial regexp for precision qualifiers:
// A capturing group that matches lowp|mediump|highp, then one or more spaces,
// all of it optional.
const precisionQualifiers = "((?:(?:lowp)|(?:mediump)|(?:highp))\\s+)?";

// Accepted GLSL types for attribute buffers
// mat2/mat3/mat4 not yet here, see https://gitlab.com/IvanSanchez/glii/-/issues/18
// A capturing group that matches float|vec2|vec3|vec4
const glsl1AttribTypes = "((?:float)|(?:vec[2-4]))";

// Accepted GLSL types for declaration of varyings. Reused for uniforms.
// A capturing group that matches float|int|bool|vec2|vec3|vec4|ivec2|ivec3|ivec4|
// bvec2|bvec3|bvec4|mat2|mat3|mat4.
const glsl1VaryingTypes = "((?:float)|(?:int)|(?:bool)|(?:[ib]?vec[2-4])|(?:mat[2-4]))";

const regexpAttrib = new RegExp(
	// Nothing before.
	"^" +
		// First capturing group: optional precision qualifier
		precisionQualifiers +
		// Second capturing group: float|vecN.
		glsl1AttribTypes +
		// Nothing afterwards.
		"$"
);

const regexpVarying = new RegExp("^" + precisionQualifiers + glsl1VaryingTypes + "$");

/**
 * Parses a string containing:
 * - An (optional) precision qualifier
 * - A GLSL type for an attribute
 *
 * Returns a string of the form [precision, type]
 */
export function parseGlslAttribType(str) {
	const match = regexpAttrib.exec(str);
	if (!match) {
		throw new Error(
			`Invalid GLSL type. Expected float|vec2|vec3|vec4 (optionally prepended by lowp|mediump|highp), but found "${str}"`
		);
	}
	const [_, precision, type] = match;
	return [precision, type];
}

/**
 * Parses a string containing:
 * - An (optional) precision qualifier
 * - A GLSL type for a varying (reused for uniforms)
 *
 * Returns a string of the form [precision, type]
 */
export function parseGlslVaryingType(str) {
	const match = regexpVarying.exec(str);
	if (!match) {
		throw new Error(
			`Invalid GLSL type. Expected float|vec(234)|(ib)vec(234)|mat(234) (optionally prepended by lowp|mediump|highp), but found "${str}"`
		);
	}
	const [_, precision, type] = match;
	return [precision, type];
}

export const parseGlslUniformType = parseGlslVaryingType;
