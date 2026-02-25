import { registerFactory } from "../GliiFactory.mjs";
import { default as SparseIndices } from "./SparseIndices.mjs";

/**
 * @class TriangleIndices
 * @inherits SparseIndices
 * @relationship associated Triangle
 * @relationship associated Quad
 *
 * A decorated flavour of `SparseIndices`. Allows spawning instances of the `Triangle`
 * utility class.
 *
 * The `drawMode` of a `TriangleIndices` is forced to `gl.TRIANGLES`.
 *
 * @example
 *
 * ```
 * const myTriangles = new GliiFactory.TriangleIndices();
 *
 * const trig1 = new myTriangles.Triangle(0,1,2);
 * trig1.setVertices(0,1,2);
 * trig1.destroy();
 * ```
 */

export default class TriangleIndices extends SparseIndices {
	constructor(gl, gliiFactory, options = {}) {
		/**
		 * @section
		 * @aka TriangleIndices options
		 * @option size: Number = 85; The (initial) amount of triangles (not vertices) to hold
		 */
		options.size = (options.size || 85) * 3;
		super(gl, options);

		const container = this;
		/**
		 * @property Triangle: Triangle prototype
		 * Class prototype for `Triangle`s.
		 */
		this.Triangle = class WrappedTriangle extends Triangle {
			constructor() {
				super(container);
			}
		};

		/**
		 * @property Quad: Quad prototype
		 * Class prototype for `Quad`s.
		 */
		this.Quad = class WrappedQuad extends Quad {
			constructor() {
				super(container);
			}
		};
	}

	/**
	 * @method allocateSlots(count: Number): Number
	 * Allocates `count` slots for indices, `count` must be a multiple of 3.
	 * Returns the offset of the first slot.
	 */
	allocateSlots(count) {
		if (count % 3) {
			throw new Error(
				"Number of vertices to be allocated from a TriangleIndices must be a multiple of 3"
			);
		}
		return super.allocateSlots(count);
	}

	/**
	 * @method deallocateSlots(start, count: Number): this
	 * Deallocates `count` slots for indices, started with the `start`th slot.
	 * Both `start` and `count` must be multiples of 3.
	 */
	deallocateSlots(start, count) {
		if (count % 3) {
			throw new Error(
				"Number of vertices to be deallocated from a TriangleIndices must be a multiple of 3"
			);
		}
		if (start % 3) {
			throw new Error(
				"The starting slot to be deallocated from a TriangleIndices must be a multiple of 3"
			);
		}
		this._slotAllocator.deallocateBlock(start, count);
		return this;
	}

	// Internal only. Does the GL drawElement() calls, but assumes that everyhing else
	// (bound program, textures, attribute name-locations, uniform name-locations-values)
	// has been set up already.
	drawMe() {
		this.bindMe();
		this._slotAllocator.forEachBlock((start, length) => {
			const startByte = start * this._bytesPerSlot;
			this._gl.drawElements(this._gl.TRIANGLES, length, this._type, startByte);
		});
	}
}

/**
 * @class Triangle
 *
 * Sintactic sugar over a triplet of vertex indices.
 *
 * Cannot be instantiated directly; a `TriangleIndices` must be used (since every
 * `Triangle` belongs to one and only one `TriangleIndices`).
 *
 * Instantiating a `Triangle` automativally allocates vertex slots in
 * the `TriangleIndices`.
 *
 * @example
 * ```
 * const trigs = new glii.TriangleIndices( // etc // );
 *
 * let trig1 = new trigs.Triangle();
 * trig1.setVertices(1000,1001,1002);
 * ```
 */

class Triangle {
	constructor(indices) {
		this._container = indices;
		this._idx = indices.allocateSlots(3);
		this._allocated = true;
	}

	/**
	 * @method setVertices(v1: Number, v2: Number, v3: Number): this
	 * Sets the vertex indices for this triangle.
	 *
	 * The indices passed must exist (and likely, have values) in any
	 * `AttributeBuffer`s being used together with the containing `TriangleIndices`
	 * in a GL program.
	 *
	 * Internally, this is akin to calling `IndexBuffer.set(idx, [v1,v2,v3])`,
	 * if `idx` to `idx+2` were allocated manually.
	 */
	setVertices(v1, v2, v3) {
		if (!this._allocated) {
			throw new Error(
				"Cannot set vertices in a `Triangle` which has been destroyed."
			);
		}
		this._container.set(this._idx, [v1, v2, v3]);
		return this;
	}

	/**
	 * @method destroy():this
	 * Signals the containing `TriangleIndices` that this triangle should not
	 * be drawn anymore.
	 *
	 * [`delete`](https://developer.mozilla.org/en-US/docs/Web/JavaScript/Reference/Operators/delete)ing
	 * all references to this instances manually, right afterwards, is encouraged.
	 */
	destroy() {
		this._container.deallocateSlots(this._idx, 3);
		this._allocated = false;
		return this;
	}
}

/**
 * @class Quad
 *
 * Sintactic sugar over two triangles forming a quadrangle (hence, "quad").
 *
 * Cannot be instantiated directly; a `TriangleIndices` must be used (since every
 * `Quad` belongs to one and only one `TriangleIndices`).
 *
 * Instantiating a `Quad` automativally allocates vertex slots in
 * the `TriangleIndices` enough for two triangles.
 *
 * @example
 * ```
 * const trigs = new glii.TriangleIndices( // etc // );
 *
 * let quad1 = new trigs.Quad();
 * quad1.setVertices(1000,1001,1002,1003);
 * ```
 */

class Quad {
	constructor(indices) {
		this._container = indices;
		this._idx = indices.allocateSlots(6);
		this._allocated = true;
	}

	/**
	 * @method setVertices(v1: Number, v2: Number, v3: Number, v4: Number): this
	 * Sets the vertex indices for this quad.
	 *
	 * The indices passed must exist (and likely, have values) in any
	 * `AttributeBuffer`s being used together with the containing `TriangleIndices`
	 * in a GL program.
	 *
	 * Internally, this is akin to calling `IndexBuffer.set(idx, [v1,v2,v3])`
	 * and `IndexBuffer.set(idx, [v1,v3,v4])`, if `idx` to `idx+5`
	 * were allocated manually.
	 */
	setVertices(v1, v2, v3, v4) {
		if (!this._allocated) {
			throw new Error("Cannot set vertices in a `Quad` which has been destroyed.");
		}
		this._container.set(this._idx, [v1, v2, v3]);
		this._container.set(this._idx + 3, [v1, v3, v4]);
		return this;
	}

	/**
	 * @method destroy():this
	 * Signals the containing `TriangleIndices` that this triangle should not
	 * be drawn anymore.
	 *
	 * [`delete`](https://developer.mozilla.org/en-US/docs/Web/JavaScript/Reference/Operators/delete)ing
	 * all references to this instances manually, right afterwards, is encouraged.
	 */
	destroy() {
		this._container.deallocateSlots(this._idx, 6);
		this._allocated = false;
	}
}

/**
 * @class TriangleIndices
 * @factory GliiFactory.TriangleIndices(options: TriangleIndices options)
 * @class Glii
 * @section Class wrappers
 * @property TriangleIndices(options: TriangleIndices options): Prototype of TriangleIndices
 * Wrapped `TriangleIndices` class
 */
registerFactory("TriangleIndices", function (gl, gliiFactory) {
	return class WrappedTriangleIndices extends TriangleIndices {
		constructor(options) {
			super(gl, gliiFactory, options);
		}
	};
});
