import { registerFactory } from "../GliiFactory.mjs";
import { default as IndexBuffer } from "./IndexBuffer.mjs";
import { default as LoDAllocator } from "../LoDAllocator.mjs";

/**
 * @class LoDIndices
 * @inherits IndexBuffer
 * @relationship compositionOf LoDAllocator, 0..1, 1..1
 *
 * Similar to `SparseIndices`, a `LoDIndices` allows for allocating and
 * deallocating blocks of primitive slots via `allocateSlots` and `deallocateSlots`.
 *
 * The main difference is that each allocation must be done with a level-of-detail
 * (LoD) identifier, commonly a `Number` or a `String` (but also a `Symbol`).
 *
 * Like `SparseIndices`, calling `drawMe` will make a WebGL draw call per
 * allocation block. Unlike `SparseIndices`, only those blocks with a specific
 * LoD will be drawn.
 *
 */

export default class LoDIndices extends IndexBuffer {
	constructor(gl, gliiFactory, options = {}) {
		super(gl, gliiFactory, options);

		// Allocator instance for blocks in self's `ELEMENT_ARRAY_BUFFER`.
		if (this._growFactor) {
			this._slotAllocator = new LoDAllocator();
		} else {
			this._slotAllocator = new LoDAllocator(this._size);
		}
	}

	/**
	 * @method allocateSlots(count: Number, lod: Number): Number
	 * Allocates `count` slots for indices. Returns the offset of the first
	 * slot.
	 *
	 * For `gl.TRIANGLES` (`gl.LINES`), allocate 3 (2) slots per triangle (line).
	 * @alternative
	 * @method allocateSlots(count: Number, lod: String): Number
	 */
	allocateSlots(count, lod) {
		return this._slotAllocator.allocateBlock(count, lod);
	}

	/**
	 * @method allocateSet(indices: Array of Number, lod: Number, relative?:Boolean): Number
	 * Combination of `allocate()` and `set()`. Allocates the neccesary
	 * space for the given indices in the given LoD, and sets their values.
	 *
	 * If `relative` is set to `true`, then indices are considered to be
	 * relative to the allocation start (otherwise, they're absolute).
	 *
	 * Returns the offset of the allocation block start.
	 * @alternative
	 * @method allocateSet(indices: Array of Number, lod: String, relative?:Boolean): Number
	 */
	allocateSet(lod, indices, relative = false) {
		const start = this.allocateSlots(indices.length, lod);
		if (relative) {
			this.set(
				start,
				indices.map((i) => start + i)
			);
		} else {
			this.set(start, indices);
		}
		return start;
	}

	/**
	 * @method deallocateSlots(start, count: Number): this
	 * Deallocates `count` slots for indices, started with the `start`th slot.
	 *
	 * For `gl.TRIANGLES` (`gl.LINES`), allocate 3 (2) slots per triangle (line).
	 *
	 * All deallocated slots must belong to the same LoD. Otherwose, behaviour
	 * may be unpredictable.
	 * @alternative
	 * @method deallocateSlots(start, count: Number): this
	 */
	deallocateSlots(start, count) {
		this._slotAllocator.deallocateBlock(start, count);
		return this;
	}

	/**
	 * @method deallocateLoD(lod: Number): this
	 * Deallocates all the slots of the given LoD.
	 * @alternative
	 * @method deallocateLoD(lod: String): this
	 */
	deallocateLoD(lod) {
		return this.forEachBlock(lod, this.deallocateSlots.bind(this));
	}

	/**
	 * @method forEachBlock(lod: Number, fn: Function): this
	 * Runs the given callback `Function` `fn` once per allocated block, but
	 * only for blocks with the given LoD.
	 *
	 * The callback function shall receive the start and length of each
	 * allocated block. Both figures are given in number of *vertex slots*
	 * and not in primitives (i.e. divide by 3 when working with triangles).
	 * @alternative
	 * @method forEachBlock(lod: String, count: Number): Number
	 */
	forEachBlock(fn, lod) {
		this._slotAllocator.forEachBlock(fn, lod);
		return this;
	}

	/**
	 * @method copyWithin(target: Number, start: Number, end: Number): this
	 * Akin to `TypedArray.copyWithin()` copies indices from `start` to `end`
	 * into a section of itself, starting at `target`.
	 *
	 * Unlike `TypedArray.copyWithin()`, it will grow the data structures if needed.
	 *
	 * The typical use case is to copy a portion of a LoD into another LoD.
	 */
	copyWithin(target, start, end) {
		if (!this._ramData) {
			throw new Error("Cannot copyWithin() in a non-growable LoDIndices.");
		}
		this.grow(target + end - start);
		this._ramData.copyWithin(target, start, end);

		this._gl.bufferSubData(
			this._gl.ELEMENT_ARRAY_BUFFER,
			target * this._bytesPerSlot,
			this._ramData.subarray(start, end)
		);
		return this;
	}

	// Internal only. Does the GL drawElement() calls, but assumes that everyhing else
	// (bound program, textures, attribute name-locations, uniform name-locations-values)
	// has been set up already.
	drawMe(lod) {
		this.bindMe();
		this._slotAllocator.forEachBlock((start, length) => {
			const startByte = start * this._bytesPerSlot;
			this._gl.drawElements(this._drawMode, length, this._type, startByte);
		}, lod);
	}

	// Internal only. Does the GL drawElement() calls, but assumes that everyhing else
	// (bound program, textures, attribute name-locations, uniform name-locations-values)
	// has been set up already.
	drawMePartial(start, count) {
		this.bindMe();
		this._gl.drawElements(
			this._drawMode,
			count,
			this._type,
			start * this._bytesPerSlot
		);
	}
}

/**
 * @factory GliiFactory.LoDIndices(options: LoDIndices options)
 * @class Glii
 * @section Class wrappers
 * @property LoDIndices(options: LoDIndices options): Prototype of LoDIndices
 * Wrapped `LoDIndices` class
 */
registerFactory("LoDIndices", function (gl, gliiFactory) {
	return class WrappedLoDIndices extends LoDIndices {
		constructor(options) {
			super(gl, gliiFactory, options);
		}
	};
});
