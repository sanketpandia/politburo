import Allocator from "./Allocator.mjs";

/**
 * @class VerboseAllocator
 * @inherits Allocator
 *
 * An `allocator` that `console.log()`s its state after every allocation or
 * deallocation. Useful for (and meant only for!) debugging purposes.
 *
 */

export default class VerboseAllocator extends Allocator {
	allocateBlock(size) {
		const returnValue = super.allocateBlock(size);
		this.#log(`alloc ${returnValue.toString(16)} ${size.toString(16)}`);
		return returnValue;
	}

	deallocateBlock(start, size) {
		const returnValue = super.deallocateBlock(start, size);
		this.#log(`deloc ${start.toString(16)} ${size.toString(16)}`);
		return returnValue;
	}

	#log(title) {
		let ptr = 0;
		let str = title + ":";
		let styles = [];
		let point = this._points.get(ptr);
		while (ptr < this._max) {
			if (point.next === this._max) {
				str += ` %c${ptr.toString(16)}→MAX`;
			} else {
				str += ` %c${ptr.toString(16)}→${point.next.toString(16)}`;
			}
			if (point.free) {
				styles.push("color: darkgreen");
			} else {
				styles.push("color: darkred");
			}

			ptr = point.next;
			point = this._points.get(ptr);
		}
		console.log.apply(window, [str, ...styles]);
	}
}
