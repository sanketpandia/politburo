import ExtrudedPoint from "./ExtrudedPoint.mjs";
import parseColour from "../3rd-party/css-colour-parser.mjs";
import AcetateSolidBorder from "../acetates/AcetateSolidBorder.mjs";

/**
 * @class HeadingTriangle
 * @inherits ExtrudedPoint
 * @relationship drawnOn AcetateHeadingTriangle
 *
 * A small triangle, meant to signify heading (or direction, or course) of
 * a feature; should be used in conjunction with other symbol to represent
 * the feature itself.
 *
 * Works with point geometries only.
 */

export default class HeadingTriangle extends ExtrudedPoint {
	static Acetate = AcetateSolidBorder;

	#distance;
	#width;
	#length;
	#fillColour;
	#borderColour;
	#borderWidth;
	#feather;
	#yaw;

	constructor(
		geom,
		{
			/**
			 * @section
			 * @aka HeadingTriangle Options
			 * @option distance: Number = 16
			 * The distance from the geometry to the base of the triangle, in CSS pixels.
			 *
			 * @option width: Number = 8
			 * The width of the triangle, in CSS pixels.
			 *
			 * @option length: Number = 6
			 * The length of the triangle, in CSS pixels.
			 *
			 * @option fillColour: Colour = 'white'
			 * The colour for the inside area of the triangle.
			 *
			 * @option borderColour: Colour = 'black'
			 * The colour for the border of the triangle.
			 *
			 * @option borderWidth: Number = 1
			 * The width of the border, in CSS pixels.
			 *
			 * @option feather: Number = 0.5
			 * The width of the antialiasing feather, in CSS pixels.
			 */
			distance = 16,
			width = 8,
			length = 6,
			fillColour = [255, 255, 255, 255],
			borderColour = [0, 0, 0, 255],
			borderWidth = 1,
			feather = 0.5,

			/**
			 * @option yaw: Number = 0
			 * The yaw rotation of the triangle, in clockwise degrees from "north"
			 */
			yaw = 0,

			...opts
		}
	) {
		super(geom, opts);

		this.#distance = distance - feather / 2;
		this.#width = width + feather;
		this.#length = length + feather / 2;
		this.#fillColour = this.constructor._parseColour(fillColour);
		this.#borderColour = this.constructor._parseColour(borderColour);
		this.#borderWidth = borderWidth;
		this.#feather = feather;

		this.#yaw = yaw;

		this.attrLength = 3;
		this.idxLength = 3;
	}

	/**
	 * @section
	 * @property yaw: Number
	 * Runtime value of the `yaw` option: the yaw rotation of the sprite,
	 * in clockwise degrees. Can be updated.
	 */
	set yaw(yaw) {
		this.#yaw = yaw;
		this._refreshExtrusion();
	}
	get yaw() {
		return this.#yaw;
	}

	/**
	 * @property fillColour: Colour
	 * The fill colour of the triangle. Can be updated.
	 */
	get fillColour() {
		return this.#fillColour;
	}

	set fillColour(c) {
		this.#fillColour = this.constructor._parseColour(c);
		if (!this._inAcetate) {
			return;
		}

		const stridedArrays = this._inAcetate._getStridedArrays(
			this.attrBase + this.attrLength,
			this.idxBase + this.idxLength
		);
		this._setGlobalStrides(...stridedArrays);
		this._inAcetate._commitStridedArrays(
			this.attrBase,
			this.attrLength,
			this.idxBase,
			this.idxLength
		);
		this._inAcetate.dirty = true;
	}

	/**
	 * @section Acetate interface
	 * @method _setGlobalStrides(strideExtrusion: StridedTypedArray, strideFillColour: StridedTypedArray, strideBorderColour: StridedTypedArray, strideBorder: StridedTypedArray, strideEdgeDistance: StridedTypedArray, typedIdxs: TypedArray, feather: Number): undefined
	 * Sets the appropriate values into the strided arrays, based on the
	 * symbol's `attrBase` and `idxBase`.
	 *
	 * Receives the width of the feathering as a parameter, in pixels.
	 */
	_setGlobalStrides(
		strideExtrusion,
		strideFillColour,
		strideBorderColour,
		strideBorder,
		strideEdgeDistance,
		typedIdxs
	) {
		const yawRadians = (-this.#yaw * Math.PI) / 180;
		const s = Math.sin(yawRadians);
		const c = Math.cos(yawRadians);
		const l = this.#length;
		const w = this.#width / 2;
		const d = this.#distance;

		let [Δx, Δy] = this.offset;
		Δx -= s * d;
		Δy += c * d;

		// prettier-ignore
		strideExtrusion.set([
			Δx - s*l, Δy + c*l,
			Δx - c*w, Δy - s*w,
			Δx + c*w, Δy + s*w
		], this.attrBase);

		if (!strideFillColour) {
			return;
		}

		for (let i = 0; i < 3; i++) {
			strideFillColour.set(this.#fillColour, this.attrBase + i);
			strideBorderColour.set(this.#borderColour, this.attrBase + i);
			strideBorder.set([this.#borderWidth, this.#feather], this.attrBase + i);
		}

		strideEdgeDistance.set([this.#length, 0, 0], this.attrBase);

		// Relation between length & width; half the angle of the triangle tip;
		// same as relative angle from base vertex to create an orthogonal
		// to a side
		const α = Math.atan2(this.#length, this.#width);

		const dist = Math.cos(α) * this.#width;

		strideEdgeDistance.set([0, dist, 0], this.attrBase + 1);
		strideEdgeDistance.set([0, 0, dist], this.attrBase + 2);

		typedIdxs.set(
			[this.attrBase, this.attrBase + 1, this.attrBase + 2],
			this.idxBase
		);
	}

	_refreshExtrusion() {
		if (!this._inAcetate) {
			return this;
		}

		let strideExtrude = this._inAcetate._extrusions.asStridedArray();
		this._setGlobalStrides(strideExtrude);

		this._inAcetate._extrusions.commit(this.attrBase, this.attrLength);
		this._inAcetate.dirty = true;
		return this;
	}

	_setStridedExtrusion(strideExtrusion) {
		this._setGlobalStrides(strideExtrusion);
	}

	// Can be overriden by subclasses or the `intensify` decorator
	static _parseColour = parseColour;
}
