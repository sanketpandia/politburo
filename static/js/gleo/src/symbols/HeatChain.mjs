import Chain from "./Chain.mjs";

import { ScalarField } from "../fields/Field.mjs";
import getBlendEquationConstant from "../util/getBlendEquationConstant.mjs";

/**
 * @class AcetateHeatChain
 * @inherits AcetateChain
 * @relationship drawnOn ScalarField
 *
 * An `Acetate` to place `HeatChain`s into a scalar field.
 *
 */
class AcetateHeatChain extends Chain.Acetate {
	#blendEquation;

	/**
	 * @constructor AcetateHeatChain(target: GliiFactory, opts: AcetateHeatChain Options)
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

			// Heat acetates are not gonna be interactive - otherwise
			// confusion will ensue when several heatpoints overlap each other.
			// Arguably interactivity could be achieved by a more complex shader,
			// leveraging the depth buffer - only the heatpoint closest to
			// the camera would be registered.
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
					// Heat intensity
					glslType: "float",
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
				vIntensity: "float",
				...opts.varyings,
			},
			fragmentShaderMain: `
			gl_FragColor.r = vIntensity;

			float position = min(vLength.x, vLength.y - vLength.x);
			float opacity = 0.5 + min(position / vWidth, 1.0) / 2.;

			gl_FragColor.r *= opacity;
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
}

/**
 * @class HeatChain
 * @inherits Chain
 * @relationship drawnOn AcetetateHeatChain
 *
 * A mix of `Chain` and `HeatPoint` - given a (poly)line geometry, this will
 * increase the value of a scalar field along the centre of the line, falling
 * off towards the edges of the line.
 *
 * Use a `HeatMap` or any other subclass of `ScalarField`, same as
 * `HeatPoint`.
 *
 * See also `HeatChain`. For high fidelity on visible corners, use `HeatStroke`.
 * To minimize rendering artefacts when zooming out on geometries with sharp
 * turns, use `HeatChain`.
 */
export default class HeatChain extends Chain {
	static Acetate = AcetateHeatChain;

	#intensity;
	/**
	 * @constructor HeatChain(geom: Geometry, opts?: HeatChain Options)
	 */
	constructor(
		geom,
		{
			/**
			 * @section
			 * @aka HeatChain Options
			 * @option intensity: Number = 10
			 * Intensity of the scalar field on the chain's centerline.
			 * The intensity will fall off towards zero on the chain's edge, in
			 * a linear fashion.
			 * @alternative
			 * @option intensity: Array of Number
			 * Intensity of the scalar field on the centerline of each segment
			 * of the chain. There must be enough elements.
			 */
			intensity = 10,
			...opts
		} = {}
	) {
		super(geom, opts);
		this.#intensity = intensity;
	}

	_setPerSegmentStrides(n, vtx, vtxCount, _geom, strideIntensity) {
		const segmentIntensity = Array.isArray(this.#intensity)
			? this.#intensity[n]
			: this.#intensity;

		for (let i = 0; i < vtxCount; i++) {
			strideIntensity.set([i === 1 || i === 6 ? segmentIntensity : 0], vtx + i);
		}
	}
}
