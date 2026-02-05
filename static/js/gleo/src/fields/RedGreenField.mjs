import { VectorField } from "./Field.mjs";
import glslFloatify from "../util/glslFloatify.mjs";

/**
 * @class RedGreenField
 * @inherits VectorField
 *
 * A `VectorField` which displays as a simple red-green bivariate ramp.
 *
 *
 * Accepts `SlopePoint`s, etc as symbols.
 *
 * @example
 *
 * ```
 * const field = new RedGreenField(map, {maxValue: 10});
 *
 * new SlopePoint(geometry, {intensity: 10}).addTo(field);
 * ```
 */

export default class GreyScaleField extends VectorField {
	#minValue;
	#maxValue;

	constructor(
		target,
		{
			/**
			 * @section RedGreenField Options
			 * @option minValue: Number = 0
			 * Minimum value of the scalar field taken into consideration
			 *
			 * @option maxValue: Number = 1
			 * Maximum value of the scalar field taken into consideration
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
			`,
			fragmentShaderMain: `
				vec2 value = texture2D(uField, vUV).xy;
				gl_FragColor.r = mix(
					0.,
					1.,
					(value.x - min) / (range)
				);
				gl_FragColor.g = mix(
					0.,
					1.,
					(value.y - min) / (range)
				);
				gl_FragColor.a = max(gl_FragColor.r, gl_FragColor.g);
			`,
		};
	}
}
