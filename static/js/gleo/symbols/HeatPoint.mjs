import AcetateExtrudedPoint from "../acetates/AcetateExtrudedPoint.mjs";
import { ScalarField } from "../fields/Field.mjs";
import getBlendEquationConstant from "../util/getBlendEquationConstant.mjs";

/**
 * @class AcetateHeatPoint
 * @inherits AcetateExtrudedPoint
 * @relationship drawnOn ScalarField
 *
 * An `Acetate` to place `HeatPoint`s into a scalar field.
 *
 */

class AcetateHeatPoint extends AcetateExtrudedPoint {
	#blendEquation;

	/**
	 * @constructor AcetateHeatPoint(target: , opts: AcetateHeatPoint Options)
	 */
	constructor(
		target,
		{
			/**
			 * @option blendEquation: String
			 * Defines how the symbols' intensity affects the value of the
			 * scalar field. The default is `"ADD"`, which means the intensity
			 * is added to the scalar field. Other possible values are `"SUBTRACT"`,
			 * `"MIN"` and `"MAX"`.
			 */
			blendEquation = "ADD",

			...opts
		} = {}
	) {
		super(target, {
			zIndex: 2000,
			...opts,

			// Heatpoint acetates are not gonna be interactive - otherwise
			// confusion will ensue when several heatpoints overlap each other.
			// Arguably interactivity could be achieved by a more complex shader,
			// leveraging the depth buffer - only the heatpoint closest to
			// the camera would be registered.
			interactive: false,
		});

		this._attrs = new this.glii.SingleAttribute({
			usage: this.glii.STATIC_DRAW,
			size: 1,
			growFactor: 1.2,

			// Heat intensity
			glslType: "float",
			type: Float32Array,
			normalized: false,
		});

		this.#blendEquation = getBlendEquationConstant(this.glii, blendEquation);
	}

	/**
	 * @property PostAcetate: ScalarField
	 * Signals that this `Acetate` isn't rendered as a RGBA8 texture,
	 * but instead uses a scalar field.
	 */
	static get PostAcetate() {
		return ScalarField;
	}

	// This is the definition for the *first* program, turning HeatPoint
	// symbols into a float32 texture (AKA "scalar field")
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
				gl_Position = vec4(
					vec3(aCoords, 1.0) * uTransformMatrix +
					vec3(aExtrude * uPixelSize, 0.0)
					, 1.0);
			`,
			varyings: {
				vIntensity: "float",
			},
			fragmentShaderMain: ` gl_FragColor.r = vIntensity; `,
			target: this._inAcetate.framebuffer,
			blend: {
				equationRGB: this.#blendEquation,
				// equationRGB: this.glii.FUNC_ADD,
				equationAlpha: this.#blendEquation,

				/**
				 * NOTE: When using blend modes that multiply the src RGB
				 * components by the scr alpha, then the fragment shader
				 * needs to set the alpha component. i.e. there's a need to
				 * set `gl_FragColor.a = 1.;`.
				 *
				 * This is counter-intuitive, since the R32F texture has no
				 * alpha component. **BUT**, the alpha component of
				 * gl_FragColor lives until the blend operation.
				 *
				 * By setting the srcRGB blend parameter to `ONE`, the output
				 * is unaffected by the alpha component.
				 */
				srcRGB: this.glii.ONE,
				srcAlpha: this.glii.ZERO,
				dstRGB: this.glii.ONE,
				dstAlpha: this.glii.ZERO,
			},
		};
	}

	resize(x, y) {
		super.resize(x, y);
		this._program._target = this._inAcetate.framebuffer;
		const dpr2 = (devicePixelRatio ?? 1) * 2;

		const invCellSize = 1 / (this._inAcetate?.cellSize ?? 1);
		this._program.setUniform("uPixelSize", [
			(invCellSize * dpr2) / x,
			(invCellSize * dpr2) / y,
		]);
	}

	_getStridedArrays(maxVtx, maxIdx) {
		return [
			// Static extrusion
			this._extrusions.asStridedArray(maxVtx),
			// Field intensity
			this._attrs.asStridedArray(maxVtx),
			// Triangle indices
			this._indices.asTypedArray(maxIdx),
		];
	}
}

import ExtrudedPoint from "./ExtrudedPoint.mjs";

/**
 * @class HeatPoint
 * @inherits ExtrudedPoint
 * @relationship drawnOn AcetateHeatPoint
 *
 * A point for a heatmap - an abstract blob that will increase/change
 * the colour on the `ScalarField` it is in, typically a
 * `HeatMap`.
 */

export default class HeatPoint extends ExtrudedPoint {
	/// @section Static properties
	/// @property Acetate: Prototype of AcetateHeatPoint
	// The `Acetate` class that draws this symbol.
	static Acetate = AcetateHeatPoint;

	#radius;
	#intensity;

	/**
	 * @constructor HeatPoint(geom: Geometry, opts?: HeatPoint Options)
	 */
	constructor(
		geom,
		{
			/**
			 * @section
			 * @aka HeatPoint Options
			 * @option radius: Number = 20; Radius of the circle, in CSS pixels
			 * @option intensity: Number = 10
			 * Intensity of the point, at its center. The intensity fades linearly
			 * towards its edge (half intensity at half the radius, etc)
			 */
			radius = 20,
			intensity = 10,
			...opts
		} = {}
	) {
		super(geom, opts);
		this.#radius = radius;
		this.#intensity = intensity;

		// Length of circumference
		const length = Math.PI * 2 * this.#radius;
		// Divide in triangles so there's a triangle per...
		// 6 pixels of circumference length. That should be enough.
		this.steps = Math.max(6, Math.ceil(length / 6));

		this.attrLength = this.steps + 1;
		this.idxLength = this.steps * 3;
	}

	_setGlobalStrides(strideExtrusion, strideIntensity, typedIdxs) {
		// Radian increment per step
		const ɛ = (Math.PI * 2) / this.steps;

		const ρ = this.#radius;
		const [Δx, Δy] = this.offset;

		// Attributes start with the center point
		strideExtrusion.set([Δx, Δy], this.attrBase);
		strideIntensity?.set([this.#intensity], this.attrBase);

		let θ = 0;
		let vtx = this.attrBase + 1;
		let idx = this.idxBase;

		// Intensity is a single attribute, and all values but the first must be set to zero
		strideIntensity?.set(new Array(this.attrLength - 1).fill(0), vtx);

		for (let i = 0; i < this.steps; i++) {
			strideExtrusion.set([Math.sin(θ) * ρ + Δx, Math.cos(θ) * ρ + Δy], vtx);

			// Vertices of the i-th triangle are: center, current, next
			if (i !== this.steps - 1) {
				typedIdxs?.set([this.attrBase, vtx, vtx + 1], idx);
			} else {
				typedIdxs?.set([this.attrBase, vtx, this.attrBase + 1], idx);
			}

			θ += ɛ;
			vtx++;
			idx += 3;
		}
	}

	_setStridedExtrusion(strideExtrusion) {
		this._setGlobalStrides(strideExtrusion);
	}
}
