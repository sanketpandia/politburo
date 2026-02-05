import { registerFactory } from "../GliiFactory.mjs";
import SequentialIndices from "./SequentialIndices.mjs";
import { default as Allocator } from "../Allocator.mjs";

/**
 * @class SequentialSparseIndices
 * @inherits SequentialIndices
 * @relationship compositionOf Allocator, 0..1, 1..1
 *
 * Works as a `SequentialIndices` in that makes `drawArrays` calls, but one
 * call per allocated block (as per `SparseIndices`).
 */

export default class SequentialSparseIndices extends SequentialIndices {
	constructor(gl, options = {}) {
		super(gl, options);
		this._slotAllocator = new Allocator();
	}

	/**
	 * @method allocateSlots(count: Number): Number
	 * Allocates `count` slots for indices. Returns the offset of the first
	 * slot.
	 *
	 * For `gl.TRIANGLES` (`gl.LINES`), allocate 3 (2) slots per triangle (line).
	 */
	allocateSlots(count) {
		return this._slotAllocator.allocateBlock(count);
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

	// Internal only. Does the GL drawElement() calls, but assumes that everyhing else
	// (bound program, textures, attribute name-locations, uniform name-locations-values)
	// has been set up already.
	drawMe() {
		this._slotAllocator.forEachBlock((start, length) => {
			this._gl.drawArrays(this._drawMode, start, length);
		});
	}

	// Internal only. Does the GL drawElement() calls, but assumes that everyhing else
	// (bound program, textures, attribute name-locations, uniform name-locations-values)
	// has been set up already.
	drawMePartial(start, count) {
		this._gl.drawArrays(this._drawMode, start, count);
	}
}

/**
 * @factory GliiFactory.SparseIndices(options: SparseIndices options)
 * @class Glii
 * @section Class wrappers
 * @property SparseIndices(options: SparseIndices options): Prototype of SparseIndices
 * Wrapped `SparseIndices` class
 */
registerFactory("SequentialSparseIndices", function (gl) {
	return class WrappedSequentialSparseIndices extends SequentialSparseIndices {
		constructor(options) {
			super(gl, options);
		}
	};
});
