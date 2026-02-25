import AbstractTileLoader from "./AbstractTileLoader.mjs";
import AcetateStitchedTiles from "../acetates/AcetateStitchedTiles.mjs";
import Tile from "../symbols/Tile.mjs";
import Geometry from "../geometry/Geometry.mjs";
import imagePromise from "../util/imagePromise.mjs";

// Aux function to cover headless environments.
// NOTE: This implies an uncovered edge case - a resizable platina on a
// headless environment (where `screen` does not exist). There doesn't
// seem to be a trivial best approach covering that case. So, 1024px.
function getMaxScreenSize() {
	try {
		return Math.max(screen.width, screen.height);
	} catch (ex) {
		return 1024;
	}
}

/**
 * @class RasterTileLoader
 * @inherits AbstractTileLoader
 * @relationship compositionOf AcetateStitchedTiles, 1..1, 1..1
 *
 * Loads raster tiles, according to a Gleo `TilePyramid` and a callback function
 * that returns tiles given the tile coordinates.
 *
 * Will automatically spawn an `AcetateStitchedTiles`.
 *
 */
export default class RasterTileLoader extends AbstractTileLoader {
	#boundOnLevelExpelled;

	// Tile and tile request cache.
	// Raster tile loaders use a sliding window - the cache holds a tile on
	// a position given by the modulo of the tile XY coordinate.
	// The cache itself is a simple key-value JS object, keyed by the names
	// of the pyramid levels.
	// Each value is an `Array` of tiles/tile requests. The array is 1-dimensional,
	// and has a set maximum size (tileWrapX times tileWrapY);  the index of
	// the array comes from the tile coordinates modulo tileWrapX/tileWrapY.
	// Each tile/tile request is a JS object of the form: {x, y, req, data, abortController}
	#cached = {};

	#opts = {};
	#zIndex = 0;
	#tileFn;
	#fallback;
	#retry;

	#pendingReqs = 0;
	#lastLevel;
	#fadeInDuration;
	#cleanupTimeout;

	/**
	 * @section
	 *
	 * A `RasterTileLoader` needs a `TilePyramid` and a function that, given the
	 * pyramid level ("`z`"), the coordinates of a tile within that level
	 * ("`x`" and "`y`"), and an instance of `AbortController`, returns an
	 * instance of `HTMLImageElement`, or a `Promise` to such an image. The
	 * promise should be rejected whenever the abort controller's signal is
	 * activated.
	 *
	 * @constructor TileLoader(pyramid:TilePyramid, tileFn: Function, opts: TileLoader Options)
	 */
	constructor(
		pyramid,
		fn,
		{
			/// FIXME: tile resolution is per pyramid level, not global!!
			/**
			 * @section TileLoader Options
			 * @option tileResX: Number = 256; Horizontal size, in source raster pixels, of each tile.
			 * @alternative
			 * @option tileResX: Object of String to Number
			 * A map of level identifier to horizontal raster size (in source raster pixels).
			 * e.g. `{"0": 512, "1": 256}`
			 * @option tileResY: Number = 256; Vertical size, in source raster pixels, of each tile.
			 * @option tileResY: Object of String to Number
			 * A map of level identifier to vertical raster size (in source raster pixels).
			 * e.g. `{"0": 512, "1": 256}`
			 * @option zIndex: Number = -5500; The z-index of the acetate for these tiles.
			 */
			tileResX = 256,
			tileResY = 256,

			zIndex = -5500,

			/**
			 * @option fallback: HTMLImageElement
			 * An image to use as fallback is loading a tile fails.
			 * @alternative
			 * @option fallback: URL
			 * Idem, but using the `URL` to an image.
			 * @alternative
			 * @option fallback: String
			 * Idem, but using a `String` containing a URL
			 */
			fallback,

			/**
			 * @option retry: Boolean = false
			 * When `true`, tiles that failed to load will be re-requested
			 * the next time the tile extent changes (i.e. moving the map enough
			 * so that new tiles become visible). This can potentially
			 * lead to lots of requests for missing tiles.
			 */
			retry = false,

			/// TODO: Additional option to enable/disable scale snap points

			/**
			 * @section Options passed to spawned acetate
			 * A `RasterTileLoader` creates a `AcetateStitchedTiles` under the hood.
			 * The following options are passed through to this acetate.
			 * @option interpolate: Boolean = false
			 * Whether to use bilinear pixel interpolation or not.
			 *
			 * In other words: `false` means pixellated, `true` means smoother.
			 * @option fadeInDuration: Number = 250
			 * Duration, in milliseconds, of the tile fade-in animation.
			 * @option maxLoadedLevels: Number = 3
			 * Number of maximum tile levels to keep loaded in their textures.
			 * Higher values can provide a slightly better experience when
			 * zooming in and out, but will use more GPU RAM.
			 * @option resizablePlatina: Boolean = true
			 * Whether the platina can be expected to be resized up to the size
			 * of the screen. When `false`, less GPU RAM is used for the textures.
			 */
			fadeInDuration = 250,

			...opts
		} = {}
	) {
		super(pyramid, opts);

		this.#boundOnLevelExpelled = this.#onLevelExpelled.bind(this);
		this.#tileFn = fn;
		this.#tileResX = tileResX;
		this.#tileResY = tileResY;
		this.#zIndex = zIndex;
		this.#opts = opts;
		if (fallback) {
			this.#fallback = imagePromise(fallback);
		} else {
			this.#fallback;
		}
		this.#retry = retry;
		this.#fadeInDuration = fadeInDuration;
	}

	#tileResX;
	#tileResY;
	// 	#textureSizeX;
	// 	#textureSizeY;

	addTo(target) {
		super.addTo(target);

		let maxTileSize = 0;
		if (isFinite(this.#tileResX)) {
			maxTileSize = this.#tileResX;
		} else {
			maxTileSize = Math.max.apply(null, Object.values(this.#tileResX));
		}
		if (isFinite(this.#tileResY)) {
			maxTileSize = Math.max(maxTileSize, this.#tileResY);
		} else {
			maxTileSize = Math.max.apply(null, Object.values(this.#tileResY));
		}

		const minTextureSize =
			// 	this.platina.resizable && typeof screen !== undefined
			// 		? getMaxScreenSize() :
			Math.max.apply(null, this.platina.pxSize) + maxTileSize;
		// const minTextureSize = 1024;

		this._ac = new AcetateStitchedTiles(this.platina.glii, {
			...this.#opts,

			pyramid: this.pyramid,
			tileResX: this.#tileResX,
			tileResY: this.#tileResY,
			minTextureSize,
			// textureSizeX: this.#textureSizeX,
			// textureSizeY: this.#textureSizeY,
			zIndex: this.#zIndex,
			fadeInDuration: this.#fadeInDuration,
		});
		if (target.addAcetate) {
			target.addAcetate(this._ac);
			this._ac._platina = this.platina;
		} else {
			this.platina.addAcetate(this._ac);
		}

		this.pyramid.forEachLevel((_name, def) => {
			this.platina.setScaleStop(this.pyramid.crs.name, def.scale);
		});

		//this.platina.on("viewchanged", this._boundOnViewChange);
		this._ac.on("levelexpelled", this.#boundOnLevelExpelled);

		if (target.actuators && target.actuators.get("zoomsnap")) {
			// Trigger the map setter, and thus the ZoomYawSnapActuator functionality
			target.scale = target.scale;
		}

		this._boundOnViewChange();

		return this;
	}

	remove() {
		this._ac.off("levelexpelled", this.#boundOnLevelExpelled);

		/// remove acetate from map
		this._ac.destroy();

		super.remove();
		/// TODO: Remove the scale stops
		return this;
	}

	_abortLevel(level) {
		this.#cached[level].forEach(({ data, abortController, x, y }) => {
			if (!data) {
				abortController?.abort();
				// console.log("aborted", level, x, y);
			}
		});
	}

	_onRangeChange(level, minX, minY, maxX, maxY) {
		if (!this.#cached[level]) {
			// Init cache for level
			// console.log("Create tile cache for level", level);
			const levelInfo = this._ac.getLevelsInfo()[level];
			this.#cached[level] = new Array(levelInfo.wrapX * levelInfo.wrapY)
				.fill(0)
				.map(() => {
					return {
						x: undefined,
						y: undefined,
						req: undefined,
						data: undefined,
						abortController: undefined,
					};
				});
		}

		const cachedLevel = this.#cached[level];
		this.#lastLevel = level;

		// console.log(cachedLevel);

		const { spanX, spanY } = this.pyramid.getLevelDef(level);

		if ((maxX - minX) * (maxY - minY) > 256) {
			// This amount of tiles shouldn't appear during normal operation
			console.warn("Attempted to load too many raster tiles");
			return;
		}

		// Abort tiles outside the range, by looping through all
		// the cache slots in the current level.
		cachedLevel.forEach(({ x, y, abortController }) => {
			// Check if the request is outside the range,
			// accounting for the non-trivial case of comparing
			// a maxX that wraps around spanX
			if (
				(maxX > spanX ? x < minX && x > maxX % spanX : x < minX || x > maxX) ||
				(maxY > spanY ? y < minY && y > maxY % spanY : y < minY || y > maxY)
			) {
				// console.log("Aborting", cachedLevel, x, y);
				/// Abort tiles in the level, but outside the range.
				abortController?.abort();
			}
		});

		let levelInfo = this._ac.getLevelsInfo()[level];
		// const reqCount = 0;

		// Load tiles inside the range, by looping through the range.
		for (let i = minX; i < maxX; i++) {
			for (let j = minY; j < maxY; j++) {
				const x = i % spanX;
				const y = j % spanY;

				const xmod = (x % levelInfo.wrapX) * levelInfo.wrapY;
				const ymod = y % levelInfo.wrapY;
				const cacheSlot = cachedLevel[xmod + ymod];

				if (
					cacheSlot.x !== x ||
					cacheSlot.y !== y ||
					cacheSlot.abortController?.signal?.aborted
				) {
					cacheSlot.abortController?.abort();
					cacheSlot.data = undefined;
					cacheSlot.x = x;
					cacheSlot.y = y;

					const abortController = (cacheSlot.abortController =
						new AbortController());
					const req = (cacheSlot.req = Promise.resolve(
						this.#tileFn(level, x, y, abortController)
					));

					this.#pendingReqs++;

					req.then((data) => {
						this.#decreasePendingReqs();
						const [lastMinX, lastMinY, lastMaxX, lastMaxY] =
							this.currentRange;

						/// Async, so compare against the current range, not the range
						/// inside the closure
						if (
							this.currentLevel !== level ||
							i < lastMinX ||
							i > lastMaxX ||
							j < lastMinY ||
							j > lastMaxY
						) {
							// Async, non-abortable tile finished loading when
							// the viewport already changed
							return;
						}
						cacheSlot.data = data;
						this._onTileLoad(level, x, y, data);

						// this.#prune(level, x, y);
					}).catch((err) => {
						this.#decreasePendingReqs();
						const [lastMinX, lastMinY, lastMaxX, lastMaxY] =
							this.currentRange;
						if (
							this.currentLevel !== level ||
							i < lastMinX ||
							i > lastMaxX ||
							j < lastMinY ||
							j > lastMaxY
						) {
							// Async, non-abortable tile failed when
							// the viewport already changed
							return;
						}

						if (this.#retry) {
							// Invalidate this cache slot
							cacheSlot.x = NaN;
							cacheSlot.y = NaN;
							cacheSlot.data = undefined;
						}

						if (this.#fallback) {
							this.#fallback.then((f) =>
								this._onTileLoad(level, x, y, f, true)
							);
						} else {
							this._onTileError(level, x, y, err);
						}
					});
				}
			}
		}
		// console.log("range change; pending:", this.#pendingReqs);

		if (this.#pendingReqs) {
			clearTimeout(this.#cleanupTimeout);
		}
	}

	_onTileLoad(level, x, y, img, isFallback = false) {
		const bounds = this.pyramid.tileCoordsToBbox(level, [x, y]);
		const geom = new Geometry(
			this.pyramid.crs,
			[
				[bounds[0], bounds[1]],
				[bounds[2], bounds[1]],
				[bounds[2], bounds[3]],
				[bounds[0], bounds[3]],
			],
			{ wrap: false }
		);
		this._ac.add(new Tile(geom, level, x, y, img));
		if (isFallback) {
			super._onTileError(level, x, y);
		} else {
			super._onTileLoad(level, x, y, img);
		}
	}

	#decreasePendingReqs() {
		this.#pendingReqs--;

		// console.log("pending:", this.#pendingReqs);
		if (this.#pendingReqs == 0) {
			this.#cleanupTimeout = setTimeout(() => {
				// console.log("cleanup");
				// Tell the acetate to destroy textures
				this._ac.destroyHigherScaleLevels(this.#lastLevel);

				// Mark tiles from those levels as invalid
				const acLevels = this._ac.getLevelsInfo();
				const scale = acLevels[this.#lastLevel].scale;

				Object.entries(acLevels).forEach(([name, level]) => {
					if (level.scale < scale) {
						delete this.#cached[name];
					}
				});
				// console.log(this.#cached);
			}, this.#fadeInDuration);
		}
	}

	#onLevelExpelled(ev) {
		const level = ev.detail.levelName;

		/// TODO: Expel the level from the acetate (mark as unavailable, free the texture, etc)
		delete this.#cached[level];

		//console.log("Level invalidated", ev.detail.levelName);
	}
}
