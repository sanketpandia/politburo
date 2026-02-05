import { registerFactory } from "../GliiFactory.mjs";
import { default as IndexBuffer } from "./IndexBuffer.mjs";
import { default as Allocator } from "../Allocator.mjs";

/**
 * @class SparseIndices
 * @inherits IndexBuffer
 * @relationship compositionOf Allocator, 0..1, 1..1
 *
 * A `SparseIndices` is an `IndexBuffer`, plus an `Allocator` of index
 * slots - whenever more index slots for primitives are needed,
 * call `allocateSlots` (and later `deallocateSlots` if needed).
 *
 * (Drawing this `Indices` shall trigger one draw call per contiguous
 * block of used indices. By comparison, the basic `IndexBuffer` triggers
 * just one `drawElements` call, from 0 to number-of-used-indices -1)
 *
 */

export default class SparseIndices extends IndexBuffer {
	constructor(gl, gliiFactory, options = {}) {
		super(gl, gliiFactory, options);

		// Allocator instance for blocks in self's `ELEMENT_ARRAY_BUFFER`.
		if (this._growFactor) {
			this._slotAllocator = new Allocator();
		} else {
			this._slotAllocator = new Allocator(this._size);
		}
	}

	/**
	 * @method allocateSlots(count: Number): Number
	 * Allocates `count` slots for indices. Returns the offset of the first
	 * slot.
	 *
	 * For `gl.TRIANGLES` (`gl.LINES`), allocate 3 (2) slots per triangle (line).
	 */
	allocateSlots(count) {
		const start = this._slotAllocator.allocateBlock(count);
		this.grow(start + count);
		return start;
	}

	/**
	 * @method deallocateSlots(start: Number, count: Number): this
	 * Deallocates `count` slots for indices, started with the `start`th slot.
	 *
	 * For `gl.TRIANGLES` (`gl.LINES`), allocate 3 (2) slots per triangle (line).
	 */
	deallocateSlots(start, count) {
		this._slotAllocator.deallocateBlock(start, count);
		return this;
	}

	/**
	 * @method allocateSet(indices: Array of Number): Number
	 * Combination of `allocate()` and `set()`. Allocates the neccesary
	 * space for the given indices, and sets their values.
	 *
	 * Returns the offset of the allocation block start.
	 */
	allocateSet(indices) {
		const start = this.allocateSlots(indices.length);
		this.set(start, indices);
		return start;
	}

	/**
	 * @method forEachBlock(fn: Function): this
	 * Runs the given callback `Function` `fn` once per allocated block.
	 *
	 * The callback function shall receive the start and length of each
	 * allocated block. Both figures are given in number of *vertex slots*
	 * and not in primitives (i.e. divide by 3 when working with triangles).
	 */
	forEachBlock(fn) {
		this._slotAllocator.forEachBlock(fn);
		return this;
	}

	truncate(n) {
		return this.deallocateSlots(n, Number.MAX_SAFE_INTEGER);
	}

	// Internal only. Does the GL drawElement() calls, but assumes that everyhing else
	// (bound program, textures, attribute name-locations, uniform name-locations-values)
	// has been set up already.
	drawMe() {
		this.bindMe();
		this._slotAllocator.forEachBlock((start, length) => {
			const startByte = start * this._bytesPerSlot;
			this._gl.drawElements(this._drawMode, length, this._type, startByte);
		});
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
 * @factory GliiFactory.SparseIndices(options: SparseIndices options)
 * @class Glii
 * @section Class wrappers
 * @property SparseIndices(options: SparseIndices options): Prototype of SparseIndices
 * Wrapped `SparseIndices` class
 */
registerFactory("SparseIndices", function (gl, gliiFactory) {
	return class WrappedSparseIndices extends SparseIndices {
		constructor(options) {
			super(gl, gliiFactory, options);
		}
	};
});
