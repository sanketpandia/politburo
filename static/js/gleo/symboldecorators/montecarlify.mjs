import ExtrudedPoint from "../symbols/ExtrudedPoint.mjs";
import earcut from "../3rd-party/earcut/earcut.mjs";

/**
 * @namespace montecarlify
 * @inherits Symbol Decorator
 *
 * Turns a point symbol into a polygon symbol that will draw the original
 * point symbol several times.
 *
 * Instead of taking a point geometry, the symbol will now take a polygon
 * geometry and a `count` option. The original point symbol will
 * be drawn several times (`count`) at random points inside the polygon geometry.
 */

export default function montecarlify(base) {
	if (!base instanceof ExtrudedPoint) {
		throw new Error(
			"The 'montecarlify' symbol decorator can only be applied to extruded points"
		);
	}

	class MonteCarlifiedAcetate extends base.Acetate {
		// Copied from AcetateMonteCarlo.sprinkle,
		// but accounting for several vertices per sprinkled geometry
		sprinkle(montecarlos, baseVtx) {
			const allVtxCoords = montecarlos
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
					const vtxCoords = new Array(mc.attrLength * 2);
					let vtx = 0; // (relative)

					if (areas.length === 0) {
						/// FIXME!!! Fails for polygons that are single triangles??!!?!
						//debugger;
						vtxCoords.fill(NaN);
						return vtxCoords;
					}

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

							for (let v = 0, l = mc.singlePointAttrLength; v < l; v++) {
								vtxCoords[vtx++] = x;
								vtxCoords[vtx++] = y;
							}
						}
						accArea += area;
					});
					return vtxCoords;
				})
				.flat();

			this.multiSetCoords(baseVtx, allVtxCoords);
		}

		// Copied from AcetateMonteCarloFill.reproject.
		// Re-calculates the montecarlo positions (i.e. "sprinkles" the points around)
		// on a CRS change; else manual offset on a CRS offset.
		reproject(start, length) {
			if (this._crs.name !== this._oldCrs.name) {
				this.sprinkle(
					this._knownSymbols.filter((symbol, attrIdx) => {
						return (
							attrIdx >= start &&
							attrIdx + symbol.attrLength <= start + length
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

				coordSlice = coordSlice.map((xy, i) =>
					i % 2 ? xy - offsetY : xy - offsetX
				);

				this.multiSetCoords(start, coordSlice);
			}
		}
	}

	return class MonteCarlifiedSymbol extends base {
		static Acetate = MonteCarlifiedAcetate;

		#count;
		#origAttrLength;
		#origIdxLength;

		constructor(geometry, { count = 1, ...opts } = {}) {
			super(geometry, opts);
			this.#count = count;

			this.#origAttrLength = this.attrLength;
			this.#origIdxLength = this.idxLength;

			this.attrLength *= count;
			this.idxLength *= count;
		}

		_assertGeom() {
			return true;
		}

		get count() {
			return this.#count;
		}

		get singlePointAttrLength() {
			return this.#origAttrLength;
		}

		// Similar trick than the one in tajectorify(): call parent
		// _setGlobalStrides while faking .attrBase and .idxBase
		_setGlobalStrides(...strides) {
			const attrBase = this.attrBase;
			const idxBase = this.idxBase;
			this.attrLength = this.#origAttrLength;
			this.idxLength = this.#origIdxLength;

			for (let i = 0; i < this.#count; i++) {
				super._setGlobalStrides(...strides);

				this.attrBase += this.attrLength;
				this.idxBase += this.idxLength;
			}

			this.attrBase = attrBase;
			this.idxBase = idxBase;
			this.attrLength = this.#origAttrLength * this.#count;
			this.idxLength = this.#origIdxLength * this.#count;
		}
	};
}
