import QuadBin from "./QuadBin.mjs";
import parseColour from "../3rd-party/css-colour-parser.mjs";
import glslFloatify from "../util/glslFloatify.mjs";
import glslVecNify from "../util/glslVecNify.mjs";

/**
 * @class QuadMarginBin
 * @inherits QuadBin
 *
 * As `QuadBin`, but each cell is displayed with a transparent margin.
 *
 */

export default class QuadMarginBin extends QuadBin {
	/**
	 * @constructor QuadMarginBin(target: GliiFactory, opts?: QuadMarginBin Options)
	 */
	constructor(
		target,
		{
			/**
			 * @option marginSize: Number = 8
			 * The size of the cells' margin, in CSS pixels.
			 * Must be smaller than half the cell size (or else the cell won't be
			 * displayed at all).
			 */
			marginSize = 16,
			...opts
		} = {}
	) {
		super(target, opts);

		this._attrs = new this.glii.InterleavedAttributes(
			{
				usage: this.glii.STATIC_DRAW,
				size: 4,
				growFactor: 1,
			},
			[
				{
					// Vertex position
					glslType: "vec2",
					type: Float32Array,
					normalized: false,
				},
				{
					// Texel coords
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

		this._marginSize = marginSize;
	}

	glProgramDefinition() {
		const opts = super.glProgramDefinition();

		// Unlike HeatMap, the colour ramp will be applied in the vertex
		// shader instead of the fragment shader.

		let intensities = [];
		let colours = [];
		Object.entries(this.stops)
			.map(([intensity, colour]) => [Number(intensity), parseColour(colour)])
			.sort(([a, _], [b, __]) => a - b)
			.forEach(([intensity, colour]) => {
				intensities.push(intensity);
				colours.push(colour);
			});

		const stopCount = colours.length;
		const intensitiesInit = intensities
			.map((i, j) => `intensities[${j}] = float(${glslFloatify(i)});`)
			.join("\n");
		const coloursInit = colours
			.map((c, j) => `colours[${j}] = ${glslVecNify(c.map((b) => b / 255))};`)
			.join("\n");

		return {
			...opts,
			indexBuffer: this._indexBuffer,
			attributes: {
				aPos: this._attrs.getBindableAttribute(0),
				aUV: this._attrs.getBindableAttribute(1),
			},
			// uniforms: {},
			vertexShaderMain: `
				float intensities[${stopCount}];
				vec4 colours[${stopCount}];
				${intensitiesInit}
				${coloursInit}

				gl_Position = vec4(aPos, 0., 1.);

				float value = texture2D(uField, aUV).x;

				vColour = colours[0];

				for (int i=1; i< ${stopCount}; i++) {
					vColour = mix(
						vColour,
						colours[i],
						smoothstep(intensities[i-1], intensities[i], value)
					);
				}
			`,
			varyings: {
				vColour: "vec4",
			},
			fragmentShaderMain: `
				gl_FragColor = vColour;
				// gl_FragColor = vec4(0., 0., 0., 1.);
			`,
		};
	}

	resize(x_device, y_device, x_css, y_css) {
		super.resize(x_device, y_device, x_css, y_css);

		// The data is not shown as a single quad (as the parent classes do).
		// Instead, this will calculate one quad per texel of the scalar field,
		// and fill up extrusion and colour attributes for those quads.
		// Note that the quads do not cover the entire area of a scalar field
		// texel, since there's a margin.
		const cellX = Math.ceil(x_css / this.cellSize);
		const cellY = Math.ceil(y_css / this.cellSize);
		const oversizeX = cellX * this.cellSize;
		const oversizeY = cellY * this.cellSize;
		const offsetX = (oversizeX - x_css) / 2;
		const offsetY = (oversizeY - y_css) / 2;

		const stride = this._attrs.asStridedArray(0, 4 * cellX * cellY);
		this._indexBuffer.grow(6 * cellX * cellY);
		this._indexBuffer._activeIndices = 6 * cellX * cellY; // Truncate indices

		const pxSizeX = 2 / x_css; // Size of a CSS pixel in horizontal clipspace units
		const pxSizeY = 2 / y_css; // Size of a CSS pixel in vertical clipspace units

		const extrPx = this.cellSize / 2 - this._marginSize;
		const extrX = pxSizeX * extrPx; // Extrusion amount, in clipspace units
		const extrY = pxSizeY * extrPx;

		let vtx = 0;
		let idx = 0;
		const maxX = cellX - 1;
		const maxY = cellY - 1;
		for (let i = 0; i < cellX; i++) {
			const posX = (offsetX + i * this.cellSize) * pxSizeX - 1;
			const texelX = i / maxX;

			for (let j = 0; j < cellY; j++) {
				const posY = (offsetY + j * this.cellSize) * pxSizeY - 1;
				const texelY = j / maxY;

				// The four vertices of a quad have the same texel coordinates,
				// the same position, but a different extrusion direction.
				// prettier-ignore
				stride.set([
					posX + extrX, posY + extrY, texelX, texelY,
					posX + extrX, posY - extrY, texelX, texelY,
					posX - extrX, posY - extrY, texelX, texelY,
					posX - extrX, posY + extrY, texelX, texelY,
				], vtx)

				// prettier-ignore
				this._indexBuffer.set(idx, [
					vtx, vtx+1, vtx+2,
					vtx, vtx+3, vtx+2,
				]);

				vtx += 4;
				idx += 6;
			}
		}

		this._attrs.commit(0, vtx);
		// console.log(stride);
	}
}
