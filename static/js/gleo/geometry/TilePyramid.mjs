/**
 * Meaning is as per the OGC WMTS
 * spec (portal.opengeospatial.org/files/?artifact_id=35326):
 *
 * 4.13
 * tile matrix set
 * a collection of tile matrices defined at different scales
 *
 * 4.12
 * tile matrix
 * a collection of tiles for a fixed scale
 *
 */

/**
 * @class TilePyramid
 *
 * A `TilePyramid` defines the size and disposition of raster tiles.
 *
 * The concept is equivalent to the "Tile Matrix Set" in the
 * [OGC WMTS specification](portal.opengeospatial.org/files/?artifact_id=35326),
 * and also equivalent to OpenLayer's `TileGrid`s.
 *
 * Tile pyramids do not have a concept of "tile size" but rather per-level "scale".
 * A level will be loaded when the map's (platina's) scale is equal or breater
 * than the level's.
 *
 * @example
 * ```
 * const epsg3857zoom0 = new TilePyramid(
 * 	epsg3857,
 * 	{
 * 		"0": {	// (string) identifier of the pyramid level
 * 			scale: 156543,03390625	// scale, in CRS units per CSS pixel
 * 			bbox: [-l, l, l, -l],	// x1,y1, x2,y2
 * 			spanX: 1,	// horizontal tiles in the level
 * 			spanY: 1,	// vertical tiles in the level
 * 		}
 * 	}
 * );
 * ```
 */

export default class TilePyramid {
	#crs;
	#scales; // Ordered array of scale factors
	#levels;
	#ids; // Map of scale factor to level name
	#orderedIds; // Ids ordered by scale

	/**
	 * @constructor TilePyramid(crs: BaseCRS, levels: Object of Object)
	 * Defines a new `TilePyramid`, given its CRS and a set of pyramid levels,
	 * indexed by the scale of each.
	 *
	 * Scales are Gleo scales: CRS units per CSS pixel.
	 */
	constructor(crs, levels) {
		this.#crs = crs;
		this.#levels = levels;
		this.#ids = Object.fromEntries(
			Object.entries(levels).map(([id, { scale }]) => [String(scale), id])
		);
		// .map(Object.fromEntries);

		// NOTE: ordering is ascending, so first item is the level
		// with the smallest scale - which means the highest zoom.
		this.#scales = Object.values(levels)
			.map(({ scale }) => Number(scale))
			.sort((a, b) => a - b);

		this.#orderedIds = this.#scales.map((scale) => this.#ids[scale]);
	}

	/**
	 * @method ceilLevel(scale: Number): String
	 * Given a scale (in terms of CRS units per CSS pixel), returns the
	 * identifier of the pyramid level with the nearest known scale in
	 * the pyramid that is *equal or higher* than the given one.
	 *
	 * Returns `undefined` if there's no known equal-or-higher scale.
	 */
	ceilLevel(scale) {
		// Yes, a bisect search would be slightly more efficient, I know.
		for (let l = this.#scales.length, i = l; i > 0; i--) {
			if (this.#scales[i] >= scale) {
				return this.#ids[this.#scales[i]];
			}
		}
		return undefined;
	}

	/**
	 * @method floorLevel(scale: Number): String
	 * Given a scale (in terms of CRS units per CSS pixel), returns the
	 * identifier of the pyramid level with the nearest known scale in
	 * the pyramid that is *equal or higher* than the given one.
	 *
	 * Returns `undefined` if there's no known equal-or-lower scale.
	 */
	floorLevel(scale) {
		for (let l = this.#scales.length, i = 0; i < l; i++) {
			if (this.#scales[i] <= scale) {
				return this.#ids[this.#scales[i]];
			}
		}
		return undefined;
	}

	/**
	 * @method nearestLevel(scale:Number): String
	 * Given a scale, returns the identifier of the level with the *nearest*
	 * scale known in the pyramid levels,
	 * "nearest" in terms of "minimum distance in terms of base-2 logarithm"
	 */
	nearestLevel(scale) {
		const l = this.#scales.length - 1;

		if (scale <= this.#scales[0]) {
			// Smaller than the smallest available scale
			return this.#ids[this.#scales[0]];
		}
		if (scale >= this.#scales[l]) {
			// Bigger than the biggest available scale
			return this.#ids[this.#scales[l]];
		}

		for (let i = 0; i < l; i++) {
			const lower = this.#scales[i];
			const upper = this.#scales[i + 1];
			if (lower < scale && scale <= upper) {
				const log2scale = Math.log2(scale);
				const log2upper = Math.log2(upper);
				const log2lower = Math.log2(lower);

				if (Math.abs(log2upper - log2scale) < Math.abs(log2lower - log2scale)) {
					return this.#ids[upper];
				} else {
					return this.#ids[lower];
				}
			}
		}
	}

	/**
	 * @method bboxToTileRange(levelId: String, bbox: Array of Number): Array of Number
	 * Given a bounding box of the form `[x1,y1, x2,y2]` and the string
	 * identifier for a level of the pyramid, returns a bounding box
	 * containing the integer min/max tile coordinates that *overlap* the given
	 * bbox.
	 *
	 * The return values can be higher than the span. This is a safeguard against
	 * misbehaviour and negative coordinates when requesting tiles across the
	 * antimeridian. When looping through this values, modulo by the tile span.
	 *
	 * The bbox is expected to be in the same CRS as the tile pyramid.
	 */
	bboxToTileRange(levelId, [x1, y1, x2, y2]) {
		const level = this.#levels[levelId];

		if (!level) {
			throw new Error(`Level identifier ${levelId} does not exist in the pyramid.`);
		}

		const [minx, miny, maxx, maxy] = level.bbox;
		const w = maxx - minx;
		const h = maxy - miny;

		let tx1 = (level.spanX * (x1 - minx)) / w;
		let ty1 = (level.spanY * (y1 - miny)) / h;
		let tx2 = (level.spanX * (x2 - minx)) / w;
		let ty2 = (level.spanY * (y2 - miny)) / h;

		// console.log(tx1, ty1, tx2, ty2);

		const range = [
			Math.floor(Math.min(tx1, tx2)),
			Math.floor(Math.min(ty1, ty2)),
			Math.ceil(Math.max(tx1, tx2)),
			Math.ceil(Math.max(ty1, ty2)),
		];

		if (range[2] - range[0] > level.spanX) {
			range[0] = 0;
			range[2] = level.spanX;
		}
		if (range[3] - range[1] > level.spanY) {
			range[1] = 0;
			range[3] = level.spanY;
		}

		if (range[0] < 0) {
			if (this.#crs.wrapPeriodX !== Infinity) {
				const i = Math.ceil(-range[0] / level.spanX);
				range[0] += level.spanX * i;
				range[2] += level.spanX * i;
			} else {
				range[0] = 0;
				range[2] = Math.min(range[2], level.spanX);
			}
		}
		if (range[1] < 0) {
			if (this.#crs.wrapPeriodY !== Infinity) {
				const i = Math.ceil(-range[1] / level.spanY);
				range[1] += level.spanX * i;
				range[3] += level.spanX * i;
			} else {
				range[1] = 0;
				range[3] = Math.min(range[3], level.spanY);
			}
		}

		return range;
	}

	/**
	 * @method tileRangeToBbox(levelId: String, range: Array of Number): Array of Number
	 *
	 * Given the identifier of a pyramid level and a tile range of the form
	 * `[minX, minY, maxX, maxY]`, returns the bounding box (in the pyramid's
	 * CRS) that encloses the tiles within the given range.
	 *
	 */
	tileRangeToBbox(levelId, [tx1, ty1, tx2, ty2]) {
		const level = this.#levels[levelId];

		if (!level) {
			throw new Error(`Level identifier ${levelId} does not exist in the pyramid.`);
		}

		const [minx, miny, maxx, maxy] = level.bbox;
		const w = maxx - minx;
		const h = maxy - miny;

		const x1 = (tx1 / level.spanX) * w + minx;
		const y1 = (ty1 / level.spanY) * h + miny;
		const x2 = ((tx2 + 1) / level.spanX) * w + minx;
		const y2 = ((ty2 + 1) / level.spanY) * h + miny;

		return [x1, y1, x2, y2];
	}

	/**
	 * @method tileCoordsToBbox(levelId: String, coords: Array of Number): Array of Number
	 *
	 * Given the coordinates of a tile (the identifier of a level plus an array
	 * of the form `[x,y]`), returns a bounding box (of the form `[x1,y1, x2,y2]`)
	 * with the boinding box for that tile (in the pyramid's CRS).
	 */
	tileCoordsToBbox(levelId, [x, y]) {
		return this.tileRangeToBbox(levelId, [x, y, x, y]);
	}

	/**
	 * @method childTiles(levelId: String, x: Number, y: Number): Array of Array of Number
	 * For a tile of the given level and coordinates, returns an array of
	 * tile coordinates for the child tiles: tiles from a lower level (one with
	 * more detail) whose bounding box overlap that of the given tile.
	 * This will be an empty array if there are no child tiles.
	 */
	childTiles(levelId, x, y) {
		return this.#familyTiles(levelId, x, y, -1);
	}

	/**
	 * @method parentTiles(levelId: String, x: Number, y: Number): Array of Array of Number
	 * Akin to `childTiles()`, but for the parent tiles: tiles from a higher
	 * level (one with less detail).
	 * This will be an empty array if there are no parent tiles.
	 */
	parentTiles(levelId, x, y) {
		return this.#familyTiles(levelId, x, y, +1);
	}

	// Functionality common to both childTiles() and parentTiles()
	#familyTiles(levelId, x, y, levelOffset) {
		const levelIdx = this.#scales.indexOf(this.#levels[levelId].scale);
		const children = [];
		const nextLevelId = this.#orderedIds[levelIdx + levelOffset];
		if (!nextLevelId) {
			return children;
		}
		const bbox = this.tileCoordsToBbox(levelId, [x, y]);
		const [minX, minY, maxX, maxY] = this.bboxToTileRange(nextLevelId, bbox);
		const { spanX, spanY } = this.#levels[nextLevelId];

		for (let x = minX; x < maxX; x++) {
			for (let y = minY; y < maxY; y++) {
				children.push([(x + 0) % spanX, (y + 0) % spanY]);
			}
		}
		return children;
	}

	/**
	 * @property crs: BaseCRS
	 * Read-only getter to the pyramid's CRS.
	 */
	get crs() {
		return this.#crs;
	}

	/**
	 * @section Level iterators
	 * @method forEachLevel(fn: Function): this
	 * Runs the given `Function` `fn` on each level of the pyramid.
	 *
	 * The function will receive two parameters: the name of the level (as a `String`), and
	 * the level definition (as an `Object` with `scale`, `bbox`, `spanX`, `spanY` properties).
	 */
	forEachLevel(fn) {
		Object.entries(this.#levels).forEach(([name, def]) => fn(name, def));

		return this;
	}

	/**
	 * @method mapLevels(fn: Function): Array
	 * Runs the given `Function` `fn` on each level of the pyramid, and returns an array
	 * containing all the return values from each call.
	 *
	 * In other words, works akin to [`Array.prototype.map`](https://developer.mozilla.org/en-US/docs/Web/JavaScript/Reference/Global_Objects/Array/map.html).
	 *
	 * The function will receive two parameters: the name of the level (as a `String`), and
	 * the level definition (as an `Object` with `scale`, `bbox`, `spanX`, `spanY` properties).
	 */
	mapLevels(fn) {
		return Object.entries(this.#levels).map(([name, def]) => fn(name, def));
	}

	/**
	 * @method getLevelsCount(): Number
	 * Returns the number of levels in this pyramid.
	 *
	 * Note that the names of the levels might not be numeric: this is just the lenght
	 * of an hypothetical array containing the levels.
	 */
	getLevelsCount() {
		return Object.keys(this.#levels).length;
	}

	/**
	 * @method getLevelDef(name: String): Object
	 *
	 * Returns the definition of a pyramid level given its identifier/name (or `undefined`
	 * if there's no level with that identifier).
	 */
	getLevelDef(name) {
		return this.#levels[name];
	}
}
