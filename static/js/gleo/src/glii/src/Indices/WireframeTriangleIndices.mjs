import { registerFactory } from "../GliiFactory.mjs";
import { default as TriangleIndices } from "./TriangleIndices.mjs";

/**
 * @class WireframeTriangleIndices
 * @inherits TriangleIndices
 *
 * A decorated flavour of `TriangleIndices`; draws each triplet of vertices as a
 * `gl.LINE_LOOP` to produce a wireframe result.
 *
 * The width of the wireframe can be configured via the `width` constructor option.
 * Otherwise, this class works as a drop-in replacement for `TriangleIndices`.
 *
 * Note that the available line widths depend on your platform (i.e. graphics card +
 * web browser + OpenGL software stack). You should not assume that line widths
 * greater than 1 are available.
 */

export default class WireframeTriangleIndices extends TriangleIndices {
	constructor(gl, gliiFactory, options = {}) {
		super(gl, gliiFactory, options);
		/**
		 * @section
		 * @aka WireframeTriangleIndices options
		 * @option width: Number = 1; Width of the wireframe lines, in pixels.
		 */
		this._width = options.width || 1;
	}

	// Internal only. Does the GL drawElement() calls, but assumes that everyhing else
	// (bound program, textures, attribute name-locations, uniform name-locations-values)
	// has been set up already.
	drawMe() {
		this.bindMe();
		this._gl.lineWidth(this._width);
		this._slotAllocator.forEachBlock((start, length) => {
			for (let i = 0; i < length; i += 3) {
				const startByte = (start + i) * this._bytesPerSlot;
				this._gl.drawElements(this._gl.LINE_LOOP, 3, this._type, startByte);
			}
		});
	}

	drawMePartial(start, count) {
		this.bindMe();
		this._gl.lineWidth(this._width);
		for (let i = 0; i < count; i += 3) {
			const startByte = (start + i) * this._bytesPerSlot;
			this._gl.drawElements(this._gl.LINE_LOOP, 3, this._type, startByte);
		}
	}
}

/**
 * @class WireframeTriangleIndices
 * @factory GliiFactory.WireframeTriangleIndices(options: WireframeTriangleIndices options)
 * @class Glii
 * @section Class wrappers
 * @property WireframeTriangleIndices(options: WireframeTriangleIndices options): Prototype of WireframeTriangleIndices
 * Wrapped `WireframeTriangleIndices` class
 */
registerFactory("WireframeTriangleIndices", function (gl, gliiFactory) {
	return class WrappedWireframeTriangleIndices extends WireframeTriangleIndices {
		constructor(options) {
			super(gl, gliiFactory, options);
		}
	};
});
