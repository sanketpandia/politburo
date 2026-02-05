// ES6-style class mixin (see
// https://developer.mozilla.org/en-US/docs/Web/JavaScript/Reference/Classes#mix-ins )
// for both `GleoMouseEvent` and `GleoPointerEvent`

/**
 * @class GleoEvent
 *
 * [Class mix-in](https://developer.mozilla.org/en-US/docs/Web/JavaScript/Reference/Classes#mix-ins)
 * for functionality common to `GleoPointerEvent` and `GleoMouseEvent`.
 */

export default function GleofyEventClass(Base) {
	return class GleoEvent extends Base {
		constructor(type, init) {
			super(type, init);

			/**
			 * @property geometry: Geometry
			 * A point `Geometry`, containing the coordinates in map's CRS where the
			 * event took place.
			 * This is akin to Leaflet's `latlng` event property.
			 *
			 * @property canvasX: Number
			 * Akin to `clientX`, but relative to the map's `<canvas>` element.
			 *
			 * @property canvasY: Number
			 * Akin to `clientY`, but relative to the map's `<canvas>` element.
			 */
			this.geometry = init.geometry;
			this.canvasX = init.canvasX;
			this.canvasY = init.canvasY;

			/**
			 * @property colour: Array of Number
			 * For `Acetate`s set as "queryable" (and `GleoSymbol`s being
			 * drawn in such acetates), this contains the colour of the pixel
			 * the event took place over, as a 4-element array of the form
			 * `[r, g, b, a]`, with values between 0 and 255.
			 * @alternative
			 * @property colour: undefined
			 * For `Acetate`s **not** set as "queryable" (and `GleoSymbol`s
			 * being drawn in such acetates), and for event that happened outside
			 * of the map (e.g. `pointerout` events), this value will always
			 * be `undefined`)
			 */
			this.colour = init.colour;

			this._canPropagate = true;
		}

		stopPropagation() {
			this._canPropagate = false;
			return super.stopPropagation();
		}
	};
}
