import { ScalarField } from "./Field.mjs";
import glslFloatify from "../util/glslFloatify.mjs";
import glslVecNify from "../util/glslVecNify.mjs";
import parseColour from "../3rd-party/css-colour-parser.mjs";

/**
 * @class GreyscaleField
 * @inherits ScalarField
 *
 * A `ScalarField` which displays as a simple 2-colour ramp.
 * By default, uses black for `0` and white for `1` with
 * greyscale in-between.
 *
 * Accepts `HeatPoint`s, `HeatStroke`s, etc as symbols.
 *
 * @example
 *
 * ```
 * const field = new GreyscaleField(map);
 *
 * new HeatPoint(geometry, {intensity: 1}).addTo(field);
 * ```
 */

export default class GreyScaleField extends ScalarField {
	#minValue;
	#maxValue;
	#minColour;
	#maxColour;

	constructor(
		target,
		{
			/**
			 * @section GreyScaleField Options
			 * @option minValue: Number = 0
			 * Minimum value of the scalar field taken into consideration
			 *
			 * @option minColour: Colour = [0, 0, 0, 255]
			 * Colour given to the minimum (or below-minimum) scalar values
			 *
			 * @option maxValue: Number = 1
			 * Maximum value of the scalar field taken into consideration
			 *
			 * @option maxColour: Colour = [255, 255, 255, 255]
			 * Colour given to the maximum (or over-maximum) scalar values
			 */
			minValue = 0,
			maxValue = 1,
			minColour = [0, 0, 0, 255],
			maxColour = [255, 255, 255, 255],
			...opts
		} = {}
	) {
		super(target, opts);

		this.#minValue = minValue;
		this.#maxValue = maxValue;
		this.#minColour = parseColour(minColour).map((b) => b / 255);
		this.#maxColour = parseColour(maxColour).map((b) => b / 255);
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
			const vec4 minColour = ${glslVecNify(this.#minColour)};
			const vec4 maxColour = ${glslVecNify(this.#maxColour)};
			`,
			fragmentShaderMain: `
				float value = texture2D(uField, vUV).x;
				gl_FragColor = mix(
					minColour,
					maxColour,
					(value - min) / (range)
				);
			`,
		};
	}
}
