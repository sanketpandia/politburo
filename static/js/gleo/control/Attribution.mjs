import css from "../dom/CSS.mjs";
import Control from "./Control.mjs";

css(`
.gleo-control-attribution {
	background: #ccc;
	padding: 0.25em;
}
`);

/**
 * @class Attribution
 * @inherits Control
 * An informative attribution control. It shall display HTML text (with links)
 * based on the `attribution` option of `GleoSymbol`s added to the map.
 */
export default class Attribution extends Control {
	/**
	 * @constructor Attribution(opts: Attribution Options)
	 */
	constructor({
		/**
		 * @section Attribution Options
		 * @option separator: String = ' | '
		 * A string to separate different attributions
		 */
		separator = " | ",
		/**
		 * @option prefix: String = 'Gleo'
		 * A prefixed attribution that shall always be present irrespective of
		 * symbols in the map.
		 */
		prefix = "<a href='https://gitlab.com/IvanSanchez/gleo/' target=_blank>Gleo</a>",
		position = "br",
		...opts
	} = {}) {
		super({ position, ...opts });

		this._separator = separator;
		this._prefix = prefix;

		this._boundOnAcetateAdd = this._onAcetateAdd.bind(this);
		this._boundOnSymbolAdd = this._onSymbolAdd.bind(this);
		this._boundOnSymbolRemove = this._onSymbolRemove.bind(this);
		this._boundOnLoaderAdd = this._onLoaderAdd.bind(this);
		this._boundOnLoaderRemove = this._onLoaderRemove.bind(this);

		this.counter = new Map();
	}

	spawnElement() {
		this.element = document.createElement("div");
		this.element.className = "gleo-control gleo-control-attribution";
	}

	addTo(map) {
		super.addTo(map);
		map.on("acetateadded", this._boundOnAcetateAdd);
		map.on("symbolsadded", this._boundOnSymbolAdd);
		map.on("symbolsremoved", this._boundOnSymbolRemove);
		map.on("loaderadded", this._boundOnLoaderAdd);
		map.on("loaderremoved", this._boundOnLoaderRemove);

		/// TODO: Should fetch all of the map's loaders and symbols
		/// and calculate the initial attribution
	}

	remove() {
		super.remove();
		this._map.off("acetateadded", this._boundOnAcetateAdd);
		this._map.off("symbolsadded", this._boundOnSymbolAdd);
		this._map.off("symbolsremoved", this._boundOnSymbolRemoved);
		this._map.off("loaderadded", this._boundOnLoaderAdd);
		this._map.off("loaderremoved", this._boundOnLoaderRemove);
	}

	_onAcetateAdd(ev) {
		// this._onAdd(ev.detail.symbols.map((s) => s.attribution));
		// console.log("Attribution acetateadded", ev.detail.constructor.name, ev.detail.attribution);
		ev.detail.attribution && this._onAdd([ev.detail.attribution]);
	}
	_onSymbolAdd(ev) {
		this._onAdd(ev.detail.symbols.map((s) => s.attribution));
	}
	_onSymbolRemove(ev) {
		this._onRemove(ev.detail.symbols.map((s) => s.attribution));
	}
	_onLoaderAdd(ev) {
		this._onAdd([ev.detail.loader.attribution]);
	}
	_onLoaderRemove(ev) {
		this._onRemove([ev.detail.loader.attribution]);
	}

	_onAdd(attributions) {
		let mustUpdate = false;

		attributions
			.filter((a) => !!a)
			.forEach((a) => {
				const c = this.counter.get(a) || 0;
				if (!c) {
					mustUpdate = true;
				}
				this.counter.set(a, c + 1);
			});

		if (mustUpdate) {
			this._update();
		}
	}

	_onRemove(attributions) {
		let mustUpdate = false;

		attributions
			.filter((a) => !!a)
			.forEach((a) => {
				const c = this.counter.get(a);
				// Might fail if a symbol/loader updates its attribution without notifying it
				if (!c) {
					// throw new Error("Removed an unknown attribution");
					console.warn("Removed an unknown attribution", a);
				}
				if (c === 1) {
					this.counter.delete(a);
					mustUpdate = true;
				} else {
					this.counter.set(a, c - 1);
				}
			});

		if (mustUpdate) {
			this._update();
		}
	}

	_update() {
		this.element.innerHTML = [this._prefix]
			.concat(Array.from(this.counter.keys()))
			.join(this._separator);
	}
}
