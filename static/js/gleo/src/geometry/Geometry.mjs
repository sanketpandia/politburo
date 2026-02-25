import RawGeometry from "./RawGeometry.mjs";
import { getCRS } from "../crs/knownCRSs.mjs";
import BaseCRS from "../crs/BaseCRS.mjs";

/**
 * @class Geometry
 * @inherits RawGeometry
 *
 * A Gleo `Geometry` is akin to geometries in the OGC Simple Features Specification:
 * points, linestrings, polygons and multipolygons.
 *
 * Internally, `Geometry`s are represented as a flat array of coordinates, in the form
 * `[x1,y2, x2,y2, x3,y3 ... xn,yn]` for 2-dimensional geometries; plus a list of
 * offsets specifying the n-th coordinate where a hull starts and a ring starts (0th
 * hulls and rings are ommitted).
 *
 * (About nomenclature: a multipolygon has one or more hulls, and each hull
 * has an outer ring and zero or more inner rings. Hulls tell apart polygons
 * within a multipolygon, and rings tell apart inner/outer boundaries of a
 * polygon).
 *
 * (TODO: [x1,y1,z1, ... xn,yn,zn] for 3-dimensional, and [x1,y1,z1,m1, ... xn,yn,zn,mn]
 * for 4-dimensional)
 *
 * @example
 *
 * ```
 * let point = new Geometry(crs, [x,y]);
 *
 * let linestring = new Geometry(crs, [[x1,y1],[x2,y2]]);
 * ```
 *
 **/

/*

Depths:

0  Point
1  Multipoint Linestring
2             Multilinestring Polygon
3                             Multipolygon

 */

export default class Geometry extends RawGeometry {
	/**
	 * @section
	 * The constructor for a `Geometry` can take either a `BaseCRS` instance, or
	 * its name.
	 *
	 * The geometry can be:
	 * - An `Array` of two `Number`s (for points)
	 * - An `Array` of `Array`s of two `Number`s (for multipoints or linestrings)
	 * - An `Array` of `Array`s of `Array`s of two `Number`s (for multilinestrings or polygons)
	 * - An `Array` of `Array`s of `Array`s of `Array`s of two `Number`s (for multipolygons)
	 *
	 * @constructor Geometry(crs: BaseCRS, coords: Array of Number, opts: Geometry Options)
	 * @alternative
	 * @constructor Geometry(crs: String, coords: Array of Number, opts: Geometry Options)
	 * @alternative
	 * @constructor Geometry(crs: BaseCRS, coords: Array of Array of Number, opts: Geometry Options)
	 * @alternative
	 * @constructor Geometry(crs: BaseCRS, coords: Array of Array of Array of Number, opts: Geometry Options)
	 * @alternative
	 * @constructor Geometry(crs: BaseCRS, coords: Array of Array of Array of Array of Number, opts: Geometry Options)
	 */
	constructor(
		crs,
		coords,
		{
			wrap,
			dimension = 2,
			/**
			 * @section Geometry Options
			 * @option deduplicate: Boolean = true
			 * Whether to detect and remove duplicated consecutive coordinates. Prevents
			 * graphical artefacts on some edge cases of topologically malformed data.
			 */
			deduplicate = true,
		} = {}
	) {
		let rings = [];
		let hulls = [];
		let depth;
		if (typeof coords[0] === "number") {
			depth = 0;
		} else {
			if (deduplicate) {
				coords = deduplicateConsecutives(coords);
			}

			if (typeof coords[0][0] === "number") {
				depth = 1;
			} else if (typeof coords[0][0][0] === "number") {
				depth = 2;
				// Calculate rings
				let ringOffset = 0;
				for (let r = 0, l = coords.length - 1; r < l; r++) {
					rings.push((ringOffset += coords[r].length));
				}
			} else if (typeof coords[0][0][0][0] === "number") {
				depth = 3;
				// hulls and rings
				let hullOffset = 0;
				let ringOffset = 0;
				for (let h = 0, l = coords.length; h < l; h++) {
					let hullSize = 0;
					for (let r = 0, ll = coords[h].length; r < ll; r++) {
						const ringLength = coords[h][r].length;
						ringOffset += ringLength;
						if (r !== ll - 1) {
							rings.push(ringOffset);
						}
						hullSize += ringLength;
					}

					if (h !== l - 1) {
						hulls.push((hullOffset += hullSize));
					}
				}
			} else {
				throw new Error(
					"Coordinate array passed to Geometry constructor has too many levels of array nesting."
				);
			}
		}

		// Assert dimension
		if (depth) {
			if (!coords.flat(depth - 1).every((v) => v.length === dimension)) {
				throw new Error(
					`While instancing Geometry, expected all coordinates to be of dimension ${dimension}`
				);
			}
		} else {
			if (coords.length !== dimension) {
				throw new Error(
					`While instancing point Geometry, expected all coordinates to be of dimension ${dimension}`
				);
			}
		}

		if (!(crs instanceof BaseCRS)) {
			crs = getCRS(crs);
		}

		super(crs, coords.flat(depth), rings, hulls, { wrap, dimension });
	}
}

function deduplicateConsecutives(coords) {
	if (typeof coords[0][0] !== "number") {
		return coords.map(deduplicateConsecutives);
	}

	const l = coords.length - 1;
	return coords.filter(
		(c, i) => i === l || c[0] !== coords[i + 1][0] || c[1] !== coords[i + 1][1]
	);
}
