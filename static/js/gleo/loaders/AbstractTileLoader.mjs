import Loader from "./Loader.mjs";
import TileEvent from "../dom/TileEvent.mjs";

/**
 * @class AbstractTileLoader
 * @inherits Loader
 *
 * @relationship association TileEvent, 1..1, 0..n
 *
 * Functionality common to `RasterTileLoader` and `AbstractVectorTileLoader`.
 *
 * A `AbstractTileLoader` watches for changes in the map's viewport and
 * loads/unloads/overwrites raster/vector tiles.
 */
export default class AbstractTileLoader extends Loader {
	#pyramid;
	// #boundOnViewChange;
	//#tileFn;

	// Pyramid level that was the best fit for the platina's scale during the
	// last viewchange event
	#lastLevel;

	// Tile range fitting the last viewchange event
	#lastRange = [NaN, NaN, NaN, NaN];

	/**
	 * @constructor GenericVectorTileLoader(pyramid: TilePyramid, opts: GenericVectorTileLoader Options, tileWrapX: Number, tileWrapY: Number)
	 */
	constructor(pyramid, { ...opts } = {}) {
		super(opts);
		this.#pyramid = pyramid;
		this._boundOnViewChange = this.#onViewChange.bind(this);
	}

	/**
	 * @property pyramid: TilePyramid
	 * The tile pyramid used. Read-only.
	 */
	get pyramid() {
		return this.#pyramid;
	}

	/**
	 * @property currentLevel: String
	 * The name of the level of the pyramid currently active (the one best
	 * fitting the map/platina's scale). Read-only.
	 */
	get currentLevel() {
		return this.#lastLevel;
	}

	/**
	 * @property currentRange: Array of Number
	 * An array of the form `[minX, minY, maxX, maxY]` containing the range
	 * of tile XY coordinates which cover the current map/platina viewport.
	 * Read-only.
	 */
	get currentRange() {
		return this.#lastRange;
	}

	addTo(target) {
		super.addTo(target);
		this.platina.on("viewchanged", this._boundOnViewChange);
	}

	remove() {
		this.platina.off("viewchanged", this._boundOnViewChange);

		super.remove();
		this.#lastRange = [NaN, NaN, NaN, NaN];

		/// TODO: remove all symbols. Subclasses are best equipped to deal with that.
		return this;
	}

	_getVisibleRange(level) {
		/// TODO: PROJECT THE BBOX TO THE PYRAMID CRS!!!!!!!
		const mapBBox = this.platina.bbox;
		const crs = this.platina.crs;
		const bbox = crs
			.offsetToBase([mapBBox.minX, mapBBox.minY])
			.concat(crs.offsetToBase([mapBBox.maxX, mapBBox.maxY]));
		const range = this.#pyramid.bboxToTileRange(level, bbox);
		return range;
	}

	/**
	 * @section Extension methods
	 * @uninheritable
	 * @method _isTileWithinRange(x: Number, y: Number, minX: Number, minY: Number, maxX: Number, maxY: Number, spanX: Number, spanY: Number): Boolean
	 * Can (and should) be used by implementations to check whether a set
	 * of `x`, `y` tile coordinates are within the given tile range with
	 * the given tile span.
	 * Tile ranges are assumed to be minimum-inclusive but maximum exclusive,
	 * i.e. `[min, max)`
	 */
	_isTileWithinRange(x, y, minX, minY, maxX, maxY, spanX, spanY) {
		return (
			(maxX > spanX ? x >= minX || x < maxX % spanX : x >= minX && x < maxX) &&
			(maxY > spanY ? y >= minY || y < maxY % spanY : y >= minY && y < maxY)
		);
	}

	#onViewChange(ev) {
		const level = this.#pyramid.nearestLevel(this.platina.scale);
		if (level === undefined) {
			// Happens when the platina doesn't have a scale set (yet)
			return;
		}
		const range = this._getVisibleRange(level);

		if (level !== this.#lastLevel && this.#lastLevel !== undefined) {
			/// If there has been a level change, (try to) abort all
			/// tiles from the outgoing level

			/**
			 * @section Extension methods
			 * @uninheritable
			 * @method _abortLevel(level:String): undefined
			 * Must be provided by raster and vector implementations. Should
			 * (try to) abort all pending `Promise`s for the given level.
			 */
			this._abortLevel(this.#lastLevel);

			this.#lastRange = [NaN, NaN, NaN, NaN];
		}

		if (this.#lastRange.every((v, i) => v === range[i])) {
			return;
		}

		const [minX, minY, maxX, maxY] = range;

		/**
		 * @section
		 * @event rangechange: Event
		 * Fired whenever the visible tiles (the "tile range") have changed.
		 * Not every change in the viewport triggers a range change.
		 */
		this.fire("rangechange", {
			level,
			minX,
			minY,
			maxX,
			maxY,
		});

		/**
		 * @section Extension methods
		 * @uninheritable
		 * @method _onRangeChange(level:String, minX: Number, minY: Number, maxX: Number, maxY: Number, levelChange: Boolean): undefined
		 * Must be provided by raster and vector implementations.
		 * Called whenever a `rangechange` event occurs. Implementations should
		 * (a) abort tiles outside the range and (b) load tiles inside the range,
		 * all according to their caching algorithm.
		 */
		this._onRangeChange(level, minX, minY, maxX, maxY, level !== this.#lastLevel);

		this.#lastLevel = level;
		this.#lastRange = range;
	}

	/**
	 * @section Extension methods
	 * @uninheritable
	 * @method _onTileLoad(level: String, x: Number, y: Number, tile: *): undefined
	 * Should be called when a tile loads. The abstract implementation will
	 * only fire a `tileload` event and trigger a redraw.
	 */
	_onTileLoad(level, x, y, tile) {
		/**
		 * @section
		 * @event tileload: TileEvent
		 * Dispatched when a tile loads.
		 */
		this.dispatchEvent(
			new TileEvent("tileload", {
				tileLevel: level,
				tileX: x,
				tileY: y,
				tile: tile,
			})
		);
	}

	/**
	 * @section Extension methods
	 * @uninheritable
	 * @method _onTileError(level: String, x: Number, y: Number, tile: *): undefined
	 * Should be called when a tile fails to load. The abstract implementation will
	 * only fire a `tileerror` event.
	 */
	_onTileError(level, x, y, err) {
		/**
		 * @section
		 * @event tileerror: TileEvent
		 * Dispatched when a tile failed to load.
		 */
		this.dispatchEvent(
			new TileEvent("tileerror", {
				tileLevel: level,
				tileX: x,
				tileY: y,
				error: err,
			})
		);
	}
}
