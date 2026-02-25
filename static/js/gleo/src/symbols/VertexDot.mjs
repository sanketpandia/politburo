import AcetateVertices from "../acetates/AcetateVertices.mjs";
// import Dot from "./Dot.mjs";
import GleoSymbol from "./Symbol.mjs";
import parseColour from "../3rd-party/css-colour-parser.mjs";

class AcetateVertexDot extends AcetateVertices {
	constructor(target, opts) {
		super(target, opts);
		const glii = this.glii;

		this._indices = new glii.SparseIndices({
			type: glii.UNSIGNED_INT,
			drawMode: glii.POINTS,
		});
		this._attrs = new glii.InterleavedAttributes(
			{
				size: 1,
				growFactor: 1.2,
				usage: glii.STATIC_DRAW,
			},
			[
				{
					// Colour
					glslType: "vec4",
					type: Uint8Array,
					normalized: true,
				},
				{
					// Point size
					glslType: "float",
					type: Uint16Array,
					normalized: false,
				},
			]
		);
	}

	glProgramDefinition() {
		const opts = super.glProgramDefinition();
		return {
			...opts,
			attributes: {
				aColour: this._attrs?.getBindableAttribute(0),
				aSize: this._attrs?.getBindableAttribute(1),
				...opts.attributes,
			},
			uniforms: {
				...opts.uniforms,
			},
			vertexShaderMain: `
				vColour = aColour;
				gl_Position = vec4(vec3(aCoords, 1.0) * uTransformMatrix, 1.0);
				gl_PointSize = aSize;
			`,
			varyings: { vColour: "vec4" },
			fragmentShaderSource: `void main() { gl_FragColor = vColour; }`,
		};
	}
	_getStridedArrays(maxVtx) {
		return [
			// Colour
			this._attrs.asStridedArray(0, maxVtx),
			// Dot size
			this._attrs.asStridedArray(1),
			// Index buffer
			this._indices.asTypedArray(maxVtx),
		];
	}

	_getGeometryStridedArrays(maxVtx) {
		return [
			// CRS coords
			this._coords.asStridedArray(maxVtx),
		];
	}

	_commitStridedArrays(baseVtx, vtxCount) {
		this._attrs.commit(baseVtx, vtxCount);
		this._indices.commit(baseVtx, vtxCount);
	}

	_commitGeometryStridedArrays(baseVtx, vtxLength /*, baseIdx, totalIndices*/) {
		this._coords.commit(baseVtx, vtxLength);
	}

	multiAdd(dots) {
		// Skip already added symbols
		dots = dots.filter((d) => !d._inAcetate);
		if (dots.length === 0) {
			return;
		}

		/// number of attributes needed can be different than one per dot
		/// (e.g. trajectorified dots), so this needs to sum the attrLengths
		/// of all dots, not just use the dot count.
		const totalAttrs = dots.reduce((acc, dot) => acc + dot.attrLength, 0);

		let base = this._attribAllocator.allocateBlock(totalAttrs);
		this._indices.allocateSlots(totalAttrs);
		const maxVtx = base + totalAttrs;

		let stridedArrays = this._getStridedArrays(maxVtx);

		let i = 0;
		dots.forEach((dot) => {
			dot.updateRefs(this, base + i, base + i);
			this._knownSymbols[base + i] = dot;
			dot._setGlobalStrides(...stridedArrays);
			i += dot.attrLength;
		});
		this._commitStridedArrays(base, totalAttrs);
		this._indices.commit(base, totalAttrs);

		if (this._crs) {
			this.reproject(base, totalAttrs);
		}

		return super.multiAdd(dots);
	}
}

/**
 * @class VertexDot
 * @inherits GleoSymbol
 * @relationship drawnOn AcetateVertexDot
 *
 * An alternative implementation of the `Dot` symbol.
 *
 * This is done exclusively for compatibility with the `trajectorify` decorator.
 *
 * The technical difference is that `Dot` assumes always one vertex per `Dot`,
 * whereas `VertexDot` *behaves* as multiple-vertices-per-symbol. That allows
 * `VertexDot` to be `trajectorify`d.
 */

export default class VertexDot extends GleoSymbol {
	/// @section Static properties
	/// @property Acetate: Prototype of AcetateVertexDot
	// The `Acetate` class that draws this symbol.
	static Acetate = AcetateVertexDot;
	#colour;

	/**
	 * @constructor VertexDot(geom: Geometry, opts?: VertexDot Options)
	 */
	constructor(geom, { colour = [0, 0, 0, 255], size = 1, ...opts } = {}) {
		super(geom, opts);

		/**
		 * @section
		 * @aka VertexDot Options
		 * @option colour: Colour = [0,0,0,255]
		 * The colour of the dot.
		 */
		this.#colour = this.constructor._parseColour(colour);

		/**
		 * @option size: Number = 1
		 * The size of the dot, in GL pixels. Values larger than 1 will draw a
		 * square with this many pixels per side.
		 *
		 * The maximum value depends on the GPU and WebGL/OpenGL stack.
		 */
		this.size = size;

		this.attrLength = 1;
		this.idxLength = 1;
	}

	_setGlobalStrides(colour, dotSize, indices) {
		colour.set(this.#colour, this.attrBase);
		dotSize.set([this.size], this.attrBase);
		indices?.set([this.attrBase], this.attrBase);
		return this;
	}

	_setGeometryStrides(geom, strideCoords) {
		strideCoords.set(geom.coords.flat(), this.attrBase);
	}

	// Can be overriden by subclasses or the `intensify` decorator
	static _parseColour = parseColour;
}
