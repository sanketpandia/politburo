/**
 * @class Allocator
 *
 * The purpose of an `Allocator` is to provide a way to request (and
 * return) blocks of 0-indexed IDs.
 *
 * The general idea is that a user should request N points/linesegments/triangles,
 * and this class would be responsible for creating numeric indices for them.
 * Then, the allocated indices would use up space in a `PointIndices`/
 * `LineSegmentIndices`/`TriangleIndices`.
 *
 * @example
 *
 * The `Allocator` class is `import`ed directly from the `glii` module:
 *
 * ```
 * import { default as Glii, Allocator } from "path_to_glii/index.mjs";
 *
 * const glii = GliiFactory(// etc //);
 *
 * const myAllocator = new Allocator();
 * ```
 *
 * Trying to spawn an `Allocator` from a `GliiFactory` will fail:
 *
 * ```
 * import { default as Glii, Allocator } from "path_to_glii/index.mjs";
 * const glii = GliiFactory(// etc //);
 *
 * const myAllocator = new glii.Allocator();	// BAD!
 * ```
 *
 */

// This is kinda similar to https://github.com/redboltz/number-allocator ,
// but allows allocating a range, and has worse complexity (O(n) instead of
// O(n·log(n)) ).

export default class Allocator {
	constructor(max = Number.MAX_SAFE_INTEGER) {
		/**
		 * @constructor Allocator(max: Number)
		 * Creates a new `Allocator` instance, given the upper limit
		 * of the allocatable area.
		 */

		this._max = max;
		// The 'points' structure is effectively a linked list of
		// start points of free/allocated regions.
		this._points = new Map();
		this._points.set(0, {
			free: true,
			next: max,
		});
	}

	/**
	 * @method allocateBlock(size:Number): Number
	 * Given the count of IDs to allocate, returns a `Number` with the
	 * first ID of the allocated block (last would be return + count - 1)
	 */
	allocateBlock(size) {
		if (size === 0) {
			return NaN;
		}
		let prev = 0;
		let ptr = 0;

		while (true) {
			const block = this._points.get(ptr);
			const end = ptr + size;

			if (block.free) {
				if (ptr === 0 && end < block.next) {
					// Allocate at the very beginning
					this._points.set(0, { free: false, next: end });
					this._points.set(end, { free: true, next: block.next });
					return 0;
				}
				if (ptr === 0 && end === block.next) {
					// Allocate at the very beginning, merge with next block
					const nextBlock = this._points.get(end);
					this._points.set(0, { free: false, next: nextBlock.next });
					this._points.delete(block.next);
					return 0;
				}
				if (end < block.next) {
					// Increase the size of the previous, used, block
					this._points.set(prev, { free: false, next: end });
					this._points.delete(ptr);
					this._points.set(end, { free: true, next: block.next });
					return ptr;
				}
				if (end === block.next) {
					// Allocate an entire free block,
					// merge neighbouring used blocks
					const nextBlock = this._points.get(end);
					this._points.set(prev, { free: false, next: nextBlock.next });
					this._points.delete(ptr);
					this._points.delete(block.next);
					return ptr;
				}
			}

			prev = ptr;
			ptr = block.next;
			if (ptr <= prev) {
				throw new Error(`Bad allocation map: tried to go backwards`);
			}
			// 			if (ptr === Number.MAX_SAFE_INTEGER) {
			if (ptr >= this._max) {
				throw new Error(`No allocatable space`);
			}
		}
	}

	/**
	 * @method deallocateBlock([Number]): this
	 * Given a starting ID and the size of a block, deallocates that block
	 * (marks it as allocatable again)
	 */
	deallocateBlock(start, size) {
		if (size === 0) {
			return this;
		}
		let prev = 0;
		let ptr = 0;
		const end = start + size;

		while (true) {
			const block = this._points.get(ptr);

			if (!block.free) {
				if (ptr === 0 && start === 0 && end === block.next) {
					// Deallocate entire block at beginning
					const nextBlock = this._points.get(end);
					this._points.set(0, { free: true, next: nextBlock.next });
					this._points.delete(end);
					return this;
				}
				if (ptr === 0 && start === 0 && end < block.next) {
					// Deallocate partial block at beginning,
					// lower next block start
					this._points.set(0, { free: true, next: end });
					this._points.set(end, { free: false, next: block.next });
					return this;
				}

				if (ptr === start && end < block.next) {
					// Deallocate at the beginning of a used block
					// Grow the previous free block
					this._points.set(prev, { free: true, next: end });
					this._points.delete(ptr);
					this._points.set(end, { free: false, next: block.next });
					return this;
				}
				if (ptr === start && end === block.next) {
					// Deallocate the entire block
					// Merge neighbouring free blocks
					const nextBlock = this._points.get(block.next);
					this._points.set(prev, { free: true, next: nextBlock.next });
					this._points.delete(ptr);
					this._points.delete(block.next);
					return this;
				}
				if (ptr < start && end === block.next) {
					// Deallocate the end of the block
					// Grow the next free block
					const nextBlock = this._points.get(block.next);
					this._points.set(ptr, { free: false, next: start });
					this._points.delete(block.next);
					this._points.set(start, { free: true, next: nextBlock.next });
					return this;
				}
				if (ptr < start && end < block.next) {
					// Deallocate middle of a block
					this._points.set(ptr, { free: false, next: start });
					this._points.set(start, { free: true, next: end });
					this._points.set(end, { free: false, next: block.next });
					return;
				}
			}

			prev = ptr;
			ptr = block.next;
			if (ptr <= prev) {
				throw new Error(`Bad allocation map: tried to go backwards`);
			}
			// if (start === Number.MAX_SAFE_INTEGER) {
			if (ptr >= this._max) {
				throw new Error(`Could not deallocate. Sparse?`);
			}
		}
	}

	/**
	 * @method forEachBlock(fn: Function): this
	 * Runs the given callback `Function` `fn`. `fn` receives
	 * the start and length of each allocated block as its two
	 * parameters.
	 */
	forEachBlock(fn) {
		if (this._points.size <= 1) {
			return this;
		}
		let ptr = 0;
		while (true) {
			const block = this._points.get(ptr);
			if (!block) {
				throw new Error(
					`Bad allocation map: was modified inside a forEach() callback`
				);
			}
			if (!block.free) {
				fn(ptr, block.next - ptr);
			}
			if (block.next <= ptr) {
				throw new Error(`Bad allocation map: tried to go backwards`);
			}
			ptr = block.next;
			if (ptr >= this._max) {
				return this;
			}
		}
	}
}
