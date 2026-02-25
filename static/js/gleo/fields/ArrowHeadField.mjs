// import QuadBin from "./QuadBin.mjs";
import { VectorField } from "./Field.mjs";
import { scale } from "../3rd-party/gl-matrix/mat3.mjs";
import Acetate from "../acetates/Acetate.mjs";

/**
 * @class ArrowHeadField
 * @inherits VectorField
 *
 * A low-resolution `VectorField` that displays small arrowheads.
 *
 * Similar to `QuadMarginBin`: uses a low-resolution framebuffer,
 * and uses triangles on a per-cell basis to render.
 *
 *
 *
 */

export default class ArrowHeadField extends VectorField {
	#cellSize;
	_offset;
	#pxPerSlopeUnit;

	/**
	 * @constructor ArrowHeadField(target: GliiFactory, opts?: ArrowHeadField Options)
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
			 * @option pxPerSlopeUnit: Number = 1
			 * The length (in CSS pixels) of the arrowhead per unit of
			 * slope. In other words: the scale factor between the slope vector
			 * (in slope units) and the length of the arrowhead (in CSS pixels)
			 */
			pxPerSlopeUnit = 1,

			...opts
		} = {}
	) {
		super(target, opts);

		this.#cellSize = cellSize;
		this.#pxPerSlopeUnit = pxPerSlopeUnit;

		this._attrs = new this.glii.InterleavedAttributes(
			{
				usage: this.glii.STATIC_DRAW,
				size: 4,
				growFactor: 1,
			},
			[
				{
					// Texel coords
					glslType: "vec2",
					type: Float32Array,
					normalized: false,
				},
				{
					// clipspace coords for the center of the cell
					glslType: "vec2",
					type: Float32Array,
					normalized: false,
				},
				{
					// Vertex extrusion for a [1,0] slope vector
					glslType: "vec2",
					type: Float32Array,
					normalized: false,
				},
			]
		);

		this._indexBuffer = new this.glii.IndexBuffer({
			growFactor: 1,
			type: this.glii.UNSIGNED_INT,
		});
	}

	/// @property cellSize: Number
	/// The cell size, as defined by the homonymous option during instantiation. Read-only.
	get cellSize() {
		return this.#cellSize;
	}

	getFieldValueAt(x, y) {
		const dpr = devicePixelRatio ?? 1;
		const floorX = Math.floor((this._offsetX * dpr + x) / this.#cellSize);
		const floorY = Math.ceil((this._offsetY * dpr + y) / this.#cellSize);

		// console.log("getFieldValueAt", floorX, floorY, x, y);

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
			indexBuffer: this._indexBuffer,
			attributes: {
				aUV: this._attrs.getBindableAttribute(0),
				aPos: this._attrs.getBindableAttribute(1),
				aExtrude: this._attrs.getBindableAttribute(2),
			},
			uniforms: {
				uFactor: "vec2",
				uPixelSize: "vec2",
				// uCellSize: "float",
				...opts.uniforms,
			},
			vertexShaderMain: `
				vec2 value = texture2D(uField, aUV).xy;

				gl_Position = vec4((aPos * uFactor) + (
					((value.x * aExtrude) +	// Horizontal component
					(value.y * aExtrude.yx * vec2(-1.,1.))) // Vertical component
					* uPixelSize)
				, 0., 1.);
			`,
			varyings: {
				vColour: "vec4",
			},
			fragmentShaderMain: `
				// gl_FragColor = vColour;
				gl_FragColor = vec4(0., 0., 0., 1.);
			`,
		};
	}

	// Mostly copied from QuadMarginBin
	resize(x_device, y_device, x_css, y_css) {
		const dpr = devicePixelRatio ?? 1;
		const dpr2 = dpr * 2;

		// Note this rounds up the size of the scalar field, thus (potentially)
		// making the cell size slightly smaller than the desired value.
		const cellX = Math.ceil(x_css / this.#cellSize);
		const cellY = Math.ceil(y_css / this.#cellSize);

		// This resizes just the vector field; the acetate RGBA output framebuffer
		// is kept the same size
		super.resize(cellX, cellY);
		Acetate.prototype.resize.call(this, x_device, y_device, x_css, y_css);

		// Size, in CSS pixels, of the quadbin's catchment area.
		const oversizeX = cellX * this.#cellSize;
		const oversizeY = cellY * this.#cellSize;

		this._factor = [x_css / oversizeX, y_css / oversizeY];
		const offsetX = (this._offsetX = (oversizeX - x_css) / 2);
		const offsetY = (this._offsetY = (oversizeY - y_css) / 2);
		this._offset = [offsetX, offsetY];

		const stride = this._attrs.asStridedArray(0, 3 * cellX * cellY);
		this._indexBuffer.grow(3 * cellX * cellY);
		this._indexBuffer._activeIndices = 3 * cellX * cellY; // Truncate indices

		const pxSizeX = dpr2 / x_device; // Size of a pixel in horizontal clipspace units
		const pxSizeY = dpr2 / y_device; // Size of a pixel in vertical clipspace units

		// const arrLength = this.#pxPerSlopeUnit;
		const arrLengthThird = this.#pxPerSlopeUnit / 3;
		const arrLengthThirds = (this.#pxPerSlopeUnit * 2) / 3;
		const arrWidth = this.#pxPerSlopeUnit / 8;

		let vtx = 0;
		let idx = 0;
		const maxX = cellX - 1;
		const maxY = cellY - 1;
		for (let i = 0; i < cellX; i++) {
			const posX = (-offsetX + (i + 0.5) * this.cellSize) * pxSizeX - 1;
			const texelX = i / maxX;

			for (let j = 0; j < cellY; j++) {
				const posY = (-offsetY + (j + 0.5) * this.cellSize) * pxSizeY - 1;
				const texelY = j / maxY;

				// Each trig has three vertices; they have the same texel
				// coords, same center-of-cell coords, but different roles
				// prettier-ignore
				stride.set([
					texelX, texelY, posX, posY, -arrLengthThird, +arrWidth,
					texelX, texelY, posX, posY, arrLengthThirds, 0,
					texelX, texelY, posX, posY, -arrLengthThird, -arrWidth,
				], vtx)

				// prettier-ignore
				this._indexBuffer.set(idx, [
					vtx, vtx+1, vtx+2,
				]);

				vtx += 3;
				idx += 3;
			}
		}

		this._attrs.commit(0, vtx);

		this._program.setUniform(
			"uFactor",
			this._factor.map((n) => 1 / n)
		);

		// Acetate.prototype.resize.call(this, x, y);
		this._programs.setUniform("uPixelSize", [pxSizeX, pxSizeY]);
	}

	redraw(crs, matrix, viewportBbox) {
		this._clear.run();

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
