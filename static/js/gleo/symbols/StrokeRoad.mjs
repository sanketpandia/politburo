import Stroke from "./Stroke.mjs";

import { LINEJOIN, LINELOOP, LINECAP } from "../util/pointExtrusionTypeConstants.mjs";

const SQRT3 = Math.sqrt(3);

/**
 * @class AcetateStrokeRoad
 * @inherits AcetateStroke
 *
 * An `Acetate` that draws stroke roads (strokes with two widths and an inner
 * and outer colour).
 */
class AcetateStrokeRoad extends Stroke.Acetate {
	constructor(target, opts = {}) {
		super(target, opts);

		this._attrs = new this.glii.InterleavedAttributes(
			{
				size: 1,
				growFactor: 1.2,
				usage: this.glii.STATIC_DRAW,
			},
			[
				{
					// RGBA Colour, inside
					glslType: "vec4",
					type: Uint8Array,
					normalized: true,
				},
				{
					// (Accumulated) dash array, with up to 4 elements.
					glslType: "vec4",
					type: Uint8Array,
					normalized: false,
				},
				{
					// RGBA Colour, outside casing
					glslType: "vec4",
					type: Uint8Array,
					normalized: true,
				},
				{
					// Z-index for this stroke (within the acetate).
					// Values should be -32000 to +32000, attribute will get
					// normalized (so it can be fed directly into gl_Position.z)
					glslType: "float",
					type: Int16Array,
					normalized: true,
				},
				{
					// Relative distance to centerline and casing threshold.
					// First element is relative distance to centerline:
					// value is 0 at centerline, 1 at edge (255 unnormalized at
					// edge).
					// Second element is casing threshold: percentage of the
					// half width when the outside casing starts. When the
					// relative distance to centerline is greater than this
					// value, outside casing colour has to be applied. Note
					// attribute normalization: percentage must be relative to 255.
					glslType: "vec2",
					type: Uint8Array,
					normalized: true,
				},
				// TODO: antialias feather (or make it an Acetate uniform)
			]
		);
	}

	glProgramDefinition() {
		const opts = super.glProgramDefinition();

		// Note that the value of the depth buffer (gl_Position.z) is negative,
		// since the depth clear value is 1 and the operation is LESS.
		// The same result could be achieved with a positive value of the zIndex,
		// a GREATER operation and a clear value of -1; but this would require
		// re-setting the `this._clear` operation to reset the clear value.

		return {
			...opts,
			depth: this.glii.LESS,
			attributes: {
				...opts.attributes,
				aOutColour: this._attrs.getBindableAttribute(2),
				aZIndex: this._attrs.getBindableAttribute(3),
				aCaseThreshold: this._attrs.getBindableAttribute(4),
			},
			vertexShaderMain: `
				vColour = aColour;
				vOutColour = aOutColour;
				vDashArray = aDashArray;
				vAccLength = aAccLength / uScale;
				vCaseThreshold = aCaseThreshold;

				vec2 extrude = aExtrude;
				if (aInnerAdjustment.x != 0.) {
					float factor = clamp(
						length(aExtrude) * uScale / aInnerAdjustment.x,
						1.,
						aInnerAdjustment.y
					);
					extrude /= factor;
				}

				gl_Position = vec4(
					(vec3(aCoords, 1.0) * uTransformMatrix
					+ vec3(extrude * uPixelSize, 0.0)).xy
					, -aZIndex * 256.
					, 1.0);
			`,
			varyings: {
				...opts.varyings,
				vCaseThreshold: "vec2",
				vOutColour: "vec4",
			},
			fragmentShaderMain: `
				float dashIdx = mod(vAccLength, vDashArray.w);

				/// TODO: Apply some feathering between the two colours
				vec4 colour = vCaseThreshold.x < vCaseThreshold.y ?
					vColour : vOutColour;

				if (dashIdx <= vDashArray.x) {
					gl_FragColor = colour;
				} else if (dashIdx <= vDashArray.y) {
					discard;
				} else if (dashIdx <= vDashArray.z) {
					gl_FragColor = colour;
				} else {
					discard;
				}

				// gl_FragColor.rgb = (gl_FragCoord.zzz);

				if (!gl_FrontFacing) {gl_FragColor = vec4(1., 0., 0., .5);}
				// if (!gl_FrontFacing) { discard; }
			`,
		};
	}

	_getPerPointStridedArrays(maxVtx, maxIdx) {
		return [
			// Z-index (higher at centerpoints)
			this._attrs.asStridedArray(3, maxVtx),

			// Casing threshold
			this._attrs.asStridedArray(4),

			...super._getPerPointStridedArrays(maxVtx, maxIdx),
		];
	}

	_getStridedArrays(maxVtx, maxIdx) {
		return [
			// Casing colour
			this._attrs.asStridedArray(2, maxVtx),

			...super._getStridedArrays(maxVtx, maxIdx),
		];
	}

	_commitPerPointStridedArrays(baseVtx, vtxLength) {
		super._commitPerPointStridedArrays(baseVtx, vtxLength);
		this._attrs.commit(baseVtx, vtxLength);
	}
}

/**
 * @class StrokeRoad
 * @inherits Stroke
 * @relationship dependsOn AcetateStrokeRoad
 *
 * A symbol for drawing roads - works as two `Stroke`s in one, with two
 * different widths, two different colours, and an explicit Z-index for
 * drawing tunnels/bridges under/over other roads.
 */
export default class StrokeRoad extends Stroke {
	/// @section Static properties
	/// @property Acetate: Prototype of AcetateStrokeRoad
	// The `Acetate` class that draws this symbol.
	static Acetate = AcetateStrokeRoad;

	#outColour;
	#outWidth;

	#zIndex;
	#capZIndex;

	/**
	 * @constructor StrokeRoad(geom: Geometry, opts?: StrokeRoad Options)
	 */
	constructor(
		geom,
		{
			/**
			 * @aka StrokeRoad Options
			 * @option outColour: Colour = 'black'
			 * The colour of the outer casing of the road stroke
			 */
			outColour = [0, 0, 0, 255],
			/**
			 * @option width: Number = 2
			 * The entire width of the stroke, in CSS pixels
			 * @option outWidth: Number = 1
			 * The width of the outer casing of the road stroke, in CSS pixels.
			 *
			 * Note that the inner width of the stroke is `width` minus `outWidth`.
			 */
			outWidth = 1,
			/**
			 * @option zIndex: Number = 0
			 * The z-index of this symbol (relative to others in this acetate).
			 * It will be encoded in a `Int16Array`, so the value must be an
			 * integer between -32000 and +32000
			 */
			zIndex = 0,

			/**
			 * @option capZIndex: Number
			 * As `zIndex`, but for line caps. When not specified, the line caps
			 * will use the same z-index as the main body of the stroke.
			 *
			 * Has no effect when using `BUTT` line caps.
			 */
			capZIndex,

			...opts
		} = {}
	) {
		/// TODO: consider whether to tweak the widths at this stage, e.g.
		/// sum `width` plus `outWidth` and pass it as simply `width`. This
		/// would make `width` the inner width, i.e. the area which has
		/// `colour` - perhaps this would make the semantics of
		/// colour/outColour/width/outWidth a bit more consistent.
		super(geom, { ...opts, centerline: true });

		this.#outColour = this.constructor._parseColour(outColour);
		this.#outWidth = outWidth;
		this.#zIndex = zIndex;
		this.#capZIndex = capZIndex ?? zIndex;
	}

	_setGlobalStrides(strideOutColour, ...strides) {
		super._setGlobalStrides(...strides);

		for (let i = this.attrBase, end = this.attrBase + this.attrLength; i < end; i++) {
			strideOutColour.set(this.#outColour, i);
		}
	}

	_setPerPointStrides(
		n,
		pointType,
		vtx,
		vtxCount,
		strideZIndex,
		strideCaseThreshold,
		...strides
	) {
		super._setPerPointStrides(n, pointType, vtx, vtxCount, ...strides);

		const caseThreshold = (255 * this.width) / (this.width + this.#outWidth);

		if (pointType === LINECAP) {
			// The three vertices that still form part of the main body use
			// the normal z-index
			strideZIndex.set([this.#zIndex], vtx + 0);
			strideZIndex.set([this.#zIndex + 1], vtx + 1);
			strideZIndex.set([this.#zIndex], vtx + 2);

			strideCaseThreshold.set([255, caseThreshold], vtx + 0);
			strideCaseThreshold.set([0, caseThreshold], vtx + 1);
			strideCaseThreshold.set([255, caseThreshold], vtx + 2);

			// Any vertices that only belong to the line cap use the cap z-index
			for (let i = 3; i < vtxCount; i++) {
				strideZIndex.set([this.#capZIndex], vtx + i);
				strideCaseThreshold.set([255, caseThreshold], vtx + i);
			}

			if (this.caps === this.constructor.HEX) {
				strideCaseThreshold.set([1, caseThreshold], vtx + 4);
				strideZIndex.set([this.#capZIndex + 1], vtx + 4);
			}
		} else {
			// join, loop

			for (let i = 0; i < vtxCount; i++) {
				if (
					(i === 0 && pointType === LINELOOP) ||
					(i === 1 && pointType !== LINELOOP)
				) {
					// centerpoint
					strideZIndex.set([this.#zIndex + 1], vtx + i);
					strideCaseThreshold.set([0, caseThreshold], vtx + i);
				} else {
					// non-centerpoint
					strideZIndex.set([this.#zIndex], vtx + i);
					strideCaseThreshold.set([255, caseThreshold], vtx + i);
				}
			}
		}
	}

	// As parent, but adds extra vertices as to ramp down the z-index more
	// aggresively
	_fillLineEndHex(heading, data, geom, i, first) {
		// Fills *four* vertices with a half-hexagon cap.

		const hexHeight = data.width * 0.5 * SQRT3;

		const extrude = heading.perp()._mult(data.width);
		const halfExtrude = extrude.mult(0.5);
		const widthHeading = heading.mult(first ? -hexHeight : hexHeight);
		const leftExtrude = widthHeading.add(halfExtrude);
		const rightExtrude = widthHeading._sub(halfExtrude);

		// The rest of the method is identical to _fillLineEndSquare

		this._setPerPointStrides(i, LINECAP, data.vtx, 8, geom, ...data.perPointStrides);

		// prettier-ignore
		data.strideExtrude.set( [
			extrude.x, extrude.y, data.accDistance, 0, 0,
			0, 0, data.accDistance, 0,0,
			-extrude.x, -extrude.y, data.accDistance, 0, 0,

			extrude.x, extrude.y, data.accDistance, 0, 0,
			0, 0, data.accDistance, 0,0,
			-extrude.x, -extrude.y, data.accDistance, 0, 0,

			leftExtrude.x, leftExtrude.y, data.accDistance, 0, 0,
			rightExtrude.x, rightExtrude.y, data.accDistance, 0, 0,
		], data.vtx);

		if (first) {
			// prettier-ignore
			data.typedIdxs.set([
				data.vtx + 3, data.vtx + 6, data.vtx + 4,
				data.vtx + 4, data.vtx + 6, data.vtx + 7,
				data.vtx + 4, data.vtx + 7, data.vtx + 5,
			], data.idx);
		} else {
			// prettier-ignore
			data.typedIdxs.set([
				data.vtx + 3, data.vtx + 4, data.vtx + 6,
				data.vtx + 4, data.vtx + 7, data.vtx + 6,
				data.vtx + 4, data.vtx + 5, data.vtx + 7,
			], data.idx);
		}
		data.idx += 9;

		data.lastLeftVtx = data.vtx + 0;
		data.lastCenterVtx = data.vtx + 1;
		data.lastRightVtx = data.vtx + 2;
		data.vtx += 8;
	}

	get verticesPerEnd() {
		return this.caps === Stroke.HEX ? 7 : super.verticesPerEnd;
	}
}
