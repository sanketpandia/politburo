import GleoSymbol from "./Symbol.mjs";
import RawGeometry from "../geometry/RawGeometry.mjs";
import parseColour from "../3rd-party/css-colour-parser.mjs";

import AcetateVertices from "../acetates/AcetateVertices.mjs";
import { MESH } from "../util/pointExtrusionTypeConstants.mjs";

/**
 * @class AcetateHair
 * @inherits AcetateVertices
 *
 * An `Acetate` that draws lines as thin (1px) lines.
 *
 * In particular, this uses the `LINES` `drawMode` of WebGL. Line segments drawn
 * this way are 1px-wide and, depending on the OpenGL implementation, are not
 * antialiased and cannot be any thicker (hence "hair" instead of "line").
 */

class AcetateHair extends AcetateVertices {
	/**
	 * @constructor AcetateHair(glii: GliiFactory)
	 */
	constructor(target, opts) {
		super(target, { zIndex: 3000, ...opts });

		this._indices._drawMode = this.glii.LINES;

		this._attrs = new this.glii.InterleavedAttributes(
			{
				size: 1,
				growFactor: 1.2,
				usage: this.glii.STATIC_DRAW,
			},
			[
				{
					// RGBA Colour
					glslType: "vec4",
					type: Uint8Array,
					normalized: true,
				},
			]
		);
	}

	glProgramDefinition() {
		const opts = super.glProgramDefinition();

		return {
			...opts,
			attributes: {
				aColour: this._attrs.getBindableAttribute(0),
				...opts.attributes,
			},
			vertexShaderMain: `
				vColour = aColour;
				gl_Position = vec4(
					vec3(aCoords, 1.0) * uTransformMatrix, 1.0);
			`,
			varyings: { vColour: "vec4" },
			fragmentShaderMain: `gl_FragColor = vColour;`,
		};
	}

	#allocatedVtxs = 0;
	_getStridedArrays(maxVtx, maxIdx) {
		this.#allocatedVtxs = maxVtx;
		return [
			// Indices
			...super._getStridedArrays(maxVtx, maxIdx),

			// Colours
			this._attrs.asStridedArray(0, maxVtx),
		];
	}

	_commitStridedArrays(baseVtx, totalVertices, baseIdx, totalIndices) {
		this._attrs.commit(baseVtx, totalVertices);
		return super._commitStridedArrays(baseVtx, totalVertices, baseIdx, totalIndices);
	}

	_getGeometryStridedArrays() {
		return [];
	}

	_commitGeometryStridedArrays(_baseVtx, _vtxCount, _baseIdx, _idxCount) {
		// noop
	}

	multiAdd(syms) {
		super.multiAdd(syms);
		super.multiAllocate(syms);

		const perPointStrides = this._getPerPointStridedArrays(this.#allocatedVtxs);
		let minVtx = Infinity;
		let maxVtx = -Infinity;
		syms.forEach((sym) => {
			let vtx = sym.attrBase;
			minVtx = Math.min(vtx, minVtx);
			maxVtx = Math.max(vtx + sym.attrLength, maxVtx);
			for (let n = 0; n < sym.attrLength; n++) {
				/// TODO: This should use the *projected* geometry
				sym._setPerPointStrides(n, MESH, vtx + n, 1, ...perPointStrides);
			}
		});
		this._commitPerPointStridedArrays(minVtx, maxVtx - minVtx);

		return this;
	}
}

/**
 * @class Hair
 * @inherits GleoSymbol
 * @relationship drawnOn AcetateHair
 *
 * A 1-pixel line, with a different RGBA colour per hair.
 *
 * `Hair`s are drawn in an `AcetateHair`, which leverages the `LINES` `drawMode` of WebGL.
 * Therefore, they are not antialiased and always one device pixel (not one CSS pixel) wide.
 *
 * For thicker & antialiased lines, see the `Stroke` symbol.
 */

export default class Hair extends GleoSymbol {
	/// @section Static properties
	/// @property Acetate: Prototype of AcetateHair
	// The `Acetate` class that draws this symbol.
	static Acetate = AcetateHair;

	/**
	 * @class Hair
	 * @section
	 * @constructor Hair(geom: Geometry, opts?: Hair Options)
	 *
	 * Create a hair from a `Geometry` and an array of 4 numbers containing a RGBA
	 * colour (with values within 0 and 255).
	 *
	 * The `Geometry` might have any depth. If the depth is 1, a single continuous hair
	 * line is created. If it's deeper, then multiple lines are created.
	 */
	constructor(geom, { colour = [0, 0, 0, 255], ...opts } = {}) {
		super(geom, opts);

		if (!(this.geometry instanceof RawGeometry)) {
			/// TODO: Add an alternative, to accept plain arrays instead of coordinates
			/// This would require setting up a module to specify the default CRS to
			/// be used, like `projector.mjs` does with the Proj instance.
			throw new Error("First argument to Hair constructor is not a (Raw)Geometry.");
		}

		// A Hair symbol needs to destructure its Geometry into sets of 2-vertex
		// line primitives, one continuous linestring per ring/hull in the Geometry.

		// The vertex IDs of the line primitives are consistent even through reprojections
		// (thanks to Geometry wrapping), so the logic can run here instead of in the Acetate.

		let idx = 0;

		const ringsPrimitives = this.geometry.mapRings((_start, _end, length) => {
			const primitives = Array.from(new Array(length - 1)).map((_, i) => {
				const j = idx + i;
				return [j, j + 1];
			});
			idx += length;
			return primitives;
		});

		// This is "relative" since it's always zero-indexed, independent of the value
		// of `this.idxBase`.
		this.relativeIdxs = ringsPrimitives.flat(2);

		// Amount of vertex attribute slots needed
		this.attrLength = this.geometry.coords.length / this.geometry.dimension;

		// Amount of primitive index slots needed
		this.idxLength = this.relativeIdxs.length;

		/**
		 * @section
		 * @aka Hair Options
		 * @option colour: Colour = [0,0,0,255]
		 * The colour of the hair.
		 */
		this.colour = this.constructor._parseColour(colour);
	}

	// Can be overriden by subclasses or the `intensify` decorator
	static _parseColour = parseColour;

	_setGlobalStrides(typedIdxs, strideColour) {
		strideColour.set(
			new Array(this.attrLength).fill(this.colour).flat(),
			this.attrBase
		);

		const idxs = this.relativeIdxs.map((i) => this.attrBase + i);

		typedIdxs.set(idxs, this.idxBase);
	}

	_setGeometryStrides() {
		/* noop */
	}

	/**
	 * @section Acetate Interface
	 * @uninheritable
	 * @method _setPerPointStrides(n: Number, pointType: Symbol, vtx: Number, geom: Geometry, vtxCount: Number ...): this
	 * As `_setGlobalStrides`, but only affects the n-th point in the symbol's
	 * geometry.
	 *
	 * Takes the following parameters:
	 * - Index for the `n`th point in the geometry
	 * - Type of point extrusion (always "mesh" for hairs)
	 * - Index for the vertex attribute data
	 * - Number of vertices spawned for this geometry point (always 1 for hairs)
	 * - strided arrays, as per `_getPerPointStridedArrays`.
	 *
	 * This method can be overriden or extended by subclasses and/or decorators.
	 */
	_setPerPointStrides(_n, _pointType, _vtx, _vtxCount, ..._strides) {
		// Noop
	}
}
