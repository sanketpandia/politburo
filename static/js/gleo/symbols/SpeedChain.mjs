import Chain from "./Chain.mjs";
import AcetateVertices from "../acetates/AcetateVertices.mjs";

import { VectorField } from "../fields/Field.mjs";
import getBlendEquationConstant from "../util/getBlendEquationConstant.mjs";

/**
 * @class AcetateSpeedChain
 * @inherits AcetateChain
 * @relationship drawnOn VectorField
 *
 * An `Acetate` to place `SpeedChain`s into a scalar field.
 *
 */
class AcetateSpeedChain extends Chain.Acetate {
	#blendEquation;

	/**
	 * @constructor AcetateSpeedChain(target: GliiFactory, opts: AcetateSpeedChain Options)
	 */
	constructor(
		target,
		{
			/**
			 * @option blendEquation: String
			 * Defines how the symbols' vector affects the value of the
			 * vector field. The default is `"ADD"`, which means the intensity
			 * is added to the vector field. Other possible values are `"SUBTRACT"`,
			 * `"MIN"` and `"MAX"` (which work independently on the X and Y components).
			 */
			blendEquation = "ADD",

			...opts
		} = {}
	) {
		super(target, {
			zIndex: 2000,
			...opts,

			interactive: false,
		});

		this._attrs = new this.glii.InterleavedAttributes(
			{
				size: 1,
				growFactor: 1.2,
				usage: this.glii.STATIC_DRAW,
			},
			[
				{
					// Vector field intensity
					glslType: "vec2",
					type: Float32Array,
					normalized: false,
				},
				{
					// Width, in 256ths of CSS pixels.
					// Used for fading.
					glslType: "float",
					type: Uint16Array,
					normalized: false,
				},
			]
		);

		this.#blendEquation = getBlendEquationConstant(this.glii, blendEquation);
	}

	/**
	 * @property PostAcetate: VectorField
	 * Signals that this `Acetate` isn't rendered as a RGBA8 texture,
	 * but instead uses a vector field.
	 */
	static get PostAcetate() {
		return VectorField;
	}

	// This is the definition for the *first* program, turning HeatPoint
	// symbols into a float32 texture (AKA "scalar field")
	glProgramDefinition() {
		const opts = super.glProgramDefinition();
		return {
			...opts,
			attributes: {
				...opts.attributes,
				aIntensity: this._attrs.getBindableAttribute(0),
			},
			uniforms: {
				uPixelSize: "vec2",
				...opts.uniforms,
			},
			vertexShaderMain: `
				vIntensity = aIntensity;
				vLength = aLength / uScale;
				vWidth = aWidth / 512.;

				gl_Position = vec4(
					vec3(aCoords, 1.0) * uTransformMatrix
					+ vec3(aExtrude * uPixelSize, 0.0)
					, 1.0);
			`,
			varyings: {
				vIntensity: "vec2",
				...opts.varyings,
			},
			fragmentShaderMain: `
			gl_FragColor.rg = vIntensity;

			float position = min(vLength.x, vLength.y - vLength.x);
			float opacity = 0.5 + min(position / vWidth, 1.0) / 2.;

			gl_FragColor.rg *= opacity;
			`,
			target: this._inAcetate.framebuffer,
			blend: {
				equationRGB: this.#blendEquation,
				// equationRGB: this.glii.FUNC_ADD,
				equationAlpha: this.#blendEquation,

				srcRGB: this.glii.ONE,
				srcAlpha: this.glii.ZERO,
				dstRGB: this.glii.ONE,
				dstAlpha: this.glii.ZERO,
			},
		};
	}

	resize(w, h) {
		AcetateVertices.prototype.resize.call(this, w, h); // skip parent class' setting uPixelSize

		// const dpr2 = (devicePixelRatio ?? 1) * 2;
		const cellSize = this._inAcetate.cellSize;
		this._programs.setUniform("uPixelSize", [2 / w / cellSize, 2 / h / cellSize]);
	}
}

/**
 * @class SpeedChain
 * @inherits Chain
 * @relationship drawnOn AcetetateSpeedChain
 *
 * A mix of `Chain` and `SlopePoint` - given a (poly)line geometry, this will
 * increase the value of a vector field along the centre of the line, falling
 * off towards the edges of the line.
 *
 * The increase of the vector field is in the direction of each segment of the
 * chain. The intensity of the `SpeedChain` defines the *length* of the vector
 * which is summed to the vector field.
 */
export default class HeatChain extends Chain {
	static Acetate = AcetateSpeedChain;

	#intensity;
	#edgeIntensity;

	/**
	 * @constructor HeatChain(geom: Geometry, opts?: HeatChain Options)
	 */
	constructor(
		geom,
		{
			/**
			 * @section
			 * @aka SpeedChain Options
			 * @option intensity: Number = 10
			 * Length of the vector which is added to the vector field.
			 * The vector will fall off towards zero on the chain's edge, in
			 * a linear fashion. If `edgeIntensity` is set, then the value
			 * falls off to that instead.
			 * @alternative
			 * @option intensity: Array of Number
			 * Intensity of the scalar field on the centerline of each segment
			 * of the chain. There must be enough elements.
			 */
			intensity = 10,

			/**
			 * @section
			 * @aka SpeedChain Options
			 * @option intensity: Number = 0
			 * As `intensity`, but applies to the edge of the chain instead of
			 * its centerline.
			 * @alternative
			 * @option intensity: Array of Number
			 * Intensity of the scalar field on the edge of each segment
			 * of the chain. There must be enough elements.
			 */
			edgeIntensity = 0,
			...opts
		} = {}
	) {
		super(geom, opts);
		this.#intensity = intensity;
		this.#edgeIntensity = edgeIntensity;
	}

	_setPerSegmentStrides(n, vtx, _vtxCount, geom, strideIntensity) {
		const segmentIntensity = Array.isArray(this.#intensity)
			? this.#intensity[n]
			: this.#intensity;
		const segmentEdgeIntensity = Array.isArray(this.#intensity)
			? this.#edgeIntensity[n]
			: this.#edgeIntensity;

		const coordAx = geom.coords[n * geom.dimension];
		const coordAy = geom.coords[n * geom.dimension + 1];
		const coordBx = geom.coords[(n + 1) * geom.dimension];
		const coordBy = geom.coords[(n + 1) * geom.dimension + 1];

		const Δx = coordBx - coordAx;
		const Δy = coordBy - coordAy;
		const ϕ = Math.atan2(Δy, Δx);

		const cosϕ = Math.cos(ϕ);
		const sinϕ = Math.sin(ϕ);

		const centreCosϕ = segmentIntensity * cosϕ;
		const centreSinϕ = segmentIntensity * sinϕ;

		const edgeCosϕ = segmentEdgeIntensity * cosϕ;
		const edgeSinϕ = segmentEdgeIntensity * sinϕ;

		strideIntensity.set([edgeCosϕ, edgeSinϕ], vtx);
		strideIntensity.set([centreCosϕ, centreSinϕ], vtx + 1);
		strideIntensity.set([edgeCosϕ, edgeSinϕ], vtx + 2);
		strideIntensity.set([0, 0], vtx + 3);
		strideIntensity.set([0, 0], vtx + 4);
		strideIntensity.set([edgeCosϕ, edgeSinϕ], vtx + 5);
		strideIntensity.set([centreCosϕ, centreSinϕ], vtx + 6);
		strideIntensity.set([edgeCosϕ, edgeSinϕ], vtx + 7);
		strideIntensity.set([0, 0], vtx + 8);
		strideIntensity.set([0, 0], vtx + 9);
	}
}
