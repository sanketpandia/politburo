import GleofyEventClass from "./GleoEventMixin.mjs";

/**
 *
 * @class GleoPointerEvent
 * @inherits PointerEvent
 * @inherits GleoEvent
 *
 * Gleo maps handle pointer events - such as `pointerdown`. All pointer
 * events fired by a map instance are not instances of `PointerEvent`, but of
 * `GleoPointerEvent`.
 *
 * This means that a map's pointer events have extra properties - most notably
 * a geometry with the CRS coordinates where the pointer event happened.
 *
 * For the `click` event, see `GleoMouseEvent`.
 *
 * @example
 *
 * ```js
 * map.on('pointerdown', function(ev) {
 * 	console.log(ev.geom);
 * });
 * ```
 */

export default class GleoPointerEvent extends GleofyEventClass(PointerEvent) {}
