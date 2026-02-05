import GleoSymbol from "./Symbol.mjs";
import parseColour from "../3rd-party/css-colour-parser.mjs";
import Acetate from "../acetates/Acetate.mjs";

/**
 * @class AcetateDot
 * @inherits Acetate
 *
 * An `Acetate` that draws points as coloured dots (given `vec4` RGBA data per dot).
 *
 * In particular, this uses the `POINTS` `drawMode` of WebGL. That means every symbol
 * gets *only* one vertex.
 */

class AcetateDot extends Acetate {
	constructor(target, opts) {
		super(target, { zIndex: 5000, ...opts });

		this._indices = new this.glii.SequentialSparseIndices({
			drawMode: this.glii.POINTS,
		});

		this._attrs = new this.glii.InterleavedAttributes(
			{
				size: 1,
				growFactor: 1.2,
				usage: this.glii.STATIC_DRAW,
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
				gl_Position = vec4(
					vec3(aCoords, 1.0) * uTransformMatrix, 1.0);
				gl_PointSize = aSize;
			`,
			varyings: { vColour: "vec4" },
			fragmentShaderMain: `gl_FragColor = vColour;`,
			indexBuffer: this._indices,
			blend: {
				equationRGB: this.glii.FUNC_ADD,
				equationAlpha: this.glii.FUNC_ADD,

				srcRGB: this.glii.SRC_ALPHA,
				dstRGB: this.glii.ONE_MINUS_SRC_ALPHA,
				srcAlpha: this.glii.ONE,
				dstAlpha: this.glii.ONE_MINUS_SRC_ALPHA,
			},
		};
	}

	_getStridedArrays(_, maxIdx) {
		return [
			// RGBA colour
			this._attrs.asStridedArray(0, maxIdx),

			// Dot size
			this._attrs.asStridedArray(1),
		];
	}

	_commitStridedArrays(baseVtx, vtxCount) {
		this._attrs.commit(baseVtx, vtxCount);
	}

	/**
	 * @method multiAdd(dots: Array of Dot): this
	 * Adds the dots to this acetate (so they're drawn on the next refresh),
	 * using as few WebGL calls as feasible.
	 *
	 * GPU memory space allocated to the given points will be adjacent.
	 */
	multiAdd(dots) {
		this.multiAllocate(dots);
		return super.multiAdd(dots);
	}

	multiAllocate(symbols) {
		// Skip already added symbols
		symbols = symbols.filter((s) => isNaN(s.attrBase));
		if (symbols.length === 0) {
			return;
		}

		const totalVertices = symbols.reduce((acc, ext) => acc + ext.attrLength, 0);
		const base = this._indices.allocateSlots(totalVertices);

		if (this._crs) {
			this.reproject(base, totalVertices, symbols);
		}

		let stridedArrays = this._getStridedArrays(
			base + totalVertices,
			base + totalVertices
		);

		let acc = base;

		symbols.forEach((sym) => {
			sym._inAcetate = this;
			sym.attrBase = sym.idxBase = acc;
			this._knownSymbols[acc] = sym;

			sym._setGlobalStrides(...stridedArrays);

			acc += sym.attrLength;
		});

		this._commitStridedArrays(base, totalVertices, base, totalVertices);

		this.dirty = true;
		return this;
	}

	/**
	 * @method deallocate(dot: Dot): this
	 * Deallocates the dot from this acetate (so it's *not* drawn on the next refresh).
	 */
	deallocate(dot) {
		if (this._knownSymbols[dot.attrBase] !== dot) {
			throw new Error("Trying to remove a Dot symbol from the wrong Acetate.");
		}
		this._indices.deallocateSlots(dot.attrBase, 1);
		dot.updateRefs(undefined, undefined, undefined);

		return this;
	}

	/**
	 * @method reprojectAll(): undefined
	 * Dumps a new set of values to `this._coords`, based on the known
	 * set of symbols added to the acetate.
	 */
	reprojectAll() {
		this._indices.forEachBlock(this.reproject.bind(this));
	}

	/**
	 * Internal. Reprojects a part of the this._coords attribute buffer, from
	 * `start` to `start+length`. Flattens the result and dumps into coords
	 * attribute buffer.
	 */
	reproject(start, length) {
		//console.log(`Should reproject dots allocated at ${start}, length ${length}`);

		const end = start + length;
		const stridedCoords = this._coords.asStridedArray(end);

		this._knownSymbols.forEach((s) => {
			const offset = s.attrBase;
			if (offset < start || offset >= end) {
				return;
			}
			stridedCoords.set(s.geometry.toCRS(this._crs).coords, s.attrBase);
		});

		this._coords.commit(start, length);

		const coordData = new Float32Array(stridedCoords.buffer, start * 8, length * 2);
		super.expandBBox(coordData);
		return coordData;
	}
}

/**
 * @class Dot
 * @inherits GleoSymbol
 * @relationship drawnOn AcetateDot
 *
 * A 1-pixel dot, with RGBA colour.
 *
 * `Dot`s are a minimalist symbol, in the sense that they use the least
 * GPU data structures among symbols.
 */

export default class Dot extends GleoSymbol {
	/// @section Static properties
	/// @property Acetate: Prototype of AcetateDot
	// The `Acetate` class that draws this symbol.
	static Acetate = AcetateDot;
	#colour;

	/**
	 * @constructor Dot(geom: Geometry, opts?: Dot Options)
	 */
	constructor(geom, { colour = [0, 0, 0, 255], size = 1, ...opts } = {}) {
		super(geom, opts);

		if (this.geom.coords.length !== this.geom.dimension) {
			/// TODO: Add an alternative, to accept plain arrays instead of coordinates
			/// This would require setting up a module to specify the default CRS to
			/// be used, like `projector.mjs` does with the Proj instance.
			throw new Error("Geometry passed to Dot constructor is not a single point.");
		}

		/**
		 * @section
		 * @aka Dot Options
		 * @option colour: Colour = [0,0,0,255]
		 * The colour of the dot.
		 */
		this.#colour = this.constructor._parseColour(colour);

		/**
		 * @option size: Number = 1
		 * The size of the dot, in GL pixels. Values larger than 1 will draw a
		 * square with this many pixels per side.
		 *
		 * The maximum value depends on the GPU and WebGL/OpenGL stack. It is possible
		 * for the maximum value to be 1.
		 */
		this.size = size;

		// Dots are *always* one vertex and one primitive index (and they're the same)
		this.attrLength = 1;
		this.idxLength = 1;
	}

	/**
	 * @section Acetate interface methods
	 * @uninheritable
	 * For internal use only
	 * @method _setGlobalStrides(stridedColour: StridedTypedArray, stridedDotSize: StridedTypedArray): this
	 */
	_setGlobalStrides(colour, dotSize) {
		colour.set(this.#colour, this.attrBase);
		dotSize.set([this.size], this.attrBase);
		return this;
	}

	// Can be overriden by subclasses or the `intensify` decorator
	static _parseColour = parseColour;
}
