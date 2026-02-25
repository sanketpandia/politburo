import Platina from "../Platina.mjs";
import Evented from "../dom/Evented.mjs";

/**
 * @class Loader
 * @inherits Evented
 *
 * A `Loader` loads/unloads symbols as needed.
 *
 * Some `Loader`s might watch for changes in the map's (or the `Platina`s)
 * state (center, scale, etc) and load/unload symbols based on that.
 *
 * Some other `Loader`s work with data formats, making network requests and
 * parsing data.
 *
 *
 * @event symbolsadded
 * Fired whenever the loader reports new symbols. Even details include such symbols.
 * @event symbolsremoved
 * Fired whenever the loader forgets or unloads old symbols. Event details include such symbols.
 */

export default class Loader extends Evented {
	#target;
	#platina;

	constructor({ attribution } = {}) {
		super();
		/**
		 * @option attribution: String = undefined
		 * The HTML attribution to be shown in the `AttributionControl`, if any.
		 */
		this.attribution = attribution;
	}

	/**
	 * @method addTo(target: GleoMap): this
	 * Adds the `Loader` to the given `GleoMap`
	 * @alternative
	 * @method addTo(target: Platina): this
	 * Adds the `Loader` to the given `Platina`
	 * @alternative addTo(target: SymbolGroup
	 * Adds the `Loader` to the given `SymbolGroup`
	 */
	addTo(target) {
		if (!this.#target && target.has(this)) {
			// This loader is in the process of being added to a SymbolGroup
			this.#target = target;
		} else {
			this.#target = target;
			target.add(this);
		}

		if (target.platina) {
			this._addToPlatina(target.platina);
		} else if (target instanceof Platina) {
			this._addToPlatina(target);
		}
		return this;
	}

	/**
	 * @property target: GleoMap
	 * The target where the loader was added to. Might be a `GleoMap`, a `Platina`
	 * or a `SymbolGroup`. Read-only.
	 * @alternative
	 * @property target: Platina
	 * @alternative
	 * @property target: SymbolGroup
	 */
	get target() {
		return this.#target;
	}

	/**
	 * @property platina
	 * The `Platina` where symbols from this loader will be drawn into. Read-only.
	 */
	get platina() {
		return this.#platina;
	}

	/**
	 * @class Loader
	 * @method remove(): this
	 * Removes the `Loader` from the map/platina it was in.
	 */
	remove() {
		if (!this.#target) {
			throw new Error("Cannot remove Loader: is already removed");
		}

		this.#platina = this.#target = undefined;
		return this;
	}

	// Internal use only.
	// Called when the loader gets attached to a platina. Might not happen
	// immediately, in cases like a loader gets added to a SymbolGroup, and later
	// that SymbolGroup gets added to a Platina.
	// Subclasses may extend this method.
	_addToPlatina(platina) {
		this.#platina = platina;
		this.#target ||= platina;
	}
}
