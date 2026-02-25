import GleoSymbol from "./Symbol.mjs";
import parseColour from "../3rd-party/css-colour-parser.mjs";

import AcetateInteractive from "../acetates/AcetateInteractive.mjs";

// import Allocator from "../glii/src/Allocator.mjs";
import earcut from "../3rd-party/earcut/earcut.mjs";

/**
 * @class AcetateFill
 * @inherits AcetateVertices
 *
 * An `Acetate` that draws a simple (single-colour) fill for polygons.
 *
 * This leverages Vladimir Agafonkin's `earcut` library. Polygon triangulation
 * happens after (every) data (re)projection.
 *
 */

class AcetateFill extends AcetateInteractive {
	/**
	 * @constructor AcetateFill(target: GliiFactory)
	 */
	constructor(target, opts) {
		super(target, { zIndex: 1000, ...opts });

		// Could be done as a SingleAttribute, but is a InterleavedAttributes for
		// compatibility with the `intensify` decorator.

		this._attrs = new this.glii.InterleavedAttributes(
			{
				size: 1,
				growFactor: 1.2,
				usage: this.glii.STATIC_DRAW,
			},
			[
				{
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
				...opts.attributes,
				aColour: this._attrs.getBindableAttribute(0),
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

	_getStridedArrays(maxVtx, _maxIdx) {
		return [
			// Indices
			//...super._getStridedArrays(maxVtx, maxIdx),

			// Colours
			this._attrs.asStridedArray(0, maxVtx),
		];
	}

	_commitStridedArrays(baseVtx, totalVertices, _baseIdx, _totalIndices) {
		this._attrs.commit(baseVtx, totalVertices);
		// return super._commitStridedArrays(baseVtx, totalVertices, baseIdx, totalIndices);
		return this;
	}

	_getGeometryStridedArrays(_maxVtx, _maxIdx) {
		return [];
	}

	_commitGeometryStridedArrays(_baseVtx, _vtxCount, _baseIdx, _idxCount) {
		// noop
	}

	multiAdd(syms) {
		super.multiAdd(syms);
		return this.multiAllocate(syms);
	}

	multiAllocate(fills) {
		// Skip already added symbols
		fills = fills.filter((f) => isNaN(f.attrBase));
		if (fills.length === 0) {
			return;
		}

		const totalVertices = fills.reduce((acc, fill) => acc + fill.attrLength, 0);
		let baseVtx = this._attribAllocator.allocateBlock(totalVertices);
		let vtxAcc = baseVtx;

		let stridedArrays = this._getStridedArrays(
			baseVtx + totalVertices
			// baseIdx + totalIndices
		);

		fills.forEach((fill) => {
			fill._inAcetate = this;
			fill.attrBase = vtxAcc;
			// fill.idxBase = idxAcc;
			this._knownSymbols[vtxAcc] = fill;

			fill._setGlobalStrides(...stridedArrays);

			vtxAcc += fill.attrLength;
			// idxAcc += fill.idxLength;
		});

		this._commitStridedArrays(baseVtx, totalVertices /*, baseIdx, totalIndices*/);

		if (!this._crs) {
			// Fill symbols have been added before setting a CRS. The CRS of the first
			// Fill symbol shall be used temporarily.
			this._oldCrs = this._crs = fills[0].geom.crs;
		}

		const coordData = this.reproject(baseVtx, totalVertices, fills, true);

		// Once attributes are done (and coordinates have been projected),
		// do primitive indices.
		// Triangles are earcut, and the order depends on the specific projection.
		// That's the reason earcut is done here instead of inside `Fill` functionality:
		// the data needs to be in the display projection.
		this.cutFills(fills, coordData, baseVtx);

		super.multiAddIds(fills, baseVtx);

		return this;
	}

	/**
	 * Internal. Given an array of `Fill` symbols, and a compact `Float32Array` with
	 * their projected CRS coordinates, loops through their geometries, and
	 * runs `earcut` on them.
	 *
	 * This is common to both adding `Fill`s and reprojecting.
	 */
	cutFills(fills, coordData, baseVtx) {
		let idxOffset = 0; // Pointer to the item in the coordData typedarray

		const earcuttedFills = fills.map((fill) => {
			const d = fill.geom.dimension;
			const stops = [...fill.geom.hulls, fill.geom.coords.length / d];
			let start = 0;

			return stops
				.map((stop) => {
					const vertexCount = stop - start;

					// Get the ring offsets ("hole positions") for the current hull
					const rings = fill.geom.rings
						.filter((r) => r > start && r < stop)
						.map((r) => r - start);

					const idxs = earcut(
						coordData.slice(idxOffset * d, idxOffset * d + vertexCount * d),
						rings,
						d
					).map((i) => idxOffset + i);

					idxOffset += vertexCount;
					start = stop;

					return idxs;
				})
				.flat();
		});

		const flatEarcuts = earcuttedFills.flat();

		const totalIndices = flatEarcuts.length;

		// Skip degenerate case of adding fills with 2 or less vertices each,
		// which generate zero triangles when earcutted, which means zero
		// indices, which means allocation would error out.
		if (totalIndices > 0) {
			let baseIdx = this._indices.allocateSlots(totalIndices);

			// idxOffset changes semantics: now it's the offset in the IndexBuffer
			// where a `Fill` will start storing its triangle indices data.
			idxOffset = baseIdx;

			fills.forEach((fill, i) => {
				const length = earcuttedFills[i].length;
				fill.updateRefs(this, fill.attrBase, idxOffset);
				fill.idxLength = length;
				idxOffset += length;
			});

			// FillAcetate foregoes the usual strided arrays approach to triangle
			// indices, and instead does one single set() call to _indices.
			this._indices.set(
				baseIdx,
				flatEarcuts.map((i) => baseVtx + i)
			);
		}
		this.dirty = true;
	}

	/**
	 * @method reproject(start: Number, length: Number, skipEarcut?: Boolean, symbols?: Array of Fill): Array of Number
	 * As `AcetateVertices.reproject()`, but also recalculates the `earcut`
	 * triangulation for the affected symbols.
	 */
	reproject(start, length, symbols, skipEarcut = false) {
		const coordData = super.reproject(start, length, symbols);

		if (this._crs.name !== this._oldCrs.name && !skipEarcut) {
			/// This deallocates all triangles in a loop (CPU-bound, not a performance
			/// issue), then reallocates all of them; effectively removing and
			/// re-adding the triangle indices from the index buffer.
			/// This is because it might not be safe to assume that the triangle
			/// index data is compact for the relevant fills (which are guaranteed
			/// to be compact regarding their vertex attributes only).

			let relevantSymbols =
				symbols ??
				this._knownSymbols.filter((symbol, attrIdx) => {
					return (
						attrIdx >= start && attrIdx + symbol.attrLength <= start + length
					);
				});

			relevantSymbols.forEach((fill) => {
				this._indices.deallocateSlots(fill.idxBase, fill.idxLength);
			});

			//console.time("Earcutting due to reprojection");
			this.cutFills(relevantSymbols, coordData, start);
			//console.timeEnd("Earcutting due to reprojection");
		}

		return coordData;
	}
}

/**
 * @class Fill
 * @inherits GleoSymbol
 * @relationship drawnOn AcetateFill
 *
 * Simple (single-colour) fill for polygons.
 *
 * The `Geometry` might have any depth:
 * - Depth 1 is interpreted as a polygon with a single outer ring
 * - Depth 2 is interpreted as a polygon with an outer ring and inner rings
 * - Depth 3 is interpreted as a multipolygon
 *
 * This leverages [Volodymir Agafonkin's `earcut` library](https://github.com/mapbox/earcut).
 */

export default class Fill extends GleoSymbol {
	/// @section Static properties
	/// @property Acetate: Prototype of AcetateFill
	// The `Acetate` class that draws this symbol.
	static Acetate = AcetateFill;

	/**
	 * @constructor Fill(geom: Geometry, opts?: Fill Options)
	 */
	constructor(
		geom,
		{
			/**
			 * @section
			 * @aka Fill Options
			 * @option colour: Colour = '#3388ff33'
			 * The colour of the fill symbol.
			 */
			colour = [0x33, 0x88, 0xff, 0x33],
			...opts
		} = {}
	) {
		// Length of each linestring
		//this._lengths = linestrings.map((ls) => ls.length);
		super(geom, opts);

		// Amount of vertex attribute slots needed
		// Attribute slots is *half* of the lenght of the [x1,y2, ...xn,xy] flat array
		this.attrLength = this.geom.coords.length / this.geom.dimension;

		// Amount of index slots needed (calc'd by earcut)
		//this.idxLength = (this.attrLength - this._lengths.length) * 2;

		this.#colour = this.constructor._parseColour(colour);
		if (this.colour === null) {
			throw new Error("Invalid colour specified for Fill.");
		}
	}

	#colour;
	/**
	 * @property colour: Colour
	 * The colour for all the dots. Can be updated.
	 */
	get colour() {
		return this.#colour;
	}
	set colour(newColour) {
		this.#colour = this.constructor._parseColour(newColour);
		if (!this._inAcetate) {
			return this;
		}
		const stridedColour = this._inAcetate._attrs.asStridedArray(0);
		let end = this.attrBase + this.attrLength;
		for (let vtx = this.attrBase; vtx < end; vtx++) {
			stridedColour.set(this.#colour, vtx);
		}
		this._inAcetate._attrs.commit(this.attrBase, this.attrLength);
		this._inAcetate.dirty = true;
	}

	get geometry() {
		return super.geometry;
	}
	set geometry(geom) {
		const ac = this._inAcetate;
		if (ac) {
			ac.remove(this);
			super.geometry = geom;
			this.attrLength = this.geometry.coords.length / this.geometry.dimension;
			ac.add(this);
		} else {
			super.geometry = geom;
			this.attrLength = this.geometry.coords.length / this.geometry.dimension;
		}
		return this;
	}

	_setGlobalStrides(strideColour) {
		const attrMax = this.attrBase + this.attrLength;
		for (let i = this.attrBase; i < attrMax; i++) {
			strideColour.set(this.colour, i);
		}
		return this;
	}

	_setGeometryStrides() {
		// noop - earcut calculations run elsewhere.
	}

	// Can be overriden by subclasses or the `intensify` decorator
	static _parseColour = parseColour;
}
