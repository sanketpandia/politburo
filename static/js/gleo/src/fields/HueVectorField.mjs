import { VectorField } from "./Field.mjs";
import glslFloatify from "../util/glslFloatify.mjs";
import hsv2rgb from "../util/glslHsv2rgb.mjs";

/**
 * @class HueVectorField
 * @inherits VectorField
 *
 * A `VectorField` which displays vector angle as hue, and vector length
 * as opacity.
 *
 *
 * Accepts `SlopePoint`s, etc as symbols.
 *
 * @example
 *
 * ```
 * const field = new HueVectorField(map, {maxValue: 10});
 *
 * new SlopePoint(geometry, {intensity: 10}).addTo(field);
 * ```
 */

export default class HueVectorField extends VectorField {
	#minValue;
	#maxValue;

	constructor(
		target,
		{
			/**
			 * @section HueVectorField Options
			 * @option minValue: Number = 0
			 * Minimum vector length to be taken into consideration. Under this
			 * length, opacity will be zero.
			 *
			 * @option maxValue: Number = 1
			 * Maximum vector length to be taken into consideration. Over this
			 * length, opacity will be one.
			 */
			minValue = 0,
			maxValue = 1,
			...opts
		} = {}
	) {
		super(target, opts);

		this.#minValue = minValue;
		this.#maxValue = maxValue;
	}

	// A program that takes scalar field textures and renders a simple,
	// two-stop gradient
	glProgramDefinition() {
		const opts = super.glProgramDefinition();

		return {
			...opts,
			fragmentShaderSource: `
			const float min = ${glslFloatify(this.#minValue)};
			const float range = ${glslFloatify(this.#maxValue - this.#minValue)};
			${hsv2rgb}`,
			fragmentShaderMain: `
				vec2 value = texture2D(uField, vUV).xy;

				float theta = atan(value.y, value.x) / 6.28318530717958647693;
				float rho = length(value);

				gl_FragColor.rgb = hsv2rgb(vec3(theta, 1., 1.));
				gl_FragColor.a = (rho - min) / range;
			`,
		};
	}
}
