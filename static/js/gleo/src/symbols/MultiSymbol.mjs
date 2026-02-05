import GleoSymbol from "./Symbol.mjs";
import ExpandBox from "../geometry/ExpandBox.mjs";

/**
 * @class MultiSymbol
 * @inherits GleoSymbol
 *
 * @relationship compositionOf GleoSymbol, 0..1, 0..n
 *
 * A logical grouping of `GleoSymbol`s, to ease the task of managing them at once.
 *
 * Adding/removing it from a map, will add/remove all of the component symbols at once.
 * Idem for (re-)setting its geometry. Idem for pointer events: events
 * defined for the `MultiSymbol` will be trigger on any of its components.
 *
 * This is meant for static sets of symbols which represent the same geographical
 * feature, and share the same geometry (or close geometries). Once created,
 * no new symbols can be added to a `MultiSymbol`.
 *
 * For a counterpart where symbols can be added/removed, see the `SymbolGroup` loader.
 */

/// TODO: Offer an iterator, **if** needed.

export default class MultiSymbol extends GleoSymbol {
	#symbols = [];

	/**
	 * @constructor MultiSymbol(syms: Array of GleoSymbol)
	 */
	constructor(syms) {
		super();
		this.geometry = syms[0]?.geometry;
		this.#symbols = syms;
		syms.forEach((s) => s._eventParents.push(this));
	}

	get symbols() {
		return this.#symbols;
	}

	addTo(target) {
		target.multiAdd(this.#symbols);
		return this;
	}

	remove() {
		// TODO: Bin into similar acetates, call MultiRemove. Low priority.
		this.#symbols.forEach((s) => s.remove());
		return this;
	}

	/**
	 * @property bbox
	 * Returns a bounding box which covers all the geometries of all component
	 * symbols.
	 */
	get bbox() {
		if (!this.#bbox) {
			this.#bbox = new ExpandBox();
			this.#symbols.forEach((s) => {
				this.#bbox.expandGeometry(s.geometry.toCRS(this.geometry.crs));
			});
		}
		return this.#bbox;
	}
	#bbox;

	get geometry() {
		return super.geometry;
	}
	set geometry(geom) {
		super.geometry = geom;
		this.#symbols?.forEach((s) => (s.geometry = geom));
	}

	set cursor(c) {
		this.#symbols.forEach((s) => (s.cursor = c));
	}
	get cursor() {
		return this.#symbols[0].cursor;
	}

	isActive() {
		return this.#symbols.some((s) => s.isActive());
	}
}
