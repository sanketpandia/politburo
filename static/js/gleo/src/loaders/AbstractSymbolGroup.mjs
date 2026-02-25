import Loader from "./Loader.mjs";

/**
 * @class AbstractSymbolGroup
 * @inherits Loader
 * @relationship aggregationOf GleoSymbol, 0..1, 0..n
 *
 * Abstract base class for `Loader`s that can have `GleoSymbol`s added to them
 * (e.g. symbols can be added to a `Clusterer` loader instead of a `Platina` or
 * `GleoMap`).
 *
 */

/// TODO: Should this have events for symbols added / removed??

export default class AbstractSymbolGroup extends Loader {
	constructor(opts) {
		super(opts);

		/**
		 * @section Subclass interface
		 * @uninheritable
		 * @property symbols: Set of GleoSymbol
		 * The `GleoSymbol`s added to this group.
		 * @property loaders: Array of Loader
		 * The `Loader`s added to this group. These can be loaders that spawn symbols,
		 * or
		 */
		this.symbols = new Set();

		// this.#boundAddSymbols = this._addSymbols.bind(this);
		this.#boundAddSymbols = (ev) => {
			this._addSymbols(ev.detail.symbols);
		};
		// this.#boundRemoveSymbols = this._removeSymbols.bind(this);
		this.#boundRemoveSymbols = (ev) => {
			this._removeSymbols(ev.detail.symbols);
		};
	}

	#boundAddSymbols;
	#boundRemoveSymbols;
	#loaders = [];

	/**
	 * @section
	 * @method add(symbol: GleoSymbol): this
	 * Adds the given symbol to this symbol group.
	 * @alternative
	 * @method add(loader: Loader): this
	 * Adds the given loader to this symbol group. Symbols from that loader will
	 * be put into this group.
	 */
	add(symbol) {
		if (symbol instanceof Loader) {
			this._addLoaders([symbol]);
		} else {
			this._addSymbols([symbol]);
		}
		return this;
	}

	/**
	 * @section Subclass interface
	 * @method _addSymbols(Array of GleoSymbol)
	 * Tracks the given symbols. Can be overriden by subclasses.
	 */
	_addSymbols(symbols) {
		for (const s of symbols) {
			this.symbols.add(s);
		}
	}

	/**
	 * @section Subclass interface
	 * @method _addLoaders(Array of Loader)
	 * Tracks the given loaders. Can be overriden by subclasses.
	 */
	_addLoaders(loaders) {
		this.#loaders = this.#loaders.concat(loaders);
		loaders.forEach((l) => {
			l.on("symbolsadded", this.#boundAddSymbols);
			l.on("symbolsremoved", this.#boundRemoveSymbols);
			if (l.target !== this) {
				if (!l.target) {
					l.addTo(this);
				} else {
					throw new Error("Cannot add a Loader that already has a target");
				}
			}
			// l.addTo(this);
		});
	}

	/**
	 * @section Subclass interface
	 * @method _removeSymbols(Array of GleoSymbol)
	 * Stops tracking the given symbols. Can be overriden by subclasses.
	 */
	_removeSymbols(symbols) {
		for (const s of symbols) {
			this.symbols.delete(s);
		}
	}

	/**
	 * @section Subclass interface
	 * @method _removeLoaders(Array of Loader)
	 * Stops tracking the given loaders. Can be overriden by subclasses.
	 */
	_removeLoaders(loaders) {
		loaders.forEach((l) => {
			if (l.target !== this) {
				throw new Error("Cannot remove a Loader that hasn't been added here");
			}
			l.remove();
			l.off("symbolsadded", this.#boundAddSymbols);
			l.off("symbolsremoved", this.#boundRemoveSymbols);
		});
		this.#loaders = this.#loaders.filter((l) => !loaders.includes(l));
	}

	_addToPlatina(platina) {
		super._addToPlatina(platina);
		this.#loaders.forEach((l) => l._addToPlatina(platina));
	}

	/**
	 * @section
	 * @method multiAdd(symbols: Array of GleoSymbol): this
	 * Adds the given symbols to this group loader
	 * @alternative
	 * @method multiAdd(loaders: Array of Loader): this
	 * Adds the given loaders to this group loader
	 */
	multiAdd(symbols) {
		/// This implementations is probably inefficient, but ensures that the
		/// a `multiAdd()` call will call the `add()` method from the right subclass.
		this._addLoaders(
			symbols.filter((s) => s instanceof Loader && !this.#loaders.includes(s))
		);
		this._addSymbols(
			symbols.filter((s) => !(s instanceof Loader) && !this.symbols.has(s))
		);
		return this;
	}

	/**
	 * @method remove(): this
	 * Removes the `Loader` from the map/platina it was in.
	 * @alternative
	 * @method remove(symbol: GleoSymbol): this
	 * Removes the given symbol from this group loader.
	 * @alternative
	 * @method remove(loader: Loader): this
	 * Removes the given `Loader` from this group loader.
	 */
	remove(symbol) {
		if (symbol) {
			if (symbol instanceof Loader) {
				this._removeLoaders([symbol]);
			} else if (this.symbols.includes(symbol)) {
				this._removeSymbols([symbol]);
			}

			return this;
		} else {
			return super.remove();
		}
	}

	/**
	 * @method multiRemove(symbols): this
	 * Removes the given symbols from this group loader
	 */
	multiRemove(symbols) {
		this._removeLoaders(symbols.filter((s) => s instanceof Loader));
		this._removeSymbols(symbols.filter((s) => !(s instanceof Loader)));

		return this;
	}

	/**
	 * @method empty(): this
	 * Empties the symbol group, by removing all known symbols and loaders in it.
	 */
	empty() {
		this.symbols.clear();
		this.#loaders.length = 0;
		return this;
	}

	/**
	 * @method has(symbol: GleoSymbol): Boolean
	 * Returns `true` if this loader contains the given symbol, false otherwise.
	 * @alternative
	 * @method has(symbol: Loader): Boolean
	 * Returns `true` if this loader contains the given loader, false otherwise.
	 */
	has(s) {
		return this.symbols.has(s) || this.#loaders.includes(s);
	}
}
