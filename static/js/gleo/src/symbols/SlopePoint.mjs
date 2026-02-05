import { VectorField } from "../fields/Field.mjs";

import HeatPoint from "./HeatPoint.mjs";

/**
 * @class AcetateSlopePoint
 * @inherits AcetateHeatPoint
 * @relationship drawnOn VectorField
 *
 * An `Acetate` to place `SlopePoint`s into a vector field
 *
 */

class AcetateSlopePoint extends HeatPoint.Acetate {
	/**
	 * @property PostAcetate: VectorField
	 * Signals that this `Acetate` isn't rendered as a RGBA8 texture,
	 * but instead uses a vector field.
	 */
	static get PostAcetate() {
		return VectorField;
	}

	glProgramDefinition() {
		const opts = super.glProgramDefinition();
		return {
			...opts,
			attributes: {
				...opts.attributes,
				aIntensity: this._attrs,
			},
			uniforms: {
				uPixelSize: "vec2",
				...opts.uniforms,
			},
			vertexShaderMain: `
				vIntensity = aIntensity;
				vExtrude = aExtrude * uPixelSize;
				gl_Position = vec4(
					vec3(aCoords, 1.0) * uTransformMatrix +
					vec3(vExtrude, 0.0)
					, 1.0);
			`,
			varyings: {
				vIntensity: "float",
				vExtrude: "vec2",
			},
			fragmentShaderMain: ` gl_FragColor.rg = vIntensity * normalize(vExtrude); `,
		};
	}
}

/**
 * @class SlopePoint
 * @inherits HeatPoint
 * @relationship drawnOn AcetateSlopePoint
 *
 * A point for a vector field - will add the intensity horizontally to the first
 * component of the vector field and vertically to the second component.
 */

export default class SlopePoint extends HeatPoint {
	/// @section Static properties
	/// @property Acetate: Prototype of AcetateHeatPoint
	// The `Acetate` class that draws this symbol.
	static Acetate = AcetateSlopePoint;
}
