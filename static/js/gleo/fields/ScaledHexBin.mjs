import HexBin from "./HexBin.mjs";

/**
 * @class ScaledHexBin
 * @inherits HexBin
 *
 * As `HexBin`, but the ramp stops change with the map's scale. While a standard
 * `HexBin` has less apparent heat when zooming in, a `ScaledHexBin` aims to
 * maintain apparent heat on the screen when zooming into hot spots.
 *
 * This is akin to the difference between a `ScaledHeatMap` and a `HeatMap`.
 *
 * The colour ramp should be specified as if the map scale was 1 (i.e. one CSS
 * pixel per one CRS unit).
 *
 */

export default class ScaledHexBin extends HexBin {
	glProgramDefinition() {
		const opts = super.glProgramDefinition();

		return {
			...opts,
			vertexShaderMain: opts.vertexShaderMain.replace(
				"float value = texture2D(uField, aUV).x;",
				"float value = texture2D(uField, aUV).x / uScale;"
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
