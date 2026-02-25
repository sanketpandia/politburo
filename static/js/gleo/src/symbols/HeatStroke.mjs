import Stroke, { BEVEL } from "./Stroke.mjs";

import { LINEJOIN, LINELOOP, LINECAP } from "../util/pointExtrusionTypeConstants.mjs";

import { ScalarField } from "../fields/Field.mjs";

/**
 * @class AcetateHeatStroke
 * @inherits AcetateStroke
 * @relationship drawnOn ScalarField
 *
 * Draws `HeatStroke`s onto a scalar field.
 */

class AcetateHeatStroke extends Stroke.Acetate {
	constructor(glii, opts) {
		super(glii, opts);

		// Non-geometric attributes - the ones that don't change with a full
		// reprojection
		this._attrs = new this.glii.InterleavedAttributes(
			{
				size: 1,
				growFactor: 1.2,
				usage: this.glii.STATIC_DRAW,
			},
			[
				{
					// Field intensity
					glslType: "float",
					type: Float32Array,
				},
				// {
				// 	// RGBA Colour
				// 	glslType: "vec4",
				// 	type: Uint8Array,
				// 	normalized: true,
				// },
				{
					// (Accumulated) dash array, with up to 4 elements.
					glslType: "vec4",
					type: Uint8Array,
					normalized: false,
				},
				// TODO: antialias feather (or make it an Acetate uniform)
			]
		);
	}

	/**
	 * @property PostAcetate: AcetateScalarField
	 * Signals that this `Acetate` isn't rendered as a RGBA8 texture,
	 * but instead uses a scalar field.
	 */
	static get PostAcetate() {
		return ScalarField;
	}

	glProgramDefinition() {
		const opts = super.glProgramDefinition();

		delete opts.attributes.aColour;
		delete opts.varyings.vColour;

		return {
			...opts,
			attributes: {
				...opts.attributes,
				aIntensity: this._attrs.getBindableAttribute(0),
			},
			varyings: {
				...opts.varyings,
				vIntensity: "float",
			},
			vertexShaderSource: opts.vertexShaderSource.replace(/Colour/g, "Intensity"),
			vertexShaderMain: opts.vertexShaderMain.replace(/Colour/g, "Intensity"),
			fragmentShaderMain: `
				/// FIXME: This is an attempt at reversing some artefacts on
				/// short segments with acute angles
				// if (!gl_FrontFacing) {
				// 	// gl_FragColor.r = - gl_FragColor.r;
				// 	discard;
				// }

				float dashIdx = mod(vAccLength, vDashArray.w);
				if (dashIdx <= vDashArray.x) {
					gl_FragColor.r = vIntensity;
				} else if (dashIdx <= vDashArray.y) {
					discard;
				} else if (dashIdx <= vDashArray.z) {
					gl_FragColor.r = vIntensity;
				} else {
					discard;
				}
			`,

			target: this._inAcetate.framebuffer,

			blend: {
				// See notes about blend mode in AcetateHeatStroke
				equationRGB: this.glii.FUNC_ADD,
				equationAlpha: this.glii.FUNC_ADD,
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
	}

	// Same as Stroke
	// _getPerPointStridedArrays(maxVtx, maxIdx) {
	// 	return [
	// 		...super._getPerPointStridedArrays(maxVtx, maxIdx),
	//
	// 		// Field intensity
	// 		this._attrs.asStridedArray(0, maxVtx),
	// 	];
	// }

	// Same as Stroke
	// _getStridedArrays(maxVtx, _maxIdx) { }

	_commitPerPointStridedArrays(baseVtx, vtxLength) {
		super._commitPerPointStridedArrays(baseVtx, vtxLength);
		this._attrs.commit(baseVtx, vtxLength);
	}
}

/**
 * @class HeatStroke
 * @inherits Stroke
 * @relationship drawnOn AcetateHeatStroke
 *
 * A mix of `Stroke` and `HeatPoint` - given a (poly)line geometry, this will
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
export default class HeatStroke extends Stroke {
	/// @section Static properties
	/// @property Acetate: Prototype of AcetateHeatStroke
	// The `Acetate` class that draws this symbol.
	static Acetate = AcetateHeatStroke;

	#intensity;

	constructor(
		geom,
		{
			/**
			 * @option intensity: Number = 1
			 * Intensity of the scalar field on the stroke's centerline.
			 * The intensity will fall off towards zero on the stroke's edge, in
			 * a linear fashion.
			 */
			intensity = 1,
			...opts
		} = {}
	) {
		super(geom, {
			joins: Stroke.OUTBEVEL,
			caps: Stroke.SQUARE,
			...opts,
			colour: undefined,
			centerline: true,
		});

		this.#intensity = intensity;
	}

	_setPerPointStrides(n, pointType, vtx, vtxCount, strideIntensity) {
		const first =
			n === 0 || this.geometry.rings.includes(n) || this.geometry.hulls.includes(n);
		const isBevel = this.joins === Stroke.BEVEL || this.joins === Stroke.OUTBEVEL;

		let centerVtx =
			pointType === LINELOOP || (pointType === LINEJOIN && isBevel && first)
				? 0
				: 1;

		// centerVtx = (pointType === LINELOOP) ? 0 : 1;

		for (let i = 0; i < vtxCount; i++) {
			if (i === centerVtx) {
				// centerpoint
				strideIntensity.set([this.#intensity], vtx + i);
			} else {
				// non-centerpoint
				strideIntensity.set([0], vtx + i);
			}
		}
	}

	// // As parent, but skips colour
	// _setGlobalStrides(strideDash) {
	// 	// Normalize dasharray into an accumulated 4-element array.
	// 	let dashArray;
	// 	if (!this.dashArray || this.dashArray.length === 0) {
	// 		dashArray = Uint8Array.from([1, 1, 1, 1]);
	// 	} else if (this.dashArray.length === 2) {
	// 		const [d0, d1] = this.dashArray;
	// 		dashArray = Uint8Array.from([d0, d1 + d0, d0 + d1 + d0, d1 + d0 + d1 + d0]);
	// 	} else if (this.dashArray.length === 4) {
	// 		const [d0, d1, d2, d3] = this.dashArray;
	// 		dashArray = Uint8Array.from([d0, d1 + d0, d2 + d1 + d0, d3 + d2 + d1 + d0]);
	// 	} else {
	// 		throw new Error("Invalid length of dashArray in stroke.");
	// 	}
	//
	// 	for (let i = this.attrBase, end = this.attrBase + this.attrLength; i < end; i++) {
	// 		strideDash.set(dashArray, i);
	// 	}
	// }
}
