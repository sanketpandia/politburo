import GleofyEventClass from "./GleoEventMixin.mjs";

/**
 *
 * @class GleoMouseEvent
 * @inherits MouseEvent
 * @inherits GleoEvent
 *
 * Akin to `GleoPointerEvent`, but only used for `click` events (since `click` events are
 * instances of `MouseEvent`, not `PointerEvent`).
 *
 * Note that Gleo **does not handle** any other `MouseEvent`s. Trying to do something like
 * `map.on('mousedown', fn)` will do nothing.
 *
 * @example
 *
 * ```js
 * map.on('click', function(ev) {
 * 	console.log(ev.geom);
 * });
 * ```
 *
 */

export default class GleoMouseEvent extends GleofyEventClass(MouseEvent) {}
