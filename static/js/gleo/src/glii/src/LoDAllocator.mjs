/**
 * @class LoDAllocator
 *
 * An expansion of `Allocator`; its purpose is to allocate blocks linked to
 * an identifier; and instead of iterating through allocated blocks, can
 * iterate through a filtered subset of blocks.
 *
 * The idea is to allow for level-of-detail (LoD) triangle meshes. Several
 * meshes with different LoD IDs can be allocated on the same underlying
 * `IndexBuffer`, and then one specific LoD can be filtered out to be drawn.
 *
 * @example
 *
 * The `LoDAllocator` class is `import`ed directly from the `glii` module:
 *
 * ```
 * import { default as Glii } from "path_to_glii/index.mjs";
 * import { default as LoDAllocator } from "path_to_glii/LoDAllocator.mjs";
 *
 * const glii = GliiFactory(// etc //);
 *
 * const myAllocator = new LoDAllocator();
 * ```
 *
 * Trying to spawn an `LoDAllocator` from a `GliiFactory` will fail:
 *
 * ```
 * import { default as Glii, LoDAllocator } from "path_to_glii/index.mjs";
 * const glii = GliiFactory(// etc //);
 *
 * const myAllocator = new glii.LoDAllocator();	// BAD!
 * ```
 *
 */

export default class LoDAllocator {
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
			data: undefined,
		});
	}

	/**
	 * @method allocateBlock(size:Number, data: Number): Number
	 * Given the count of IDs to allocate, returns a `Number` with the
	 * first ID of the allocated block (last would be return + count - 1).
	 *
	 * Receives an numerical `data` parameter that will be attached
	 * internally to the allocation (and shall be used for filtering
	 * allocation blocks later on). This should be the LoD (if the LoD is
	 * numerical)
	 * @alternative
	 * @method allocateBlock(size:Number, data: String): Number
	 * Can take a `String` as the LoD identifier as well.
	 * @method allocateBlock(size:Number, data: Object): Number
	 * Can take any `Object` as the LoD identifier as well. Do note that,
	 * internally, the `===` equality operator is used to check equality
	 * of LoD identifiers among allocation blocks.
	 */
	allocateBlock(size, data) {
		let prev = 0;
		let prevBlock;
		let ptr = 0;

		while (true) {
			const block = this._points.get(ptr);
			const end = ptr + size;

			if (block.free) {
				if (ptr === 0 && end < block.next) {
					// Allocate at the very beginning, leave gap
					this._points.set(0, { free: false, next: end, data });
					this._points.set(end, { free: true, next: block.next });
					return 0;
				}

				const nextBlock = this._points.get(end);

				if (ptr === 0 && end === block.next) {
					if (data === nextBlock.data) {
						// Allocate at the very beginning, merge with next block
						this._points.set(0, { free: false, next: nextBlock.next, data });
						this._points.delete(block.next);
						return 0;
					} else {
						// Allocate at the very beginning, do not merge with next block
						this._points.set(0, { free: false, next: end, data });
						//this._points.set(end, { free: true, next: block.next });
						return 0;
					}
				}
				if (end < block.next) {
					if (prevBlock.data === data) {
						// Increase the size of the previous, used, block
						this._points.set(prev, { free: false, next: end, data });
						this._points.delete(ptr);
						this._points.set(end, { free: true, next: block.next });
						return ptr;
					} else {
						// Allocate next to previous, used, block
						this._points.set(ptr, { free: false, next: end, data });
						this._points.set(end, { free: true, next: block.next });
						return ptr;
					}
				}
				if (end === block.next) {
					if (prevBlock.data === data && nextBlock.data === data) {
						// Allocate an entire free block,
						// merge neighbouring used blocks
						this._points.set(prev, { free: false, next: nextBlock.next });
						this._points.delete(ptr);
						this._points.delete(block.next);
						return ptr;
					} else if (prevBlock.data === data && nextBlock.data !== data) {
						// Merge with the previous block
						this._points.set(prev, { free: false, next: block.next });
						this._points.delete(ptr);
						return ptr;
					} else if (prevBlock.data !== data && nextBlock.data === data) {
						// Merge with the next block
						this._points.set(ptr, {
							free: false,
							next: nextBlock.next,
							data,
						});
						this._points.delete(block.next);
					} else {
						// Set block as allocated, do not merge anything
						this._points.set(ptr, { free: false, next: end, data });
						return ptr;
					}
				}
			}

			prev = ptr;
			prevBlock = block;
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
	 * @method deallocateBlock(start: Number, size: Number): this
	 * Given a starting ID and the size of a block, deallocates that block
	 * (marks it as allocatable again).
	 *
	 * The given start and size must fall within the same allocation block (i.e.
	 * the deallocation must correspond to just one LoD).
	 */
	deallocateBlock(start, size) {
		let prev = 0;
		let prevBlock;
		let ptr = 0;
		const end = start + size;

		while (true) {
			const block = this._points.get(ptr);

			if (!block.free) {
				const nextBlock = this._points.get(end);

				if (ptr === 0 && start === 0 && end === block.next) {
					if (nextBlock.free) {
						// Deallocate entire block at beginning, grow
						// next free block
						this._points.set(0, { free: true, next: nextBlock.next });
						this._points.delete(end);
						return this;
					} else {
						// Deallocate entire block at beginning, ignore
						// next used block
						this._points.set(0, { free: true, next: block.next });
						return this;
					}
				} else if (ptr === 0 && start === 0 && end < block.next) {
					// Deallocate partial block at beginning,
					// lower next block start
					this._points.set(0, { free: true, next: end });
					this._points.set(end, {
						free: false,
						next: block.next,
						data: block.data,
					});
					return this;
				} else if (ptr === start && end < block.next) {
					if (prevBlock.free) {
						// Deallocate at the beginning of a used block
						// Grow the previous free block
						this._points.set(prev, { free: true, next: end });
						this._points.delete(ptr);
						this._points.set(end, {
							free: false,
							next: block.next,
							data: block.data,
						});
						return this;
					} else {
						// Deallocate at the beginning of a used block,
						// ignore previous free block
						this._points.set(ptr, { free: true, next: end });
						this._points.set(end, {
							free: false,
							next: block.next,
							data: block.data,
						});
						return this;
					}
				} else if (ptr === start && end === block.next) {
					if (prevBlock.free && nextBlock.free) {
						// Deallocate the entire block
						// Merge neighbouring free blocks
						this._points.set(prev, { free: true, next: nextBlock.next });
						this._points.delete(ptr);
						this._points.delete(block.next);
						return this;
					} else if (!prevBlock.free && !nextBlock.free) {
						// Deallocate the entire block
						// Ignore neighbouring used blocks
						this._points.set(ptr, { free: true, next: block.next });
						return this;
					} else if (prevBlock.free && !nextBlock.free) {
						// Deallocate the entire block
						// Merge previous free block
						this._points.set(prev, { free: true, next: block.next });
						this._points.delete(ptr);
						return this;
					} else if (!prevBlock.free && nextBlock.free) {
						// Deallocate the entire block
						// Merge next free block
						this._points.set(ptr, { free: true, next: nextBlock.next });
						this._points.delete(block.next);
						return this;
					}
				} else if (ptr < start && end === block.next) {
					if (nextBlock.free) {
						// Deallocate the end of the block
						// Grow the next free block
						this._points.set(ptr, {
							free: false,
							next: start,
							data: block.data,
						});
						this._points.delete(block.next);
						this._points.set(start, { free: true, next: nextBlock.next });
						return this;
					} else {
						// Deallocate the end of the block
						// Ignore next used block
						this._points.set(ptr, {
							free: false,
							next: start,
							data: block.data,
						});
						this._points.set(start, { free: true, next: block.next });
						return this;
					}
				} else if (ptr < start && end < block.next) {
					// Deallocate middle of a block
					this._points.set(ptr, { free: false, next: start, data: block.data });
					this._points.set(start, { free: true, next: end });
					this._points.set(end, {
						free: false,
						next: block.next,
						data: block.data,
					});
					return this;
				}
			}

			prev = ptr;
			prevBlock = block;
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
	 * @method forEachBlock(fn: Function, data: Number): this
	 * Runs the given callback `Function` `fn`, only on blocks allocated with
	 * an LoD identifier (`data`) exactly equal (`===`) to the given one. `fn`
	 * receives the start and length of each allocated block as its two
	 * parameters.
	 * @alternative
	 * @method forEachBlock(fn: Function, data: String): this
	 * @alternative
	 * @method forEachBlock(fn: Function, data: undefined): this
	 * Runs the given fallback `Function` `fn` on all allocated blocks.
	 */
	forEachBlock(fn, data) {
		let ptr = 0;
		while (true) {
			const block = this._points.get(ptr);
			if (!block.free && (data === undefined || block.data === data)) {
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
