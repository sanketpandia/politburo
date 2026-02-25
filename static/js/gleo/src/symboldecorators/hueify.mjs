import parseColour from "../3rd-party/css-colour-parser.mjs";
import hsv2rgb from "../util/glslHsv2rgb.mjs";

/**
 * @namespace hueify
 * @inherits Symbol Decorator
 *
 * Takes a symbol that is normally rendered as the interpolation of several
 * solid colours (including `Halo`, `DealunayMesh`, etc), and turns the
 * RGB(A) interpolation into a HSV(A) interpolation.
 */

// From https://stackoverflow.com/a/17243070 :
function rgb2hsv(r, g, b, a) {
	let max = Math.max(r, g, b),
		min = Math.min(r, g, b),
		d = max - min,
		h,
		s = max === 0 ? 0 : d / max;

	switch (max) {
		case min:
			h = 0;
			break;
		case r:
			h = g - b + d * (g < b ? 6 : 0);
			h /= 6 * d;
			break;
		case g:
			h = b - r + d * 2;
			h /= 6 * d;
			break;
		case b:
			h = r - g + d * 4;
			h /= 6 * d;
			break;
	}

	return [h * 255, s * 255, max, a];
}

export default function hueify(base) {
	if (!("_parseColour" in base)) {
		throw new Error(
			`The symbol class to be hueified (${base.constructor.name}) doesn't seem to operate with colours`
		);
	}

	if (base.Acetate.PostAcetate) {
		// Throw error if the symbol operates on a scalar/vector field
		throw new Error(
			`The symbol class to be hueified (${base.constructor.name}) doesn't seem to be operate with colours`
		);
	}

	class HueifiedAcetate extends base.Acetate {
		glProgramDefinition() {
			const opts = super.glProgramDefinition();

			return {
				...opts,
				fragmentShaderSource: hsv2rgb + opts.fragmentShaderSource,
				fragmentShaderMain:
					opts.fragmentShaderMain +
					`
				gl_FragColor.rgb = hsv2rgb(gl_FragColor.rgb);
				`,
			};
		}
	}

	class HueifiedSymbol extends base {
		static Acetate = HueifiedAcetate;

		static _parseColour(c) {
			return rgb2hsv(...parseColour(c));
		}
	}

	return HueifiedSymbol;
}
