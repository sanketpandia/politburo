import HeatMap from "./HeatMap.mjs";

/**
 * @class ScaledHeatMap
 * @inherits HeatMap
 *
 * As `HeatMap`, but the ramp stops change with the map's scale. While a standard
 * `HeatMap` has less apparent heat when zooming in, a `ScaledHeatMap` aims to
 * maintain apparent heat on the screen when zooming into hot spots.
 *
 * The colour ramp should be specified as if the map scale was 1 (i.e. one CSS
 * pixel per one CRS unit).
 *
 */

export default class ScaledHeatMap extends HeatMap {
	glProgramDefinition() {
		const opts = super.glProgramDefinition();

		return {
			...opts,
			fragmentShaderMain: opts.fragmentShaderMain.replace(
				"float value = texture2D(uField, vUV).x;",
				"float value = texture2D(uField, vUV).x / uScale;"
			),
			uniforms: {
				...opts.uniforms,
				uScale: "float",
			},
		};
	}

	runProgram() {
		this._programs.setUniform("uScale", this.platina.scale);
		super.runProgram();
	}
}
