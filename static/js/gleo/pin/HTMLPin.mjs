import css from "../dom/CSS.mjs";
import AbstractPin from "./AbstractPin.mjs";
import { factory } from "../geometry/DefaultGeometry.mjs";

css(`
.gleo .pin {
	z-index: 1;
	position: absolute;
}
`);

/**
 * @class HTMLPin
 * @inherits AbstractPin
 *
 * A HTML element that is pinned to a point `Geometry` inside a `GleoMap`, displaying
 * on top of the map's `Platina`.
 *
 * This is an unstyled HTML element - users should consider the `Balloon` class
 * instead, which provides styling.
 *
 * Note that lots of `HTMLPin`s mean lots of HTML elements in the DOM, which
 * means more load in the browser. Therefore, lots of `HTMLPin`s should be avoided.
 */

export default class HTMLPin extends AbstractPin {
	#map;
	#boundOnViewChange;
	#boundOnCRSChange;
	#element;
	#geometry;
	#projectedGeometry;
	#offset;

	/**
	 * @constructor HTMLPin(geometry: RawGeometry, contents: HTMLElement)
	 * @alternative
	 * @constructor HTMLPin(geometry: RawGeometry, contents: String)
	 * @alternative
	 * @constructor HTMLPin(geometry: Array of Number, contents: HTMLElement)
	 * @alternative
	 * @constructor HTMLPin(geometry: Array of Number, contents: String)
	 */
	constructor(
		geometry,
		contents,
		{
			/**
			 * @section HTMLPin Options
			 * @option offset: Array of Number = [0,0]
			 * Pixel offset of the `HTMLPin` (relative to the pixel where the
			 * point geometry is projected to).
			 * @option cssClass: String = undefined
			 * Optional CSS class to be applied to the pin's element.
			 */
			offset = [0, 0],
			cssClass,
		} = {}
	) {
		super();
		this.#boundOnViewChange = this.#onViewChange.bind(this);
		this.#boundOnCRSChange = this.#onCRSChange.bind(this);
		this.#geometry = factory(geometry);

		/// TODO: Sanity check on the dimension of the geometry. Whereas
		/// sprites and other extruded point symbols might take multipoints
		/// as an input (and cast linestrings/polys/multipolys to multipoints),
		/// an HTMLPin actually does need a point geometry.

		if (contents instanceof HTMLElement) {
			this.#element = contents;
		} else {
			this.#element = document.createElement("div");
			this.#element.innerHTML = contents;
		}
		this.#element.classList.add("pin");
		if (cssClass) {
			this.#element.classList.add(cssClass);
		}
		this.#offset = offset;
	}

	/**
	 * @section
	 * @method addTo(map: GleoMap): this
	 * Adds this pin to the given map
	 */
	addTo(map) {
		this.#map = map;
		if (map.crs) {
			this.#projectedGeometry = this.#geometry.toCRS(map.crs);
		}
		map.platina.on("viewchanged", this.#boundOnViewChange);
		map.platina.on("crschange", this.#boundOnCRSChange);
		map.platina.on("crsoffset", this.#boundOnCRSChange);
		map.container.appendChild(this.#element);
		this.#onViewChange();

		map.on("destroy", this.remove.bind(this));
		return this;
	}

	/**
	 * @method remove(): this
	 * Removes this pin fom whatever map it's in.
	 */
	remove() {
		if (!this.#map) {
			return this;
		}
		this.#map.platina?.off("viewchanged", this.#boundOnViewChange);
		this.#map.platina?.off("crschange", this.#boundOnCRSChange);
		this.#map.platina?.off("crsoffset", this.#boundOnCRSChange);
		this.#map.container?.removeChild(this.#element);
		this.#map = undefined;
		return this;
	}

	#onViewChange(ev) {
		const pos = this.#map.platina.geomToPx(this.#projectedGeometry);

		this.#element.style.left = `${pos[0] + this.#offset[0]}px`;
		this.#element.style.top = `${pos[1] + this.#offset[1]}px`;
	}

	#onCRSChange(ev) {
		this.#projectedGeometry = this.#geometry.toCRS(ev.detail.newCRS);
		this.#onViewChange();
	}

	/**
	 * @property element: HTMLElement
	 * Read-only accesor to this pin's `HTMLElement`.
	 */
	get element() {
		return this.#element;
	}

	/**
	 * @property geometry: RawGeometry
	 * The point `Geometry` for the pin. Can be overwritten with a new point
	 * `Geometry`.
	 */
	get geometry() {
		return this.#geometry;
	}
	set geometry(geom) {
		this.#geometry = factory(geom);
		if (this.#map?.platina) {
			this.#onCRSChange({
				detail: { newCRS: this.#map.platina.crs },
			});
		}
	}
}
