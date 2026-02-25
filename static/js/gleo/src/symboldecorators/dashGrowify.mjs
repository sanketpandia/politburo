/**
 * @namespace dashGrowify
 * @inherits mValuifyLine
 *
 * Very similar to `trailify`: turns a XY linestring symbol into a XYM linestring
 * symbol.
 *
 * Rather than changing the opacity of the line depending on the M value,
 * `dashGrowify` changes the `dashArray`, so the dashes grow apart as the M values
 * differ more.
 */

import mValuifyLine from "./mValuifyLine.mjs";

export default function dashGrowify(base) {
	const mValuifiedBase = mValuifyLine(base);

	/**
	 * @miniclass Trailified Acetate (trailify)
	 */
	class DashGrowifiedAcetate extends mValuifiedBase.Acetate {
		constructor(target, { ...opts } = {}) {
			/**
			 * @option opacityStart: Number = 0
			 * The initial value for trailified symbols near the acetate's interval start,
			 * i.e. the opacity at the "tail" of a symbol.
			 *
			 * @option opacityEnd: Number = 1
			 * The initial value for trailified symbols near the acetate's interval end,
			 * i.e. the opacity at the "head" of a symbol.
			 */
			super(target, opts);
		}

		glProgramDefinition() {
			const opts = super.glProgramDefinition();

			if (!opts.attributes.aDashArray) {
				throw new Error(
					"DashGrowify can only be applied to symbols which implement line dashing"
				);
			}

			/*
			const vertexShaderMain = opts.vertexShaderMain.replace(
			"vDashArray = aDashArray",
			`
			vMValuePct = (aMCoord - uMThreshold.x) / uMThreshold.y;

			vDashArray = aDashArray;
			vDashArray.xz *= vMValuePct;
			`);*/

			return {
				...opts,
				vertexShaderMain: `${opts.vertexShaderMain}
				vMValuePct = (aMCoord - uMThreshold.x) / uMThreshold.y;
				vDashArray.xz *= vMValuePct;
				`,
				// uniforms: {
				// 	...opts.uniforms,
				// 	uTrailOpacity: "vec2", // Lower opacity, higher opacity
				// },
				varyings: {
					...opts.varyings,
					vMValuePct: "float",
				},
				fragmentShaderMain: `
					if (vMValuePct < 0.0) {
						discard;
					} else if (vMValuePct > 1.0) {
						discard;
					} else {
						${opts.fragmentShaderMain}
					}
				`,
			};
		}
	}

	/**
	 * @miniclass Trailified GleoSymbol (trailify)
	 *
	 * A "trailified" symbol accepts this additional constructor options:
	 */
	return class DashGrowifiedSymbol extends mValuifiedBase {
		static Acetate = DashGrowifiedAcetate;
	};
}
