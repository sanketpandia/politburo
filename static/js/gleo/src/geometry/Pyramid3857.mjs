import TilePyramid from "./TilePyramid.mjs";
import epsg3857 from "../crs/epsg3857.mjs";

/**
 * @namespace Pyramid3857
 * @relationship associated epsg3857
 * @relationship associated TilePyramid
 *
 * Factory for tile pyramids fitting the EPSG:3857 CRS (`epsg3857`).
 * Assumes pyramid level 0 spans the whole surface.
 *
 * @example
 * ```
 * import { create3857Pyramid } from 'gleo/src/geometry/Pyramid3857';
 *
 * const myPyramid = create3857Pyramid(0, 18);
 * ```
 */

const limit = 20037508.34;
const scale0 = limit * 2;
const bbox = [-limit, limit, limit, -limit]; // x1, y1, x2, y2

/**
 * @function create3857Pyramid(min: Number, max: Number, tileSize: Number = 256): TilePyramid
 *
 * Creates a `TilePyramid` for the `epsg3857` crs, with one pyramid level for
 * each "zoom level" between the given minimum and maximum.
 *
 * The pyramid constructor also takes the (square) size of each tile, **in
 * CSS pixels**, in order to calculate the right zoom level to load on any
 * given scale. Note that the pyramid takes in CSS pixels, but the `RasterTileLoader`
 * takes source raster pixels instead.
 */
export default function create3857Pyramid(min, max, tileSize = 256) {
	const pyramid = {};
	if (max < min || !isFinite(min) || !isFinite(max)) {
		throw new Error("Invalid min/max levels for mercator tile pyramid");
	}
	for (let i = min; i <= max; i++) {
		const j = 1 << i;
		pyramid[i] = {
			scale: scale0 / j / tileSize,
			bbox: bbox,
			spanX: j,
			spanY: j,
		};
	}

	return new TilePyramid(epsg3857, pyramid);
}
