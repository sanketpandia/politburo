import AbstractTileLoader from "./AbstractTileLoader.mjs";

import TileEvent from "../dom/TileEvent.mjs";

/**
 * @class GenericVectorTileLoader
 * @inherits AbstractTileLoader
 * @relationship compositionOf GleoSymbol, 0..n, 0..n
 *
 * Generic, format-less, vector tile loader.
 *
 * Uses a `Pyramid` like a `TileLoader` does, but instead of a raster image,
 * each tile contains a number of `GleoSymbol`s instead.
 *
 * This is the base, format-less, implementation; most users will be interested
 * in a vector tile loader which requests and parses vector tiles in protobuf
 * or geojson format. This class does allow for synthetic vector tiles, though.
 */

export default class GenericVectorTileLoader extends AbstractTileLoader {
	// Tile and request cache.
	// For vector tiles, the cache follows a Leaflet-like data structure: a
	// hash `Map` of "tile keys" to cache slots, one such hash map per
	// pyramid level.
	// Each cache slot (a tile/tile request) is a JS object of the form:
	// {x, y, req, data, abortController}
	// The vector tile cache is different from the raster tile cache due to the
	// need of different caching/pruning algorithms. In particular, raster
	// can implicitly prune tiles (by overwriting the same modulo XY), but
	// vector tiles must prune with more precision. Since a vector tile
	// has symbols on different acetates, loading overlapping vector tiles
	// do NOT obscure the data from the overlapped tile.
	#cached = {};

	/**
	 * @section
	 * A `GenericVectorTileLoader` is built upon a `TilePyramid` and a function
	 * that, given the pyramid level ("`z`"), the coordinates of a tile
	 * within that level ("`x`" and "`y`"), and an `AbortController`, returns
	 * an `Array` of `GleoSymbol`s, or a `Promise` to such an array.
	 *
	 * @constructor GenericVectorTileLoader(pyramid: TilePyramid, tileFn: Function, opts: GenericVectorTileLoader Options)
	 */
	constructor(pyramid, fn, { ...opts } = {}) {
		super(pyramid, opts);
		this._tileFn = fn;

		// Init cache
		pyramid.forEachLevel((name, _def) => {
			this.#cached[name] = new Map();
		});
	}

	addTo(target) {
		super.addTo(target);
		this._boundOnViewChange();
		return this;
	}

	remove() {
		const removableSymbols = Object.values(this.#cached)
			.map((level) => {
				return Array.from(level.values()).map(({ data, abortController }) => {
					abortController?.abort();
					return data;
				});
			})
			.flat(2)
			.filter((s) => !!s);

		this.platina.multiRemove(removableSymbols);

		return super.remove();
	}

	_abortLevel(level) {
		this.#cached[level].forEach(({ abortController, data }, key) => {
			abortController?.abort();
		});
		//this.#cached[level].clear();
	}

	#deleteOutsideVisibleRange(level) {
		// TODO: It should be possible to cache the CRS bbox somehow. That
		// way, there shouldn't be a need to recalculate the offset CRS bbox
		// each time.

		const [minX, minY, maxX, maxY] = this._getVisibleRange(level);
		const { spanX, spanY } = this.pyramid.getLevelDef(level);
		const cachedLevel = this.#cached[level];
		//console.log("OOB check", level, minX, minY, maxX, maxY, spanX, spanY);
		// Abort & delete tiles outside the range
		const removableSymbols = Array.from(cachedLevel)
			.filter(
				([key, { x, y }]) =>
					!this._isTileWithinRange(x, y, minX, minY, maxX, maxY, spanX, spanY)
			)
			.map(([key, { x, y, abortController, data }]) => {
				/**
				 * @section
				 * @event tileout: TileEvent
				 * Dispatched when a vector tile is deleted due to
				 * being out of the platina's visible bounds.
				 */
				this.dispatchEvent(
					new TileEvent("tileout", {
						tileLevel: level,
						tileX: x,
						tileY: y,
						tile: data,
					})
				);

				abortController?.abort();
				cachedLevel.delete(key);
				//console.log("OOB remove: ",level, key);
				return data ? data : [];
			});

		return removableSymbols;
	}

	_onRangeChange(level, minX, minY, maxX, maxY, levelChange) {
		const cachedLevel = this.#cached[level];
		const { spanX, spanY } = this.pyramid.getLevelDef(level);

		if ((maxX - minX) * (maxY - minY) > 256) {
			// This amount of tiles shouldn't appear during normal operation
			console.warn("Attempted to load too many vector tiles");
			return;
		}

		// Abort & delete tiles outside the range, for the current level...
		const removableSymbols = [this.#deleteOutsideVisibleRange(level)];

		// ...and any other levels with loaded tiles
		Object.entries(this.#cached).forEach(([pruneLevelName, pruneLevel]) => {
			if (pruneLevel.size === 0) {
				return;
			}
			if (pruneLevelName === level) {
				return;
			}

			removableSymbols.push(this.#deleteOutsideVisibleRange(pruneLevelName));
		});

		//if (removableSymbols.flat().length) {console.log('removing symbols', removableSymbols);}
		this.platina.multiRemove(removableSymbols.flat(2));

		// Load tiles inside the range, by looping through the range.
		for (let i = minX; i < maxX; i++) {
			for (let j = minY; j < maxY; j++) {
				const x = i % spanX;
				const y = j % spanY;

				const key = `x${x}y${y}`;

				if (cachedLevel.has(key)) {
					// Trigger the pruning logic for this tile.
					// If not done, a race condition might occur:
					// - load tiles from level *z*
					// - zoom in to level *z+1*
					// - allow just a partial set of tiles to load at *z+1*,
					//   without pruning from *z*
					// - switch back to *z*
					// - tiles from *z+1* should be pruned, but won't: the tiles
					//   from *z*, even though they're current, won't trigger
					//   the pruning logic because they never load... because
					//   they were never removed.
					if (levelChange) {
						this._prune(level, x, y);
					}
				} else {
					const abortController = new AbortController();
					const req = Promise.resolve(
						this._tileFn(level, x, y, abortController)
					);

					const slot = {
						x,
						y,
						req,
						data: undefined,
						abortController,
					};

					req.then((data) => {
						const [lastMinX, lastMinY, lastMaxX, lastMaxY] =
							this.currentRange;

						/// Async, so compare against the current range, not the range
						/// inside the closure
						if (
							this.currentLevel !== level ||
							!this._isTileWithinRange(
								x,
								y,
								lastMinX,
								lastMinY,
								lastMaxX,
								lastMaxY,
								spanX,
								spanY
							)
						) {
							// Async, non-abortable tile finished loading when
							// the viewport already changed
							//console.log("Not loadable", key);
							return;
						}
						slot.data = data;
						this._onTileLoad(level, x, y, data);
					}).catch((err) => {
						// Invalidate this cache slot
						cachedLevel.delete(key);

						this._onTileError(level, x, y, err);
					});

					cachedLevel.set(key, slot);
				}
			}
		}
	}

	_onTileLoad(level, x, y, symbols) {
		this._prune(level, x, y);
		this.platina.multiAdd(symbols);
		return super._onTileLoad(level, x, y, symbols);
	}

	_prune(level, x, y) {
		/**
		 * Pruning algorithm.
		 *
		 * The same for all levels, no matter if they're parents/ascendants
		 * (lower zoom / higher scale) or children/descendents (higher zoom /
		 * lower scale).
		 *
		 * For each level with present tiles:
		 * - get the tile range for the bounds of the loaded tile
		 * - loop through the range
		 * - See if there's a cached tile. If there is,
		 * - calculate its bounding box
		 * - get the tile range for that bbox, for the current pyramid level
		 * - clip that tile range with the currently visible tile range
		 * - Loop through that tile range. If all of those exist, then tile
		 *   cah be pruned.
		 */
		const cachedLevel = this.#cached[level];
		const bounds = this.pyramid.tileCoordsToBbox(level, [x, y]);
		const visibleRange = this._getVisibleRange(level);
		const removableSymbols = [];
		const { spanX, spanY } = this.pyramid.getLevelDef(level);

		//console.log("Loaded: ", level, x, y);

		Object.entries(this.#cached).forEach(([pruneLevelName, pruneLevel]) => {
			if (pruneLevel.size === 0) {
				return;
			}
			if (pruneLevelName === level) {
				return;
			}

			const [pMinX, pMinY, pMaxX, pMaxY] = this.pyramid.bboxToTileRange(
				pruneLevelName,
				bounds
			);

			for (let pX = pMinX; pX < pMaxX; pX++) {
				for (let pY = pMinY; pY < pMaxY; pY++) {
					const pruneKey = `x${pX}y${pY}`;
					//console.log("checking prunable key", pruneLevelName, pruneKey);

					if (pruneLevel.has(pruneKey)) {
						const overlapRange = this.pyramid.bboxToTileRange(
							level,
							this.pyramid.tileCoordsToBbox(pruneLevelName, [pX, pY])
						);

						let oMinX = Math.max(overlapRange[0], visibleRange[0]);
						let oMinY = Math.max(overlapRange[1], visibleRange[1]);
						let oMaxX = Math.min(overlapRange[2], visibleRange[2]);
						let oMaxY = Math.min(overlapRange[3], visibleRange[3]);

						/// Antimeridian artefact prevention.
						/// The tile ranges (the ones overlapping
						/// the prunable tile vs the visible range) do not play
						/// nicely over the antimeridian. Namely, when overlapRange
						/// is zero (or close) and visibleRange is greater than
						/// the level span.

						if (oMinX > oMaxX) {
							// Horizontal antimeridian
							oMinX = Math.max(overlapRange[0], visibleRange[0] - spanX);
							oMaxX = Math.min(overlapRange[2], visibleRange[2] - spanX);
						}
						if (oMinY > oMaxY) {
							// Vertical antimeridian
							oMinY = Math.max(overlapRange[1], visibleRange[1] - spanY);
							oMaxY = Math.min(overlapRange[3], visibleRange[3] - spanY);
						}

						const xs = new Array(oMaxX - oMinX)
							.fill(0)
							.map((_, i) => oMinX + i);
						const ys = new Array(oMaxY - oMinY)
							.fill(0)
							.map((_, i) => oMinY + i);

						const candidates = xs
							.map((i) => ys.map((j) => `x${i}y${j}`))
							.flat();

						//console.log(overlapRange, candidates);

						//const debugCount = candidates.filter(can=>cachedLevel.get(can)?.data).length;

						if (candidates.every((can) => cachedLevel.get(can)?.data)) {
							// Tile pX,pY from pruneLevel can and will be pruned

							const prunableTile = pruneLevel.get(pruneKey);
							// this.platina.multiRemove(pruneLevel.get(pruneKey).data);
							removableSymbols.push(prunableTile.data || []);
							pruneLevel.delete(pruneKey);
							/**
							 * @section
							 * @event tileprune: TileEvent
							 * Dispatched when a tile is deleted due to pruning. This happens
							 * whenever a prunable tile is (a) not the best fit for the current
							 * map scale and (b) completely covered by tiles from the fitting
							 * pyramid level.
							 */
							this.dispatchEvent(
								new TileEvent("tileprune", {
									tileLevel: level,
									tileX: x,
									tileY: y,
									tile: prunableTile,
								})
							);

							// console.log("prune", `x${x}y${y}`, '→', pruneKey);
							//console.log(`${level}x${x}y${y}`, `→ ${pruneLevelName}${pruneKey} ${debugCount}/${candidates.length} → prune`)
						} else {
							//console.log(`${level}x${x}y${y}`, `→ ${pruneLevelName}${pruneKey} ${debugCount}/${candidates.length}`)
						}
					}
				}
			}
		});

		this.platina.multiRemove(removableSymbols.flat());
	}

	// debug
	get cache() {
		return this.#cached;
	}
}
