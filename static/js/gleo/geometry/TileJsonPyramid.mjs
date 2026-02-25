import TilePyramid from "./TilePyramid.mjs";

/// TODO: Consider dynamic imports for the CRSs
// import epsg3857 from "../crs/epsg3857.mjs";
// import epsg4326 from "../crs/epsg3857.mjs";
import create3857Pyramid from "./Pyramid3857.mjs";

/**
 * @namespace Pyramid3857
 *
 * @example
 * ```
 * import { tileJsonToPyramid } from 'gleo/src/geometry/TileJsonPyramid.mjs';
 *
 * const myPyramid = tileJsonToPyramid("url/to/tile.json");
 * ```
 */

/**
 * @function tileJsonToPyramid(json: Object, tileSize:Number = 256): Promise to Object
 * Converts a [TileJSON](https://docs.mapbox.com/help/glossary/tilejson/)
 * document into a `TilePyramid` instance and a set of URL strings.
 *
 * All tiles are supposed to be square and have the given pixel size.
 *
 * @alternative
 * @function tileJsonToPyramid(url: String, tileSize:Number = 256): Promise to TilePyramid
 * `fetch`es the URL expecting a JSON document, parses it as
 * `TileJSON` and returns the corresponding `TilePyramid` and URL(s)
 *
 * @alternative
 * @function tileJsonToPyramid(url: URL, tileSize:Number = 256): Promise to TilePyramid
 * Idem, but with an instance of `URL` instead of a `String`.
 */

/// TODO: Handle more TileJSON fields
// - scheme=xyz/tms : invert bbox
// - bounds : EPSG:4326 bbox

export default async function tileJsonToPyramid(tilejson, tileSize = 256) {
	let baseURL = document.url;
	if (typeof tilejson === "string") {
		tilejson = new URL(tilejson, document.url);
	}

	if (tilejson instanceof URL) {
		// Set base URL - strip trailing slash (if any) then plot a trailing slash.
		baseURL = tilejson.toString().replace(/\/?$/, "/");
		tilejson = await fetch(tilejson).then((res) => res.json());
	}

	if (!tilejson.tilejson) {
		console.warn("Non-compliant TileJSON!");
	}

	let min = tilejson.minzoom || 0;
	let max = tilejson.maxzoom || 22;
	let pyramid;

	if (max < min || !isFinite(min) || !isFinite(max)) {
		throw new Error("Invalid min/max zoom levels in tileJSON");
	}

	switch (tilejson.crs) {
		case "EPSG:3857":
		case undefined:
			pyramid = create3857Pyramid(min, max, tileSize);
			break;
		case "EPSG:4326":
			scale0 = 180;
			bbox = [-180, 90, 180, -90]; // x1, y1, x2, y2

			let levels = {};

			for (let i = min; i <= max; i++) {
				const j = 1 << i;
				levels[i] = {
					scale: scale0 / j / tileSize,
					bbox: bbox,
					spanX: j * 2,
					spanY: j,
				};
			}
			pyramid = new TilePyramid(epsg4326, levels);
			break;
		default:
			throw new Error("Unknown/unsupported CRS in TileJSON");
	}

	const tiles = tilejson.tiles.map((t) => decodeURI(new URL(t, baseURL)));

	// 	if (tilejson.scale) {
	// 		// Not part of the TileJSON standard, but used by MapTiler to provide
	// 		// 512px raster tiles ????
	// 		tileSize *= Number(tilejson.scale);
	// 	}

	return { pyramid, tiles };
}
