import RasterTileLoader from "./RasterTileLoader.mjs";
import abortableImagePromise from "../util/abortableImagePromise.mjs";
import create3857Pyramid from "../geometry/Pyramid3857.mjs";

import template from "../util/templateStr.mjs";

/**
 * @class MercatorTiles
 * @inherits RasterTileLoader
 * @relationship compositionOf epsg3857, 0..n, 1..1
 *
 * Convenience wrapper for `RasterTileLoader`. Loads tilesets in the de-facto
 * standard for Web Mercator tiles.
 *
 * This aims to expose a minimalistic Leaflet-like API, instead of needing to use
 * a configurable `TilePyramid` like `TileLoader` does.
 *
 * @example
 *
 * ```js
 * new MercatorTiles("https://tile.osm.org/{z}/{y}/{x}.png", {
 * 	maxZoom: 10,
 * 	attribution: "<a href='http://osm.org/copyright'>© OpenStreetMap contributors</a>",
 * }).addTo(myGleoMap);
 * ```
 */
export default class MercatorTiles extends RasterTileLoader {
	/**
	 * @constructor MercatorTiles(templateStr: String, options: MercatorTiles Options)
	 */
	constructor(templateStr, options = {}) {
		/**
		 * @section
		 * @aka MercatorTiles Options
		 * @option minZoom: Number = 0
		 * The minimum zoom level for tiles to be loaded.
		 * @option maxZoom: Number = 18
		 * The maximum zoom level for tiles to be loaded.
		 * @option tileSize: Number = 256
		 * The size of the tiles, **in CSS pixels**.
		 */
		const pyramid = create3857Pyramid(
			options.minZoom || 0,
			options.maxZoom || 18,
			options.tileSize || 256
		);

		function fetchImage(z, x, y, controller) {
			return abortableImagePromise(
				template(templateStr, { x, y, z, ...options }),
				controller
			);
		}

		super(pyramid, fetchImage, options);
	}
}
