import HeatMap from "./HeatMap.mjs";
// import {ScalarField} from "./Field.mjs";
import Acetate from "../acetates/Acetate.mjs";
import { scale } from "../3rd-party/gl-matrix/mat3.mjs";

/**
 * @class QuadBin
 * @inherits HeatMap
 *
 * As `HeatMap`, but using a scalar field with a much lower resolution.
 *
 * This is meant to use `intensify`d `Dot`s exclusively. Any other symbols (e.g.
 * `HeatPoint`s) will be scaled up by a factor equal to the cell size.
 *
 * @example
 * ```js
 * import QuadBin from "gleo/fields/QuadBin.mjs";
 * import intensify from "gleo/symboldecorators/intensify.mjs";
 * import Dot from "gleo/symbols/Dot.mjs";
 *
 * const IntensityDot = intensify(Dot);
 *
 * const heatbin = new QuadBin(map, {
 * 	// colour stops, cell size, etc
 * });
 *
 * new IntensityDot(geometry, { intensity: 100 }).addTo(heatbin);
 * ```
 */

export default class QuadBin extends HeatMap {
	#cellSize;
	#blurDuration;
	#blurOpacity;
	_offset;

	/**
	 * @constructor QuadBin(target: GliiFactory, opts?: QuadBin Options)
	 */
	constructor(
		target,
		{
			/**
			 * @option cellSize: Number = 32
			 * The size of cells, in CSS pixels.
			 */
			cellSize = 32,

			/**
			 * @option blurDuration: Number = 1
			 * The duration of the blur fade-in animation, in milliseconds.
			 *
			 * Setting this to zero will make the `QuadBin` (or `HexBin`)
			 * render immediately when the map is moved or zoomed. This causes
			 * bins to flicker rapidly, which is generally unpleasant to the
			 * eye.
			 *
			 * A value larger than zero will keep rendering semi-transparent
			 * bins each frame, until they "settle down" when the time is over.
			 */
			blurDuration = 150,

			...opts
		} = {}
	) {
		super(target, opts);

		this.#cellSize = cellSize;

		if (blurDuration < 0 || isNaN(blurDuration)) {
			throw new Error("Invalid blur time");
		}
		this.#blurDuration = blurDuration;
		this.#blurOpacity = 1;
	}

	#lastDirtyTimestamp; // in milliseconds, from performance.now();

	set dirty(d) {
		super.dirty = d;
		if (d) {
			this.#lastDirtyTimestamp = performance.now();
		}
	}
	get dirty() {
		return super.dirty || this.#blurOpacity < 1;
	}

	clear() {
		// Clear the framebuffer only if the parent functionality is dirty -
		// otherwise, keep the framebuffer dirty to draw on top and perform the fade-in.
		if (super.dirty) {
			super.clear();
		}
	}

	/// @property cellSize: Number
	/// The cell size, as defined by the homonymous option during instantiation. Read-only.
	get cellSize() {
		return this.#cellSize;
	}

	getFieldValueAt(x, y) {
		const dpr = devicePixelRatio ?? 1;
		const floorX = Math.floor((this._offset[0] * dpr + x) / this.#cellSize);
		const floorY = Math.ceil((this._offset[1] * dpr + y) / this.#cellSize);

		if (
			floorX > this.framebuffer.width ||
			floorY > this.framebuffer.height ||
			floorX < 0 ||
			floorY < 0
		) {
			console.error("getFieldValueAt: Cell out of bounds");
			return NaN;
		} else {
			return super.getFieldValueAt(floorX, floorY);
		}
	}

	glProgramDefinition() {
		const opts = super.glProgramDefinition();

		return {
			...opts,
			vertexShaderMain: `
				gl_Position = vec4(aPos * uFactor, 0., 1.);
				vUV = aUV;
			`,
			uniforms: {
				uFactor: "vec2",
			},
			blend: {
				equationRGB: this.glii.FUNC_ADD,
				equationAlpha: this.glii.FUNC_ADD,

				srcRGB: this.glii.CONSTANT_ALPHA,
				dstRGB: this.glii.ONE_MINUS_CONSTANT_ALPHA,
				srcAlpha: this.glii.CONSTANT_ALPHA,
				dstAlpha: this.glii.ONE_MINUS_CONSTANT_ALPHA,

				colour: [0, 0, 0, this.#blurOpacity],
			},
		};
	}

	resize(x_device, y_device, x_css, y_css) {
		// This resizes both the scalar field and the acetate RGBA output framebuffer,
		// so the RGBA framebuffer is resized back, after the super() call is done.

		const dpr = devicePixelRatio ?? 1;
		const dpr2 = dpr * 2;

		// Number of horizontal/vertical cells
		// Note this rounds up the size of the scalar field, thus (potentially)
		// making the cell size slightly smaller than the desired value.
		const cellX = Math.ceil(x_css / this.#cellSize);
		const cellY = Math.ceil(y_css / this.#cellSize);
		super.resize(cellX, cellY);

		// Size, in CSS pixels, of the quadbin's catchment area.
		const oversizeX = cellX * this.#cellSize;
		const oversizeY = cellY * this.#cellSize;

		this._factor = [x_css / oversizeX, y_css / oversizeY];
		this._offset = [(oversizeX - x_css) / dpr2, (oversizeY - y_css) / dpr2];

		this._program.setUniform(
			"uFactor",
			this._factor.map((n) => 1 / n)
		);

		Acetate.prototype.resize.call(this, x_device, y_device, x_css, y_css);
	}

	redraw(crs, matrix, viewportBbox) {
		let opacity =
			this.#blurDuration === 0
				? 1
				: (0.4 +
						((performance.now() - this.#lastDirtyTimestamp) /
							this.#blurDuration) *
							0.6) **
				  1.2;

		this.#blurOpacity = Math.min(opacity, 1);

		this._program.blend.colour[3] = this.#blurOpacity;

		// This will expand the affine transformation matrix and the viewport
		// bounding box by the quadbin's factor, in order to aggregate data
		// outside the visible bounds.
		// i.e. even if a data point falls just outside the visible bounds, *but*
		// inside a cell's catchment area, it has to be drawn into the scalar field.

		let box = viewportBbox
			.clone()
			.expandPercentages(this._factor[0] - 1, this._factor[1] - 1);

		let expandMatrix = scale(new Array(9), matrix, this._factor);

		return super.redraw(crs, expandMatrix, box);
	}
}
