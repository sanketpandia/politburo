import Acetate from "./Acetate.mjs";
import Allocator from "../glii/src/Allocator.mjs";

/**
 * @class AcetateVertices
 * @inherits Acetate
 *
 * An abstract `Acetate` that implements multiple vertices per symbol.
 *
 * Most `Acetate`s draw symbols that must be represented by more than one vertex
 * (and typically forming triangles), and should inherit this functionality.
 *
 * The only exception is acetates that do not need vertex indices at all because
 * they do not rely on primitives (i.e. triangles) - the `AcetateDot` being the only
 * instance of such.
 */

export default class AcetateVertices extends Acetate {
	constructor(glii, opts) {
		super(glii, opts);

		// The SparseIndices allocates *vertex slots* on primitives, e.g.:
		// * 3 slots per triangle, or
		// * 2 slots per line segment
		this._indices = new this.glii.SparseIndices({
			// Glii defaults to UNSIGNED_SHORT, meaning a max of 2^16=65536
			// primitive vertex slots (~32k lines ~21k triangles). It's
			// reasonable to expect more, so this asks for 32-bit
			// pointers, meaning a max of 2^32 primitive slots.
			type: this.glii.UNSIGNED_INT,
		});

		// The attribute allocator allocates *attribute slots*,
		// one per needed vertex (even if that vertex is used several times in several
		// slots to be shared between several triangles/segments/primitives)
		this._attribAllocator = new Allocator();
	}

	glProgramDefinition() {
		const opts = super.glProgramDefinition();
		return {
			...opts,
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

	/**
	 * @section Internal Methods
	 * @uninheritable
	 * @method reproject(): this
	 * Runs `toCRS` on the coordinates of all known symbols, and (re)sets the values in
	 * the coordinates attribute buffer.
	 */
	reprojectAll() {
		this._attribAllocator.forEachBlock((start, length) => {
			this.reproject(start, length);
		});
		return this;
	}

	_getStridedArrays(_, maxIdx) {
		return [
			// Vertex indices
			this._indices.asTypedArray(maxIdx),
		];
	}

	_getPerPointStridedArrays(maxVtx, maxIdx) {
		return [];
	}

	_commitStridedArrays(_, __, baseIdx, idxCount) {
		this._indices.commit(baseIdx, idxCount);
	}

	_commitPerPointStridedArrays(vtx, vtxCount) {
		// noop
	}

	/**
	 * @method deallocate(symbol: GleoSymbol): this
	 * Deallocate the symbol from this acetate (so it's not drawn on the next refresh)
	 */
	deallocate(symbol) {
		return this.multiDeallocate([symbol]);
	}

	multiAllocate(symbols) {
		// Skip:
		// - Already added symbols
		// - Symbols with zero vertices
		// - Symbols with zero indices/triangles (e.g. one-point strokes)
		symbols = symbols.filter(
			(s) => isNaN(s.attrBase) && s.idxLength > 0 && s.attrLength > 0
		);
		if (symbols.length === 0) {
			return;
		}

		const totalVertices = symbols.reduce((acc, ext) => acc + ext.attrLength, 0);
		const baseVtx = this._attribAllocator.allocateBlock(totalVertices);
		let vtxAcc = baseVtx;

		const totalIndices = symbols.reduce((acc, ext) => acc + ext.idxLength, 0);
		const baseIdx = this._indices.allocateSlots(totalIndices);
		let idxAcc = baseIdx;

		let stridedArrays = this._getStridedArrays(
			baseVtx + totalVertices,
			baseIdx + totalIndices
		);

		symbols.forEach((sym) => {
			sym._inAcetate = this;
			sym.attrBase = vtxAcc;
			sym.idxBase = idxAcc;
			this._knownSymbols[vtxAcc] = sym;

			sym._setGlobalStrides(...stridedArrays);

			vtxAcc += sym.attrLength;
			idxAcc += sym.idxLength;
		});

		this._commitStridedArrays(baseVtx, totalVertices, baseIdx, totalIndices);

		if (this._crs) {
			this.reproject(baseVtx, totalVertices, symbols);
		}

		// The AcetateInteractive functionality will assign IDs to symbol vertices.
		this.multiAddIds?.(symbols, baseVtx, baseVtx + totalVertices);

		this.dirty = true;
		return this;
	}

	multiDeallocate(symbols) {
		symbols = symbols.filter((s) => !!s);
		symbols.sort((a, b) => a.idxBase - b.idxBase);

		if (symbols.length === 0) {
			return this;
		}

		// let attribBlocks = [];
		let blockStart = symbols[0].idxBase,
			blockLength = 0;
		symbols.forEach((symbol) => {
			if (blockStart + blockLength === symbol.idxBase) {
				blockLength += symbol.idxLength;
			} else {
				// attribBlocks.push([ blockStart, blockLength ]);
				this._indices.deallocateSlots(blockStart, blockLength);
				blockStart = symbol.idxBase;
				blockLength = symbol.idxLength;
			}
		});
		this._indices.deallocateSlots(blockStart, blockLength);

		symbols.sort((a, b) => a.attrBase - b.attrBase);

		blockStart = symbols[0].attrBase;
		blockLength = 0;

		symbols.forEach((symbol) => {
			if (blockStart + blockLength === symbol.attrBase) {
				blockLength += symbol.attrLength;
			} else {
				// attribBlocks.push([ blockStart, blockLength ]);
				this._attribAllocator.deallocateBlock(blockStart, blockLength);
				blockStart = symbol.attrBase;
				blockLength = symbol.attrLength;
			}
			delete this._knownSymbols[symbol.attrBase];
			symbol.updateRefs(undefined, undefined, undefined);
		});
		this._attribAllocator.deallocateBlock(blockStart, blockLength);

		// Edge case for the last symbol. See comments on Acetate.multiDeallocate().
		if (!this._knownSymbols.some(() => true)) {
			this._knownSymbols = [];
		}

		return this;
	}

	/**
	 * @method reproject(start: Number, length: Number, symbols?: Array of GleoSymbol): Array of Number
	 * Dumps a new set of values to the `this._coords` attribute buffer, based
	 * on the known set of symbols added to the acetate (only those which have
	 * their attribute offsets between `start` and `start+length`.
	 *
	 * If the list of symbols is already known, they can be passed as a third
	 * argument for a performance improvement.
	 *
	 * This default implementation **assumes** that the `attrLength` of a
	 * `GleoSymbol` is equal to the length of its `Geometry` (i.e. there's
	 * `one vertex per point in the geometry).
	 *
	 * Returns the data set into the attribute buffer: a ' Float32Array`
	 * in the form `[x1,y1, x2,y2, ... xn,yn]`.
	 */
	reproject(start, length, symbols) {
		const end = start + length;
		let maxIdx = -Infinity;
		let minIdx = Infinity;

		// In most cases, it's safe to assume that relevant symbols in the same
		// attribute allocation block have their vertex attributes in a
		// compacted manner.
		// The exception is tiles: tile vertex attributes are allocated in bulk
		// (enough to fill a whole texture atlas), before actually instantiating
		// tile symbols. Tile acetates shall overload this method.

		const stridedCoords = this._coords.asStridedArray(end);
		const geomStrides = this._getGeometryStridedArrays(end);

		const relevantSymbols =
			symbols ??
			this._knownSymbols.filter((symbol, attrIdx) => {
				return attrIdx >= start && attrIdx + symbol.attrLength <= start + length;
			});

		relevantSymbols.forEach((s) => {
			const geom = s.geometry.toCRS(this._crs);
			stridedCoords.set(geom.coords, s.attrBase);
			s._setGeometryStrides(geom, ...geomStrides);
			maxIdx = Math.max(maxIdx, s.idxBase + s.idxLength);
			minIdx = Math.min(maxIdx, s.idxBase);
		});

		this._coords.commit(start, length);
		this._commitGeometryStridedArrays(start, length, minIdx, maxIdx - minIdx);

		const coordData = new Float32Array(stridedCoords.buffer, start * 8, length * 2);
		super.expandBBox(coordData);
		return coordData;
	}
}
