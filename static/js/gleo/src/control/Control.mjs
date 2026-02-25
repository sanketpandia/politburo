import css from "../dom/CSS.mjs";
import Evented from "../dom/Evented.mjs";

css(`
.gleo-control {
	display: block;
}
`);

/**
 * @class Control
 * @inherits Evented
 *
 * Abstract UI control.
 */

export default class Control extends Evented {
	/**
	 * @constructor Control(opts: Control options)
	 */
	constructor({ position = "tl" } = {}) {
		/**
		 * @option position: String
		 * One of `tl`, `tr`, `bl`, `br`. Indicates which corner of the map the
		 * control should be added to.
		 * @alternative
		 * @option position: HTMLElement
		 * Indicates that the `Control` should be added to the given HTML element.
		 * This allows for controls outside of the map interface itself.
		 */
		super();
		this.position = position;

		this.spawnElement();
	}

	/**
	 * @section Internal methods
	 * @method spawnElement(): HTMLElement
	 * Sets `this.element` to the appropriate value. Should be overriden by
	 * subclasses.
	 */
	spawnElement() {
		/**
		 * @section GleoMap interface
		 * @property element: HTMLElement
		 * The `HTMLElement` for the whole control. Should be treated as read-only.
		 */
		this.element = document.createElement("div");
		this.element.className = "gleo-control";
	}

	/**
	 * @section
	 * @method addTo(map: GleoMap): this
	 * Attaches the `Control` to the map, and appends the control's HTML element
	 * to the appropriate container.
	 */
	addTo(map) {
		if (this.position instanceof HTMLElement) {
			this.parent = this.position;
		} else {
			this.parent = map.controlPositions.get(this.position);
			if (!this.parent) {
				throw new Error("The gleo map control has no valid position/container");
			}
		}
		this.parent.appendChild(this.element);
		this._map = map;
		return this;
	}

	/**
	 * @method remove(): this
	 * Detaches the control from the map, and removes the HTML element from the DOM.
	 */
	remove() {
		this.parent.removeChild(this.element);
		this.parent = undefined;
		this._map = undefined;
	}
}
