import Platina from "../Platina.mjs";
import { factory } from "../geometry/DefaultGeometry.mjs";
import Evented from "../dom/Evented.mjs";
import AbstractSymbolGroup from "../loaders/AbstractSymbolGroup.mjs";

/**
 * @class GleoSymbol
 * @inherits Evented
 *
 * @relationship compositionOf RawGeometry, 0..n, 1..1
 *
 * An abstract base graphical symbol.
 *
 * (This would ideally called `Symbol`, but that's a reserved word in ES6 Javascript).
 *
 * A `GleoSymbol` is closely coupled to a matching `Acetate`. The `Acetate` defines
 * the WebGL program that will render symbols in the GPU; whereas `GleoSymbol`s hook
 * to an acetate, and fill up some data structures provided by it.
 *
 */

export default class GleoSymbol extends Evented {
	#attribution;
	#interactive;
	#geometry;
	#cursor;
	#allocationPromise;

	/**
	 * @constructor GleoSymbol(geom: RawGeometry, opts: GleoSymbol Options)
	 * @alternative
	 * @constructor GleoSymbol(geom: Array of Number, opts: GleoSymbol Options)
	 */
	constructor(
		geom,
		{
			/**
			 * @section
			 * @aka GleoSymbol Options
			 * @option attribution: String = undefined
			 * The HTML attribution to be shown in the `AttributionControl`, if any.
			 */
			attribution,
			/**
			 * @option interactive: Boolean = false
			 * Whether this `GleoSymbol` should fire mouse/pointer events (or not).
			 *
			 * Needs the symbol to be drawn in an appropriate `Acetate`.
			 */
			interactive = false,
			/**
			 * @option cursor: String = undefined
			 * The pointer cursor to be used when the primary is hovering over
			 * this symbol.
			 *
			 * Possible values are the keywords for the [`cursor` CSS property](https://developer.mozilla.org/docs/Web/CSS/cursor.html)
			 * (e.g. `"pointer"`, `"wait"`, `"move"`, etc).
			 *
			 * Needs `interactive` to be `true`.
			 */
			cursor,
		} = {}
	) {
		super();

		// The acetate instance this symbol is being drawn in:
		this._inAcetate = undefined;

		if (geom) {
			this.geometry = geom;
		}
		this.#attribution = attribution;
		this.#interactive = interactive;
		this.#cursor = cursor;

		// Used by `MultiSymbol` and such. Events dispatched to this symbol
		// will also be dispatched to all the event parents.
		this._eventParents = [];

		/**
		 * @section Acetate interface
		 * @uninheritable
		 *
		 * `GleoSymbol`s must expose the following information, so that `Acetate`s can
		 * allocate resources properly.
		 *
		 * `GleoSymbol`s can be thought as a collection of GL vertices (with attributes)
		 * and primitive slots (i.e. three vertex indices per triangle). A `GleoSymbol`
		 * must expose the amount of attribute and index slots needed, so that `Acetate`
		 * functionality can allocate space for attributes and for primitive indices.
		 *
		 * Note that `Dot`s and `AcetateDot`s ignore `idxBase` and `idxLength`, because
		 * they have no need for keeping track of how primitives must be drawn (because of
		 * the `POINTS` GL draw mode).
		 *
		 * @property attrBase: Number = undefined
		 * The (0-indexed) base offset for vertex attributes of this symbol. Set
		 * by `Acetate`. This is `undefined` until the `Acetate` allocates the
		 * GPU resources (vertices) for this symbol.
		 * @property idxBase: Number = undefined
		 * The (0-indexed) base offset for primitive slots of this symbol. Set
		 * by `AcetateVertices`. This is `undefined` until the `AcetateVertices`
		 * allocates the GPU resources (triangle primitive indices) for this symbol.
		 * @property attrLength: Number
		 * The amount of vertex attribute slots this symbol needs. Must be set by the symbol
		 * during construction time.
		 * @property idxLength: Number
		 * The amount of primitive slots this symbol needs. Must be set by the symbol
		 * during construction time.
		 *
		 * @property _id: Number
		 * The interactive ID of the symbol, to let `AcetateInteractive`
		 * functionality find it to dispatch pointer events.
		 */

		this.attrBase = undefined;
		this.idxBase = undefined;
		this.attrLength = undefined;
		this.idxLength = undefined;

		this.#allocationPromise = new Promise((res, _rej) => {
			this.#resolveAllocation = res;
		});
	}

	/**
	 * @section
	 * @property geometry: Geometry
	 * The symbol's (unprojected) geometry. Can be updated with a new `Geometry`
	 * or nested `Array`s of `Number`s.
	 * @property attribution: String; The symbol's attribution. Read-only.
	 * @property interactive: Boolean
	 * Whether the symbol should be interactive. Read-only.
	 * @property cursor: String
	 * The runtime value of the `cursor` option. Can only be updated when not
	 * being drawn.
	 */

	get attribution() {
		return this.#attribution;
	}
	get interactive() {
		return this.#interactive;
	}
	get cursor() {
		return this.#cursor;
	}
	set cursor(c) {
		if (this._inAcetate) {
			throw new Error(
				"Cannot update the `cursor` of a Symbol that is in an acetate. Remove it from the map first."
			);
		}
		this.#cursor = c;
	}

	get geometry() {
		return this.#geometry;
	}
	set geometry(geom) {
		this.#geometry = factory(geom);
	}

	// Compatibility alias
	get geom() {
		return this.geometry;
	}

	/**
	 * @section Static properties
	 * Any subclasses must implement the following static property.
	 * @property Acetate: Prototype of Acetate
	 * The `Acetate` prototype related to the symbol - the one that fits this class
	 * of symbol by default.
	 *
	 * This is implemented as a static property, i.e. a property of the `Dot` prototype,
	 * not of the `Dot` instances.
	 *
	 * This shall be used when adding a symbol to a map, in order
	 * to detect the default acetate it has to be added to.
	 */
	static Acetate = undefined;

	/**
	 * @section Lifecycle Methods
	 * @method addTo(map: GleoMap): this
	 * Adds this symbol to the map.
	 *
	 * The symbol will be added to the appropriate default acetate.
	 *
	 * @alternative
	 * @method addTo(acetate: Acetate): this
	 * Adds this symbol to the given `Acetate` (as long as the acetate fits the symbol).
	 *
	 * @alternative
	 * @method addTo(loader: AbstractSymbolGroup): this
	 * Adds this symbol to a `Loader` that accepts symbols.
	 */
	addTo(target) {
		const proto = this.constructor.Acetate;
		let acet, group;
		if (target instanceof proto) {
			acet = target;
		} else if (
			this.constructor.Acetate.PostAcetate &&
			target instanceof this.constructor.Acetate.PostAcetate
		) {
			acet = target.getAcetateOfClass(proto);
		} else if (target instanceof Platina) {
			acet = target.getAcetateOfClass(proto);
		} else if (target.platina instanceof Platina) {
			acet = target.platina.getAcetateOfClass(proto);
		} else if (target instanceof AbstractSymbolGroup) {
			group = target;
		}

		if (this._inAcetate) {
			if (acet === this._inAcetate) {
				return this;
			}
			throw new Error(
				`Could not add Symbol to ${target}, since symbol is already being drawn elsewhere.`
			);
		}
		if (acet) {
			acet.add(this);
		} else if (group) {
			group.add(this);
		} else {
			throw new Error(`Could not add Symbol to ${target}.`);
		}
		return this;
	}

	/**
	 * @method remove(): this
	 * Removes this symbol from its containing `Acetate` (and, therefore, from the
	 * containing `GleoMap`).
	 */
	remove() {
		if (!this._inAcetate) {
			throw new Error("Cannot remove Symbol: it's not being drawn already.");
		}
		this._inAcetate.remove(this);
		this._inAcetate = undefined;
		this.attrBase = undefined;
		this.idxBase = undefined;
		return this;
	}

	/**
	 * @method isActive(): Boolean
	 * Returns whether the symbol is "active": being drawn in any `Acetate`. In
	 * other words, it has correctly allocated all GPU resources needed for it
	 * to be drawn.
	 *
	 * Note that some symbols can take time to allocate their resources, so
	 * they can be "inactive" after they've been added to an acetate/platina/map.
	 * (e.g. `Sprite`s when they refer to an image from the network that hasn't
	 * finished loading)
	 *
	 * Note that this returns `true` for transparent or otherwise allocated yet
	 * invisible symbols.
	 */
	isActive() {
		return this._inAcetate && this.attrBase !== undefined;
	}

	/**
	 * @property allocation: Promise to Array of Number
	 * A `Promise` that resolves when the symbol has been allocated into an
	 * acetate (not just *added* to it). Before this promise resolves,
	 * the values of `attrBase` and `idxBase` are either `undefined` or
	 * not reliable.
	 */
	get allocation() {
		return this.#allocationPromise;
	}

	#resolveAllocation = function () {};
	#allocated = false;

	/**
	 * @section Acetate Interface
	 * @uninheritable
	 * @method updateRefs(ac: Acetate, atb: Number, idx: Number): this
	 * Internal usage only, called from a corresponding `Acetate`. Updates
	 * the acetate that this symbol is being currently drawn on, the base vertex
	 * attribute slot (`atb`), and the base vertex index slot (`idx`).
	 */
	updateRefs(ac, atb, idx) {
		this._inAcetate = ac;
		this.attrBase = atb;
		this.idxBase = idx;
		if (!this.#allocated && atb !== undefined && idx !== undefined) {
			// Symbol has just been allocated, resolve allocation promise
			this.#resolveAllocation([atb, idx]);
			this.#allocated = true;
		} else if (this.#allocated && (atb === undefined || idx === undefined)) {
			// Symbol has ust been deallocated, reset allocation promise
			this.#allocationPromise = new Promise((res, _rej) => {
				this.#resolveAllocation = res;
			});
			this.#allocated = false;
		}
		return this;
	}

	/**
	 * @method _setGlobalStrides(...): this
	 *
	 * OBSOLETE: use `_setGlobalStrides`/`_setGeometryStrides`/`_setPerPointStrides` instead.
	 *
	 * Should be implemented by subclasses. Acetates shall call this method
	 * with one `StridedTypedArray` per attribute, plus a `TypedArray` for the
	 * index buffer, plus any extra data they need.
	 *
	 * A symbol shall fill up values in the given strided arrays (based on
	 * the `attrBase` property) as well as in the typed array for the index
	 * buffer (based on the `idxBase` property).
	 *
	 *
	 * @method _setGlobalStrides(...): this
	 * Shall be implemented by subclasses. Acetates shall call this method
	 * with any number of `StridedTypedArray`s plus a `TypedArray` fir the
	 * index buffer, plus any extra data they need.
	 *
	 * The acetate shall call this method *once*, when the symbol is allocated.
	 *
	 * @method _setGeometryStrides(geom: Geometry, perPointStrides: Array of StridedArray, ...): this
	 * As `_setGlobalStrides`, but the acetate may call this whenever the
	 * geometry of the symbol changes. It receives the projected geometry and the
	 * per-point strides.
	 *
	 * This is needed for e.g. recalculation of line joins in `Stroke` (and
	 * updating the attribute buffer(s) where the line join data is in).
	 *
	 * @method _setPerPointStrides(n: Number, pointType: Symbol, vtx: Number, geom: Geometry, vtxCount: Number ...): this
	 * As `_setGlobalStrides`, but the acetate shall call this once per each
	 * point in the symbol's geometry.
	 *
	 * This method will receive the index of the point within the geometry
	 * (0-indexed), the type of point (whether the point is a line join, or
	 * line end, or a standalone point, or part of a mesh), the vertex index
	 * for this point, the amount of vertices spawned for that point,
	 * plus any strided arrays.
	 *
	 */

	// TODO: Consider having a _setGlobalStridesGeom, that shall run whenever
	// the geometry changes. Some attributes depend on the geometry (e.g.
	// the extrusion in `Stroke`), as do the indices of `Fill`.

	/**
	 * @section Pointer events
	 *
	 * All `GleoSymbol`s (set as `interactive`, and being drawn in an appropriate
	 * acetate) fire pointer events, in a way similar to how `HTMLElement`s fire
	 * [DOM `PointerEvent`s](https://developer.mozilla.org/docs/Web/API/Pointer_events).
	 *
	 * Gleo adds the `Geometry` corresponding to the pixel the event took place in.
	 *
	 * Most events are `GleoPointerEvent`s, but some browsers fire
	 * exclusively `MouseEvent`s for `click`/`auxclick`/`contextmenu`. In
	 * that case, expect a `GleoMouseEvent` instead.
	 *
	 * @event click: GleoPointerEvent
	 * Akin to the [DOM `click` event](https://developer.mozilla.org/docs/Web/API/Element/click_event)
	 * @event dblclick: GleoPointerEvent
	 * Akin to the [DOM `dblclick` event](https://developer.mozilla.org/docs/Web/API/Element/dblclick_event)
	 * @event auxclick: GleoPointerEvent
	 * Akin to the [DOM `auxclick` event](https://developer.mozilla.org/docs/Web/API/Element/auxclick_event)
	 * @event contextmenu: GleoPointerEvent
	 * Akin to the [DOM `contextmenu` event](https://developer.mozilla.org/docs/Web/API/Element/contextmenu_event)
	 * @event pointerover: GleoPointerEvent
	 * Akin to the [DOM `pointerover` event](https://developer.mozilla.org/docs/Web/API/HTMLElement/pointerover_event)
	 * @event pointerenter: GleoPointerEvent
	 * Akin to the [DOM `pointerenter` event](https://developer.mozilla.org/docs/Web/API/HTMLElement/pointerenter_event)
	 * @event pointerdown: GleoPointerEvent
	 * Akin to the [DOM `pointerdown` event](https://developer.mozilla.org/docs/Web/API/HTMLElement/pointerdown_event)
	 * @event pointermove: GleoPointerEvent
	 * Akin to the [DOM `pointermove` event](https://developer.mozilla.org/docs/Web/API/HTMLElement/pointermove_event)
	 * @event pointerup: GleoPointerEvent
	 * Akin to the [DOM `pointerup` event](https://developer.mozilla.org/docs/Web/API/HTMLElement/pointerup_event)
	 * @event pointercancel: GleoPointerEvent
	 * Akin to the [DOM `pointercancel` event](https://developer.mozilla.org/docs/Web/API/HTMLElement/pointercancel_event)
	 * @event pointerout: GleoPointerEvent
	 * Akin to the [DOM `pointerout` event](https://developer.mozilla.org/docs/Web/API/HTMLElement/pointerout_event)
	 * @event pointerleave: GleoPointerEvent
	 * Akin to the [DOM `pointerleave` event](https://developer.mozilla.org/docs/Web/API/HTMLElement/pointerleave_event)
	 */

	/**
	 * @section Pointer methods
	 * @method setPointerCapture(pointerId: Number): this
	 * Akin to [`Element.setPointerCapture()`](https://developer.mozilla.org/docs/Web/API/Element/setPointerCapture)
	 * @method releasePointerCapture(pointerId: Number): this
	 * Akin to [`Element.releasePointerCapture()`](https://developer.mozilla.org/docs/Web/API/Element/releasePointerCapture)
	 */
	setPointerCapture(pointerId) {
		this._inAcetate?.setPointerCapture(pointerId, this);
		return this;
	}
	releasePointerCapture(pointerId) {
		this._inAcetate?.releasePointerCapture(pointerId);
	}

	/**
	 * @section
	 * @method debugDump(): Object
	 *
	 * Returns a verbose, human-understandable representation of the underlying
	 * WebGL data (vertex attributes and triangle primitives) for this symbol.
	 *
	 * This is an computationally expensive operation and should only be used for
	 * debugging purposes. Note that this functions just retuns a value, so make
	 * sure to `console.log()` or likewise.
	 */
	debugDump() {
		if (!this._inAcetate) {
			throw new Error(
				"Cannot dump debug info for symbol since it's not being drawn in any acetate"
			);
		}
		if (!this._inAcetate._program) {
			throw new Error(
				"Cannot dump debug info for symbol since it's being drawn on an acetate that has not yet linked its WebGL program"
			);
		}

		return {
			idxBase: this.idxBase,
			idxLength: this.idxLength,
			attrBase: this.attrBase,
			attrLength: this.attrLength,
			attrs: this._inAcetate._program.debugDumpAttributes(
				this.attrBase,
				this.attrLength
			),
			idxs: this._inAcetate._program._indexBuff._ramData.slice(
				this.idxBase,
				this.idxBase + this.idxLength
			),
		};
	}
}
