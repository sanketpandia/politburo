/**
 *
 * @class TileEvent
 * @inherits Event
 *
 * A `TileLoader`'s events are of this type, and include information about the tile
 * in question.
 *
 *
 * @example
 *
 * ```js
 * loader.on('tileload', function(ev) {
 * 	console.log(ev.tileLevel);
 * });
 *
 * ```
 *
 * @property tileLevel: String
 * The level of the tile pyramid the tile is in.
 *
 * @property tileX: Number
 * The X coordinate of the tile, relative to its level.
 *
 * @property tileY: Number
 * The Y coordinate of the tile, relative to its level.
 *
 * @property tile: HTMLImageElement
 * The image for the tile.
 *
 * @property error: String
 * The cause of a `tileerror` event.
 *
 */

export default class TileEvent extends Event {
	constructor(type, init) {
		super(type, init);
		this.tileLevel = init.tileLevel;
		this.tileX = init.tileX;
		this.tileY = init.tileY;
		this.tile = init.tile;
		this.error = init.error;
	}
}
