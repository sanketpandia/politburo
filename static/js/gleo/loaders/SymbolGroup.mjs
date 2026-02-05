import AbstractSymbolGroup from "./AbstractSymbolGroup.mjs";

/**
 * @class SymbolGroup
 * @inherits AbstractSymbolGroup
 *
 * Akin to Leaflet's `LayerGroup`. Groups symbols together so that they can be
 * added to/removed  at once by adding/removing the symbol group. Symbols can be
 * added to/removed from the group as well.
 *
 * In addition to symbols, accepts nested `Loader`s.
 *
 * For grouping symbols relating to the same geographical feature, use `MultiSymbol`
 * instead.
 */

export default class SymbolGroup extends AbstractSymbolGroup {
	addTo(target) {
		super.addTo(target);
		this.target.multiAdd(Array.from(this.symbols));
		return this;
	}

	// _addToPlatina(p) {
	// 	super._addToPlatina(p);
	// 	p.multiAdd(this.symbols);
	// }

	_addSymbols(symbols) {
		super._addSymbols(symbols);
		this.target?.multiAdd(symbols);
		this.fire("symbolsadded", { symbols });
	}

	_removeSymbols(symbols) {
		super._removeSymbols(symbols);
		this.target?.multiRemove(symbols);
		this.fire("symbolsremoved", { symbols });
	}

	empty() {
		this.target?.multiRemove(Array.from(this.symbols));
		this.fire("symbolsremoved", { symbols: this.symbols });
		return super.empty();
	}

	remove(s) {
		if (!s && this.target) {
			this.target.multiRemove(Array.from(this.symbols));
		}
		return super.remove(s);
	}
}
