import GleoSymbol from "./Symbol.mjs";

/**
 * @class Tile
 * @inherits GleoSymbol
 *
 * @relationship drawnOn AcetateStitchedTiles, 0..n, 0..1
 *
 * A rectangular, conformal (i.e. matching the display CRS) RGB(A) raster image,
 * part of a bigger grid mosaic.
 *
 * Users should not use `Tile` symbols directly - in most cases, using
 * a `RasterTileLoader` will fulfil most of their use cases.
 */

export default class Tile extends GleoSymbol {
	/**
	 * @section
	 * A `Tile` needs to be passed a 4-point `Geometry` with its bounds, the
	 * name of the pyramid level it's in, its X and Y coordinates within the pyramid level,
	 * and a `HTMLImageElement`
	 *
	 * @constructor Tile(geom: RawGeometry, levelName: String, tileX: Number, tileY: Number)
	 */
	constructor(geom, levelName, tileX, tileY, image) {
		super(geom);

		this.level = levelName;
		this.tileX = tileX;
		this.tileY = tileY;
		this.image = image;

		this.attrLength = 4;
		this.idxLength = 6;
	}
}
