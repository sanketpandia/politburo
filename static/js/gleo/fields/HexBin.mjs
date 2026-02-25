import { ScalarField } from "./Field.mjs";
import QuadMarginBin from "./QuadMarginBin.mjs";
import Acetate from "../acetates/Acetate.mjs";
import HeatMap from "./HeatMap.mjs";

const SQRT3 = 1.73205080756887729353;
const SQRT3HALF = 0.86602540378443864676;
const SQRT3QUART = 0.43301270189221932338;

function fract(n) {
	return n - Math.trunc(n);
}

/**
 * @class HexBin
 * @inherits QuadMarginBin
 *
 * As `QuadMarginBin`, but cells are hexagonal instead of square.
 */

export default class HexBin extends QuadMarginBin {
	/**
	 * @option cellSize: Number = 64
	 * The *diameter* of the hexagonal cells, in CSS pixels.
	 * @option marginSize: Number = 8
	 * The size of the cells' margin, in CSS pixels.
	 * Must be smaller than half the cell size (or else the cell won't be
	 * displayed at all).
	 */

	// Store values for subacetates' uniforms, to prevent late-initialization
	// issues.
	#uHexSize;

	addAcetate(ac) {
		// console.log("Adding acetate to HexBin:", ac.constructor.name);
		// console.log("Adding acetate to HexBin:", ac.constructor.name, ac);

		// Hijack the GL program definition of the subacetate,
		// so that it'll apply an offset to the clipspace coordinates so that
		// the gl_Position falls on the right hexagon

		const fn = ac.glProgramDefinition.bind(ac);

		const search = /gl_Position\s*=\s*([^;]+);/g;
		function replacement(_, captured) {
			return `gl_Position = hexify(${captured});`;
		}

		const hexifyDef = `
		vec4 hexify(vec4 orig) {
			// Each row is (should be) as high as half the radius of the
			// hexagons.
			float row = orig.y / uHexSize.y;

			// Each column is half the width of a hexagon
			float col = orig.x / uHexSize.x;

			// 0-2: solid hex row (odd)
			// 2-3: triangles between hex rows
			// 3-5: solid hex row (even)
			// 5-6: triangles between hex rows
			row = mod(row + 1., 6.0);

			col = mod(col, 2.0);

			float fractRow = fract(row);
			float fractCol = fract(col);

			vec2 offset;

			if ( row < 2. ) {
				// Center of odd row, shift half a cell to the right
				offset.x = uHexSize.x;
			} else if (row < 3.) {
				if (col < 1.0) {
					// Downwards edge on top of odd row
					if (1.0 - fractCol < fractRow) {
						offset.y = uHexSize.y;
					} else {
						offset.y = -uHexSize.y;
					}
				} else {
					// Upwards edge on top of odd row
					if (fractCol < fractRow) {
						offset.y = uHexSize.y;
					} else {
						offset.y = -uHexSize.y;
						offset.x = uHexSize.x;
					}
				}
			} else if (row < 5.) {
				// Center of even rows need no offset.
			} else {
				if (col < 1.0) {
					// Upwards edge on top of even row
					if (fractCol < fractRow) {
						offset.y = uHexSize.y;
					} else {
						offset.y = -uHexSize.y;
					}
				} else {
					// Downwards edge on top of even row
					if (1.0 - fractCol < fractRow) {
						offset.y = uHexSize.y;
						offset.x = uHexSize.x;
					} else {
						offset.y = -uHexSize.y;
					}
				}
			}
			return vec4(orig.xy + offset.xy, orig.zw);
		}`;

		ac.glProgramDefinition = function glProgramDefinition() {
			const opts = fn();

			return {
				...opts,
				uniforms: {
					...opts.uniforms,
					uHexSize: "vec2",
				},
				vertexShaderSource:
					hexifyDef + opts.vertexShaderSource.replace(search, replacement),
				vertexShaderMain: opts.vertexShaderMain.replace(search, replacement),
			};
		};

		super.addAcetate(ac);

		if (this.#uHexSize) {
			ac.once("programlinked", () => {
				ac._programs.setUniform("uHexSize", this.#uHexSize);
			});
		}
	}

	resize(x, y) {
		const hexWidth = this.cellSize * SQRT3HALF;
		const hexHeight = this.cellSize * 0.75;
		const hexRadius = this.cellSize * 0.5;
		const hexHalfRadius = this.cellSize * 0.25;
		const hexHalfWidth = hexWidth / 2;

		// The number of cells (and the dimensions of the low-res framebuffer)
		// will always be even numbers. This simplifies calculations at the
		// cost of drawing more off-screen entities.
		const cellX = 2 + Math.ceil(x / hexWidth / 2) * 2;
		const cellY = 1 + 4 * Math.ceil((y / 2 - hexHalfRadius) / hexHeight / 2);

		// console.log("cell x/y count: ", cellX, cellY);

		const oversizeX = cellX * hexWidth;
		const oversizeY = cellY * hexHeight + hexHalfRadius;
		const offsetX = (x - oversizeX) / 2;
		const offsetY = (y - oversizeY) / 2 + hexRadius;

		HeatMap.prototype.resize.call(this, cellX, cellY);

		const pxSizeX = 2 / x; // Size of a CSS pixel in horizontal clipspace units
		const pxSizeY = 2 / y; // Size of a CSS pixel in vertical clipspace units

		this._factor = [x / oversizeX, y / oversizeY];
		// this._offset = [(oversizeX - x) / 2, (oversizeY - y) / 2];
		this._offset = [offsetX, offsetY];
		this._program.setUniform(
			"uFactor",
			this._factor.map((n) => 1 / n)
		);

		// "Hex size" is really half the width, and a quarter of the height
		// (half the radius). All of that factored so that it fits the expanded
		// bbox coverign the scalar field.
		// This is done to avoid divisions in the shader.
		this.#uHexSize = [
			(hexWidth * pxSizeX * this._factor[0]) / 2,
			hexHalfRadius * pxSizeY * this._factor[1],
		];

		// Set uniforms in the subacetates' programs
		this.subAcetates.forEach((sac) => {
			sac._programs.setUniform("uHexSize", this.#uHexSize);
		});

		// 6 vertices per hexagon
		// 4 triangles (12 indices) per hexagon
		const stride = this._attrs.asStridedArray(0, 6 * cellX * cellY);
		this._indexBuffer.grow(12 * cellX * cellY);
		this._indexBuffer._activeIndices = 12 * cellX * cellY; // Truncate indices

		// Extrusion amount, in CSS pixels:
		const extrPx = hexRadius - this._marginSize;
		// Extrusion amount, in clipspace units:
		const extrX = pxSizeX * extrPx * SQRT3HALF;
		const extrY = pxSizeY * extrPx;
		const extrY2 = extrY / 2;

		let vtx = 0;
		let idx = 0;
		const maxX = cellX - 1;
		const maxY = cellY - 1;
		for (let i = 0; i < cellX; i++) {
			const origPosX = (offsetX + i * hexWidth) * pxSizeX - 1;
			const texelX = i / maxX;

			for (let j = 0; j < cellY; j++) {
				const posX = origPosX + (j % 2 ? hexHalfWidth * pxSizeX : 0);
				const posY = (offsetY + j * hexHeight) * pxSizeY - 1;
				const texelY = j / maxY;

				// The six vertices of a hexagon have the same texel coordinates,
				// the same position, but a different extrusion direction.
				// prettier-ignore
				stride.set([
					posX        , posY + extrY , texelX, texelY,
					posX + extrX, posY + extrY2, texelX, texelY,
					posX + extrX, posY - extrY2, texelX, texelY,
					posX        , posY - extrY , texelX, texelY,
					posX - extrX, posY - extrY2, texelX, texelY,
					posX - extrX, posY + extrY2, texelX, texelY,
				], vtx)

				// prettier-ignore
				this._indexBuffer.set(idx, [
					vtx  , vtx+1, vtx+5,
					vtx+5, vtx+1, vtx+2,
					vtx+5, vtx+2, vtx+4,
					vtx+4, vtx+2, vtx+3,
				]);

				vtx += 6;
				idx += 12;
			}
		}

		this._attrs.commit(0, vtx);

		Acetate.prototype.resize.call(this, x, y);
	}

	getFieldValueAt(x, y) {
		// Same as in the shader: each row is as high as half the radius
		// of the hexagons, and each column is as wide as half an hexagon.
		const col = (x - this._offset[0]) / (this.cellSize * SQRT3QUART);
		const row = (y - this._offset[1]) / (this.cellSize * 0.25);

		const rowMod = (row - 1) % 6;
		const colMod = col % 2;

		let cellX, cellY;

		if (rowMod < 1) {
			/// Space between hexagon rows
			if (colMod < 1) {
				// Upwards edge
				if (fract(col) + fract(row) < 1) {
					cellX = col / 2;
					cellY = (row + 4) / 3;
				} else {
					cellX = (col + 1) / 2;
					cellY = (row + 7) / 3;
				}
			} else {
				// Downwards edge
				if (fract(col) > fract(row)) {
					cellX = (col + 1) / 2;
					cellY = (row + 4) / 3;
				} else {
					cellX = col / 2;
					cellY = (row + 7) / 3;
				}
			}
		} else if (rowMod < 3) {
			cellX = col / 2;
			cellY = (row + 4) / 3;
		} else if (rowMod < 4) {
			/// Space between hexagon rows
			if (colMod < 1) {
				// Downwards edge
				if (fract(col) > fract(row)) {
					cellX = (col + 1) / 2;
					cellY = (row + 4) / 3;
				} else {
					cellX = col / 2;
					cellY = (row + 7) / 3;
				}
			} else {
				// Upwards edge
				if (fract(col) + fract(row) < 1) {
					cellX = col / 2;
					cellY = (row + 4) / 3;
				} else {
					cellX = (col + 1) / 2;
					cellY = (row + 7) / 3;
				}
			}
		} else {
			cellX = (col + 1) / 2;
			cellY = (row + 4) / 3;
		}

		return ScalarField.prototype.getFieldValueAt.call(
			this,
			Math.floor(cellX),
			Math.floor(cellY)
		);
	}
}
