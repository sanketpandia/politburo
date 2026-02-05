import GleoSymbol from "./Symbol.mjs";
import parseColour from "../3rd-party/css-colour-parser.mjs";
import { factory } from "../geometry/DefaultGeometry.mjs";

import Dot from "./Dot.mjs";
import earcut from "../3rd-party/earcut/earcut.mjs";

/**
 * @class AcetateMonteCarlo
 * @inherits AcetateDot
 *
 * Displays `MonteCarloFill` symbols. Performs triangulation of polygons via
 * `earcut`, and creates points inside those polygons in a random uniform
 * manner.
 *
 * Uses the `POINTS` draw mode of WebGL, same as `AcetateDot`.
 */

class AcetateMonteCarlo extends Dot.Acetate {
	constructor(
		glii,
		{
			/**
			 * @section AcetateMonteCarlo Options
			 * @option ditSize: Number = 1
			 * The size of the dots , in GL pixels. The maximum value depends on the
			 * GPU and the WebGL/OpenGL stack.
			 */
			dotSize = 1,
			...opts
		} = {}
	) {
		super(glii, { zIndex: 1500, opts });

		// this._attrs.destroy();
		// delete this._attrs;
		this._colours = new this.glii.SingleAttribute({
			size: 1,
			growFactor: 1.2,
			usage: glii.STATIC_DRAW,
			glslType: "vec4",
			type: Uint8Array,
			normalized: true,
		});

		this.#dotSize = dotSize;
		this.on("programlinked", () => (this.dotSize = this.#dotSize));
	}

	glProgramDefinition() {
		const opts = super.glProgramDefinition();
		return {
			...opts,
			attributes: {
				aColour: this._colours,
				aCoords: opts.attributes.aCoords,
				// ...opts.attributes
			},
			uniforms: {
				uDotSize: "float",
				...opts.uniforms,
			},
			vertexShaderMain: `
				vColour = aColour;
				gl_Position = vec4(vec3(aCoords, 1.0) * uTransformMatrix, 1.0);
				gl_PointSize = uDotSize;
			`,
			fragmentShaderMain: `gl_FragColor = vColour;`,
		};
	}

	// Pretty much copied from AcetateFill.multiAdd.
	multiAdd(montecarlos) {
		// Skip already added symbols
		montecarlos = montecarlos.filter((m) => !m._inAcetate);
		if (montecarlos.length === 0) {
			return;
		}

		const count = montecarlos.reduce((acc, mc) => acc + mc.attrLength, 0);
		const base = this._indices.allocateSlots(count);
		let vtxAcc = base;

		montecarlos.forEach((mc) => {
			mc.updateRefs(this, vtxAcc, undefined);
			this._knownSymbols[vtxAcc] = mc;
			vtxAcc += mc.attrLength;
		});

		this._colours.multiSet(
			base,
			montecarlos.map((mc, i) => new Array(mc.attrLength).fill(mc.colour)).flat(2)
		);

		if (this.crs) {
			this.sprinkle(montecarlos);
		} else {
			// Fake coordinates with zeroes, just to grow the attribute storage.
			this.multiSetCoords(
				base,
				new Array(
					montecarlos.map((mc) => mc.attrLength).reduce((a, b) => a + b) * 2
				)
			);
		}

		// Call `multiAdd` of `Acetate` grandparent class, skipping the
		// implementation of `AcetateDot` parent class
		//return super.super.multiAdd(montecarlos, base);
		return Object.getPrototypeOf(
			Object.getPrototypeOf(Object.getPrototypeOf(this))
		).multiAdd.call(this, montecarlos);
	}

	sprinkle(montecarlos, baseVtx) {
		const allDotCoords = montecarlos
			.map((mc) => {
				const d = mc.geom.dimension;
				const stops = [...mc.geom.hulls, mc.geom.coords.length / d];
				let start = 0;
				const coords = mc.geom.toCRS(this._crs).coords;

				const trigs = stops
					.map((stop) => {
						// Get the ring offsets ("hole positions") for the current hull
						const rings = mc.geom.rings
							.filter((r) => r > start && r < stop)
							.map((r) => r - start);

						const trigs = earcut(
							coords.slice(start * d, stop * d),
							rings,
							d
						).map((t) => t + start);
						start = stop;
						return trigs;
					})
					.flat();

				/// Calculate triangle areas
				let areas = new Array(trigs.length / 3);
				let totalArea = 0;

				for (let i = 0, l = trigs.length; i < l; i += 3) {
					const x1 = coords[trigs[i] * d];
					const y1 = coords[trigs[i] * d + 1];
					const x2 = coords[trigs[i + 1] * d];
					const y2 = coords[trigs[i + 1] * d + 1];
					const x3 = coords[trigs[i + 2] * d];
					const y3 = coords[trigs[i + 2] * d + 1];

					// Calculate area as per https://en.wikipedia.org/wiki/Triangle#Using_coordinates

					const area =
						0.5 * Math.abs((x1 - x3) * (y2 - y1) - (x1 - x2) * (y3 - y1));

					areas[i / 3] = area;
					totalArea += area;
				}

				// Turn areas into number of dots into each area (by means of
				// percentages relative to the polygon's total area, multiplied by
				// number of points)
				areas = areas.map((a) => (mc.count * a) / totalArea);

				let accArea = 0;
				const dotCoords = new Array(mc.count * 2);
				// Loop through triangles. Each triangle has a known number of dots
				// to sprinkle.
				const maxJ = areas.length - 1;
				areas.forEach((area, j) => {
					const top =
						j === maxJ
							? mc.count // Avoid floating-point rounding errors
							: area + accArea;

					const j3 = j * 3;
					const x1 = coords[trigs[j3] * d];
					const y1 = coords[trigs[j3] * d + 1];
					const x2 = coords[trigs[j3 + 1] * d];
					const y2 = coords[trigs[j3 + 1] * d + 1];
					const x3 = coords[trigs[j3 + 2] * d];
					const y3 = coords[trigs[j3 + 2] * d + 1];

					const d1x = x2 - x1;
					const d1y = y2 - y1;
					const d2x = x3 - x1;
					const d2y = y3 - y1;

					const i1 = Math.floor(accArea) * 2;
					const i2 = Math.floor(top) * 2;

					// console.log("Sprinkling Montecarlo triangle", trigs[j3], trigs[j3+1], trigs[j3+2], "coords", x1,y1,x2,y2,x3,y3, "count", (i2 - i1) / 2);

					for (let i = i1; i < i2; i += 2) {
						let r1 = Math.random();
						let r2 = Math.random();

						if (r1 + r2 > 1) {
							r1 = 1 - r1;
							r2 = 1 - r2;
						}

						const x = x1 + d1x * r1 + d2x * r2;
						const y = y1 + d1y * r1 + d2y * r2;

						dotCoords[i] = x;
						dotCoords[i + 1] = y;
					}
					accArea += area;
				});

				return dotCoords;
			})
			.flat();

		this.multiSetCoords(baseVtx, allDotCoords);
	}

	/**
	 * @method reproject(start: Number, length: Number): Array of Number
	 * As `AcetateVertices.reproject()`, but also recalculates the random
	 * points positions for the affected symbols
	 */
	reproject(start, length) {
		if (this._crs.name !== this._oldCrs.name) {
			this.sprinkle(
				this._knownSymbols.filter((symbol, attrIdx) => {
					return (
						attrIdx >= start && attrIdx + symbol.attrLength <= start + length
					);
				}),
				start
			);
		} else {
			// Manual offsetting of points, as per AcetateArrugatedRaster

			const fromOffset = this._oldCrs?.offset ?? [0, 0];
			const toOffset = this._crs?.offset ?? [0, 0];
			const offsetX = toOffset[0] - fromOffset[0];
			const offsetY = toOffset[1] - fromOffset[1];

			let coordSlice = new Float32Array(
				this._coords._byteData.buffer,
				start * 8, // Each item is 2 4-byte floats, so 8 bytes per.
				length * 2
			);

			coordSlice = coordSlice.map((xy, i) => (i % 2 ? xy - offsetY : xy - offsetX));

			this.multiSetCoords(start, coordSlice);
		}
	}

	/**
	 * @property dotSize
	 * Size of each dot. Each dot will be a square, each side measuring this
	 * many GL pixels. Can be updated.
	 */
	#dotSize;
	get dotSize() {
		return this.#dotSize;
	}
	set dotSize(s) {
		this._program?.setUniform("uDotSize", s);
		this.#dotSize = s;
		this.dirty = true;
	}
}

/**
 * @class MonteCarloFill
 * @inherits GleoSymbol
 * @relationship drawnOn AcetateMonteCarlo
 *
 * Spawns 1-pixel dots randomly sprinkled through the area of the given polygon
 * `Geometry`. Useful for representing area-relative densities.
 *
 * Otherwise, it's similar to `Fill`.
 *
 */

export default class MonteCarloFill extends GleoSymbol {
	/// @section Static properties
	/// @property Acetate: Prototype of AcetateMonteCarlo
	// The `Acetate` class that draws this symbol.
	static Acetate = AcetateMonteCarlo;

	/**
	 * @constructor MonteCarloFill(geom: Geometry, opts?: MonteCarloFill Options)
	 */
	constructor(
		geom,
		{
			/**
			 * @section
			 * @aka MonteCarloFill Options
			 * @option colour: Colour = '#3388ff33'
			 * The colour of the fill symbol.
			 */
			colour = [0x33, 0x88, 0xff, 0x33],

			/**
			 * @option count: Number = 0
			 * The number of dots to spawn
			 */
			count = 0,
			...opts
		} = {}
	) {
		// Length of each linestring
		//this._lengths = linestrings.map((ls) => ls.length);
		super(geom, opts);

		// Amount of vertex attribute slots needed. One per dot.
		this.attrLength = count;

		// Amount of index slots needed (calc'd by earcut)
		//this.idxLength = (this.attrLength - this._lengths.length) * 2;

		this.#colour = this.constructor._parseColour(colour);

		this.count = count;
	}

	#colour;
	/**
	 * @property colour: Colour
	 * The colour for all the dots. Can be updated.
	 */
	get colour() {
		return this.#colour;
	}
	set colour(newColour) {
		this.#colour = parseColour(newColour);
		if (!this._inAcetate) {
			return this;
		}
		this._inAcetate._colours.multiSet(
			this.attrBase,
			new Array(this.attrLength).fill(this.#colour).flat()
		);
		this._inAcetate.dirty = true;
	}

	setGeometry(geom) {
		const geometry = factory(geom);
		const ac = this._inAcetate;
		if (ac) {
			ac.remove(this);
			this.geom = factory(geometry);
			this.attrLength = count;
			ac.add(this);
		} else {
			this.attrLength = count;
			this.geom = factory(geometry);
		}
		return this;
	}

	// Can be overriden by subclasses or the `intensify` decorator
	static _parseColour = parseColour;
}
