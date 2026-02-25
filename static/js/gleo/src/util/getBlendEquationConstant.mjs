/**
 * @namespace Util
 * @function getBlendEquationConstant(glii: GliiFactory, mode: String): Number
 * Given a Glii context and a blend equation string (`"ADD"`, `"SUBTRACT"`,
 * `"MIN"`, `"MAX"`), returns the appropriate numeric GL constant.
 */
export default function getBlendEquationConstant(glii, mode) {
	if (mode === "ADD") {
		return glii.FUNC_ADD;
	} else if (mode === "SUBTRACT") {
		return glii.FUNC_SUBTRACT;
	}
	const isWebGL2 = glii.isWebGL2();

	if (isWebGL2) {
		if (mode === "MIN") {
			return glii.MIN;
		} else if (mode === "MAX") {
			return glii.MAX;
		}
	} else {
		let ext;
		try {
			ext = glii.loadExtension("EXT_blend_minmax");
		} catch (ex) {
			throw new Error(
				"Cannot use min/max blend equation: context is not WebGL2, and does not support the EXT_blend_minmax extension"
			);
		}
		if (mode === "MIN") {
			return ext.MIN_EXT;
		} else if (mode === "MAX") {
			return ext.MAX_EXT;
		}
	}
	throw new Error(
		`Unknown blend equation mode "${mode}". Should be one of: ADD, SUBTRACT, MIN or MAX.`
	);
}
