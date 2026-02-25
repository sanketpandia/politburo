import Platina from "./Platina.mjs";
import Evented from "./dom/Evented.mjs";
import AbstractPin from "./pin/AbstractPin.mjs";
import Geometry from "./geometry/Geometry.mjs";
import css from "./dom/CSS.mjs";
import { factory } from "./geometry/DefaultGeometry.mjs";
import ExpandBox from "./geometry/ExpandBox.mjs";
import { invert, transpose } from "./3rd-party/gl-matrix/mat3.mjs";
import { transformMat3 } from "./3rd-party/gl-matrix/vec3.mjs";
import RawGeometry from "./geometry/RawGeometry.mjs";

// TODO: Consider packaging URW Gothic (~83KiB) as woff, and
// redistribute it. It's supposed to be AGPL3 from
// See https://github.com/ArtifexSoftware/urw-base35-fonts

// TODO: Consider packaging TeX Gyre Adventor (~170KiB) as woff,
// and redistribute it. It's LPPL from http://www.gust.org.pl/projects/e-foundry/tex-gyre

// TODO: Research Avant Garde Pro; Montserrat (https://fonts.google.com/specimen/Montserrat)
css(`
.gleo {
	position: relative;
	overflow: clip;
	font-family: "TeX Gyre Adventor", "URW Gothic L", "Century Gothic", "Futura", Sans Serif;
}

.gleo > canvas {
	position: absolute;
	top: 0;
	bottom: 0;
	left: 0;
	right: 0;
	width: 100%;
	height: 100%;
}

.gleo-controlcorner { position: absolute; }
.gleo-controlcorner.tl { top: 0; left: 0; text-align: left;}
.gleo-controlcorner.tr { top: 0; right: 0; text-align: right;}
.gleo-controlcorner.bl { bottom: 0; left: 0; text-align: left; display: flex; flex-direction: column-reverse;}
.gleo-controlcorner.br { bottom: 0; right: 0; text-align: right;display: flex; flex-direction: column-reverse;}
`);

/**
 * @class GleoMap
 *
 * @inherits Evented
 * @relationship compositionOf Platina, 0..1, 1..1
 * @relationship compositionOf Control, 0..1, 1..1
 * @relationship compositionOf Actuator, 1..1, 0..n
 * @relationship compositionOf AbstractPin, 1..1, 0..n
 *
 * The `GleoMap` is the entry point of Gleo. Users wanting a quick start might
 * should look into `MercatorMap` instead.
 *
 * A `GleoMap` wraps together, inside the same `<div>`:
 * - A `Platina` (to do the drawing)
 * - `Actuator`s (for user interactivity)
 * - HTML `Control`s such as `ZoomInOut` buttons, `Attribution` and `ScaleBar`
 * - `HTMLPin`s such as `Balloon`s to display HTML content fixed on a
 *   geographical point.
 */

const actuators = [];
// Internal use only. Instances of
// GleoMap shall instantiate one of each during their own instantiation.

export function registerActuator(name, proto, enabledByDefault) {
	// I still find this syntax for object literals confusing.
	actuators.push({ name, proto, enabledByDefault });
}

export default class GleoMap extends Evented {
	_setViewFilters = [];
	#platina;

	/**
	 * @constructor GleoMap(div: HTMLDivElement, options: GleoMap Options)
	 * @alternative
	 * @constructor GleoMap(divID: String, options: GleoMap Options)
	 */
	constructor(container, options = {}) {
		super();
		/**
		 * @section GleoMap Options
		 * @option resizable: Boolean = true
		 * Whether the map should react to changes in the size of its DOM
		 * container. Setting to `false` enables some memory optimizations.
		 *
		 * @section View initialization options
		 * The desired initial view of the map can be set with these options. They work
		 * the same as the options passed to the `setView` method.
		 *
		 * @option center: Geometry = undefined
		 * The desired map center
		 *
		 * @option scale: Number = undefined
		 * The desired map scale (**in CRS units per CSS pixel**). Mutually exclusive
		 * with `span`.
		 *
		 * @option span: Number
		 * The desired span of the map (in **CRS units** on the **diagonal of the viewport**).
		 * Mutually exclusive with `scale`.
		 *
		 * @option yawDegrees: Number = 0
		 * The desired yaw rotation, in degrees relative to "north up", clockwise.
		 * Mutually exclusive with `yawRadians`.
		 *
		 * @option yawRadians: Number = 0
		 * The desired yaw rotation, in radians relative to "north up", counterclockwise.
		 * Mutually exclusive with `yawDegrees`.
		 *
		 * @option crs: BaseCRS
		 * The desired CRS of the map.
		 */
		this.options = options;

		this.container =
			typeof container === "string"
				? document.getElementById(container)
				: container;

		if (this.container._gleo) {
			console.warn(
				"DOM element already contains a Gleo map, old one is being destroyed."
			);
			this.container._gleo.destroy?.();
		}
		this.container._gleo = this;
		this.container.classList.add("gleo");

		this.canvas = document.createElement("canvas");
		this.container.appendChild(this.canvas);
		this.#platina = new Platina(this.canvas, { ...options, map: this });

		/**
		 * @section
		 * @property actuators: Map of Actuator
		 * A key-value `Map` of actuator names to `Actuator` instances.
		 */
		this.actuators = new Map();

		// Spawn registered actuators
		actuators.forEach(({ name, proto, enabledByDefault }) => {
			const ac = new proto(this);
			if (enabledByDefault) {
				ac.enable();
			}
			this.actuators.set(name, ac);
		});

		// Hook up platina events
		/**
		 * @section Pointer events
		 *
		 * All [DOM `PointerEvent`s](https://developer.mozilla.org/en-US/docs/Web/API/Pointer_events)
		 * to the `<canvas>` of the map's `Platina` are handled by Gleo.
		 * Besides all of `PointerEvent`'s properties and methods, Gleo adds
		 * the `Geometry` corresponding to the pixel the event took place in.
		 *
		 * Most events are `GleoPointerEvent`s, but some browsers fire
		 * exclusively `MouseEvent`s for `click`/`auxclick`/`contextmenu`. In
		 * that case, expect a `GleoMouseEvent` instead.
		 *
		 * @event click: GleoPointerEvent
		 * Akin to the [DOM `click` event](https://developer.mozilla.org/en-US/docs/Web/API/Element/click_event)
		 * @event dblclick: GleoPointerEvent
		 * Akin to the [DOM `dblclick` event](https://developer.mozilla.org/en-US/docs/Web/API/Element/dblclick_event)
		 * @event auxclick: GleoPointerEvent
		 * Akin to the [DOM `auxclick` event](https://developer.mozilla.org/en-US/docs/Web/API/Element/auxclick_event)
		 * @event contextmenu: GleoPointerEvent
		 * Akin to the [DOM `contextmenu` event](https://developer.mozilla.org/en-US/docs/Web/API/Element/contextmenu_event)
		 * @event pointerover: GleoPointerEvent
		 * Akin to the [DOM `pointerover` event](https://developer.mozilla.org/en-US/docs/Web/API/HTMLElement/pointerover_event)
		 * @event pointerenter: GleoPointerEvent
		 * Akin to the [DOM `pointerenter` event](https://developer.mozilla.org/en-US/docs/Web/API/HTMLElement/pointerenter_event)
		 * @event pointerdown: GleoPointerEvent
		 * Akin to the [DOM `pointerdown` event](https://developer.mozilla.org/en-US/docs/Web/API/HTMLElement/pointerdown_event)
		 * @event pointermove: GleoPointerEvent
		 * Akin to the [DOM `pointermove` event](https://developer.mozilla.org/en-US/docs/Web/API/HTMLElement/pointermove_event)
		 * @event pointerup: GleoPointerEvent
		 * Akin to the [DOM `pointerup` event](https://developer.mozilla.org/en-US/docs/Web/API/HTMLElement/pointerup_event)
		 * @event pointercancel: GleoPointerEvent
		 * Akin to the [DOM `pointercancel` event](https://developer.mozilla.org/en-US/docs/Web/API/HTMLElement/pointercancel_event)
		 * @event pointerout: GleoPointerEvent
		 * Akin to the [DOM `pointerout` event](https://developer.mozilla.org/en-US/docs/Web/API/HTMLElement/pointerout_event)
		 * @event pointerleave: GleoPointerEvent
		 * Akin to the [DOM `pointerleave` event](https://developer.mozilla.org/en-US/docs/Web/API/HTMLElement/pointerleave_event)
		 * @event gotpointercapture: GleoPointerEvent
		 * Akin to the [DOM `gotpointercapture` event](https://developer.mozilla.org/en-US/docs/Web/API/HTMLElement/gotpointercapture_event)
		 * @event lostpointercapture: GleoPointerEvent
		 * Akin to the [DOM `lostpointercapture` event](https://developer.mozilla.org/en-US/docs/Web/API/HTMLElement/lostpointercapture_event)
		 *
		 * @section Rendering events
		 * @event prerender
		 * Fired just *before* a symbol+acetate+platina render.
		 * @event render
		 * Fired just *after* a symbol+acetate+platina render.
		 *
		 * @section View change events
		 * @event crsoffset
		 * Fired when the CRS changes explicitly (by setting the map's
		 * CRS, or passing a `crs` option to a `setView` call)
		 * @event crsoffset
		 * Fired when the CRS undergoes an implicit offset to avoid
		 * precision loss.
		 * @event viewchanged
		 * Fired whenever any of the platina's view parts (center, scale/span,
		 * yaw, crs) changes.
		 *
		 * @section Symbol/loader management events
		 * @event symbolsadded
		 * Fired whenever symbols are added to any of the platina's acetates.
		 * @event symbolsremoved
		 * Fired whenever symbols are removed from any of th platina's acetates.
		 *
		 */
		for (let evName of [
			"click",
			"dblclick",
			"auxclick",
			"contextmenu",
			"pointerover",
			"pointerenter",
			"pointerdown",
			"pointermove",
			"pointerup",
			"pointercancel",
			"pointerout",
			"pointerleave",
			"gotpointercapture",
			"lostpointercapture",

			"prerender",
			"render",

			"crschange",
			"crsoffset",
			"viewchanged",

			"acetateadded",
			"symbolsadded",
			"symbolsremoved",
			"loaderadded",
			"loaderremoved",
		]) {
			this.#platina.addEventListener(evName, this._onPlatinaEvent.bind(this));
		}

		/**
		 * @section Controls interface
		 * @property controlPositions: Map of String to HTMLElement
		 * A `Map` containing the four control container corners, indexed
		 * by a two-letter string (one of `tl`, `tr`, `bl`, `br`)
		 */
		const corners = [`tl`, `tr`, `bl`, `br`];
		this.controlPositions = new Map();
		for (let corner of corners) {
			const el = document.createElement("div");
			el.className = `gleo-controlcorner ${corner}`;
			this.controlPositions.set(corner, el);
			this.container.appendChild(el);
		}
	}

	/**
	 * @section
	 * @method destroy(): this
	 * Destroys the map, freeing the container. Should free all used GPU resources,
	 * destroy DOM elements for controls and pins, and remove all DOM event listeners.
	 *
	 * No methods should be called on a destroyed map.
	 */
	destroy() {
		this.#platina?.destroy?.();
		this.#platina = undefined;

		this.controlPositions.forEach((corner) => this.container.removeChild(corner));

		/**
		 * @event destroy: Event
		 * Fired when the map is destroyed.
		 */
		this.fire("destroy");

		delete this.container._gleo;
		delete this.canvas;
		delete this.container;
	}

	/**
	 * @section
	 * @property container: HTMLDivElement
	 * The DOM element containing the map.
	 * @property platina: Platina
	 * The `Platina` that this `GleoMap` uses. Read only.
	 */
	get platina() {
		return this.#platina;
	}

	_onPlatinaEvent(ev) {
		const myEv = new ev.constructor(ev.type, ev);
		this.dispatchEvent(myEv);
	}

	/**
	 * @section View setter/getter properties
	 * These properties allow to fetch the state of the view then read, *and*
	 * modify it. Setting the value of any of these properties has the same
	 * effect as calling `setView()` with appropriate values.
	 *
	 * Setting a value will trigger the map's `Actuator`s. In most cases, this
	 * means starting an animation (via `InertiaActuator`) and keeping any
	 * values previously set as the target state of the animation.
	 *
	 * @property center: RawGeometry
	 * The center of the map, as a point geometry.
	 * @property scale: Number
	 * The scale, in CRS units per CSS pixel.
	 * @property span: Number
	 * The span, in CRS units per diagonal.
	 * @property crs: BaseCRS
	 * The CRS being used by the platina.
	 * @property yawDegrees: Number
	 * The yaw rotation angle, in clockwise degrees.
	 * @property yawRadians: Number
	 * The yaw rotation angle, in counter-clockwise radians.
	 */
	get center() {
		return this.#platina.center;
	}
	set center(c) {
		return this.setView({ center: c });
	}

	get scale() {
		return this.#platina.scale;
	}
	set scale(s) {
		return this.setView({ scale: s });
	}

	get span() {
		return this.#platina.span;
	}
	set span(s) {
		return this.setView({ span: s });
	}

	get crs() {
		return this.#platina.crs;
	}
	set crs(c) {
		return this.setView({ crs: c });
	}

	get yawRadians() {
		return this.#platina.yawRadians;
	}
	get yawDegrees() {
		return this.#platina.yawDegrees;
	}
	set yawRadians(y) {
		return this.setView({ yawRadians: y });
	}
	set yawDegrees(y) {
		return this.setView({ yawDegrees: y });
	}

	/**
	 * @property bbox: ExpandBox
	 * A rectangular bounding box that completely covers the map
	 * viewport. This box is aligned to the CRS, not to the viewport. Setting its
	 * value is akin to running `fitBounds`.
	 */
	get bbox() {
		return this.#platina.bbox;
	}
	set bbox(b) {
		this.fitBounds(b);
	}

	/**
	 * @section
	 * @property glii: GliiFactory
	 * The Glii instance used by the platina. Read-only.
	 */
	get glii() {
		return this.#platina.glii;
	}

	/**
	 * @section View setters
	 * @method setView(opts?: SetView Options): this
	 *
	 * (Re-)sets the map view: center (including its CRS), scale (AKA zoom) and yaw.
	 *
	 * This will trigger an animation to the desired center/scale if the appropriate
	 * `Actuator`s are enabled.
	 */
	setView(opts) {
		if (opts.center) {
			opts.center = factory(opts.center);
		}
		opts = this._setViewFilters.reduce((ops, fn) => fn(ops), opts);
		if (opts) {
			this.#platina.setView(opts);
		}
		return this;
	}

	/**
	 * @method fitBounds(bounds: Array of Number, opts?: SetView Options): this
	 * Sets the platina's center and scale so that the given bounds (given in
	 * `[minX, minY, maxX, maxY]` form, and in the platina's CRS) are fully
	 * visible.
	 *
	 * Any other given `SetView Options` will be merged with the calculated center&scale.
	 * @alternative
	 * @method fitBounds(ExpandBox, opts?: SetView Options): this
	 * Idem, but using an `ExpandBox` instead.
	 * @alternative
	 * @method fitBounds(bounds: RawGeometry, opts?: SetView Options): this
	 * Idem, but fitting to the bbox of a `Geometry` instead.
	 *
	 * This performs an implicit reprojection, so that it works as expected
	 * then the geometry's CRS is different than the map's CRS.
	 */
	fitBounds(bounds, opts = {}) {
		/// NOTE: This reimplements the code from `Platina` in order to perform
		/// animations (setview filters, actuators)
		/// FIXME: Currently sets the yaw to zero. Instead it should respect it.
		let minX, minY, maxX, maxY;
		if (bounds instanceof ExpandBox) {
			({ minX, minY, maxX, maxY } = bounds);
		} else if (bounds instanceof RawGeometry) {
			({ minX, minY, maxX, maxY } = bounds.toCRS(this.crs).bbox());
		} else {
			[minX, minY, maxX, maxY] = bounds;
		}

		const [w, h] = this.platina.pxSize;

		const center = new Geometry(this.crs, [(minX + maxX) / 2, (minY + maxY) / 2]);
		const scale = Math.max((maxX - minX) / w, (maxY - minY) / h);
		return this.setView({
			...opts,
			center: center,
			scale: scale,
			yaw: 0,
		});
	}

	/**
	 * @method zoomInto(geometry: Geometry, scale: Number, opts?: SetView Options): this
	 * Performs a `setView` operation so that the given geometry stays at the
	 * same pixel.
	 *
	 * Meant for user interactions on the map, including double-clicking and
	 * zooming into clusters from a `Clusterer`. Akin to Leaflet's `zoomAround`.
	 */
	zoomInto(geometry, scale, opts = {}) {
		const [canvasX, canvasY] = this.platina.geomToPx(factory(geometry));
		const [w, h] = this.#platina.pxSize;
		let clipX = (canvasX * 2) / w - 1;
		let clipY = (canvasY * -2) / h + 1;

		const scaleFactor = scale / this.scale;

		clipX -= clipX * scaleFactor;
		clipY -= clipY * scaleFactor;

		const vec = [clipX, clipY, 1];
		const invMatrix = invert(new Array(9), this.#platina._crsMatrix);
		transpose(invMatrix, invMatrix);
		transformMat3(vec, vec, invMatrix);

		const targetCenter = new Geometry(this.crs, [vec[0], vec[1]], { wrap: false });

		return this.setView({ ...opts, center: targetCenter, scale: scale });
	}

	/**
	 * @section
	 * @method redraw(): this
	 * Forcefully redraws all acetates.
	 */
	redraw() {
		this.#platina.redraw();
		return this;
	}

	/**
	 * @section Actuator interface methods
	 * @method registerSetViewFilter(fn: Function): this
	 * Registers a new filter function `fn` which will intercept calls to `setView`.
	 * This function receives a set of `setView Options` and must return a set of
	 * `setView Options`, or `false` (which aborts the `setView`).
	 *
	 * This is used to enforce map view constraints, as those of `ZoomYawSnapActuator`.
	 */
	registerSetViewFilter(fn) {
		this._setViewFilters.push(fn);
		return this;
	}
	/**
	 * @method unregisterSetViewFilter(fn: Function): this
	 * Opposite of `registerSetViewFilter`.
	 */
	unregisterSetViewFilter(fn) {
		const position = this._setViewFilters.indexOf(fn);
		if (position >= 0) {
			this._setViewFilters.splice(position, 1);
		}
		return this;
	}

	/**
	 * @section Symbol/Loader management
	 *
	 * A `GleoMap` doesn't really hold the state of symbols/loaders in the map.
	 * These calls are proxied to the map's `Platina`.
	 *
	 * @method add(symbol: GleoSymbol): this
	 * Adds the given `GleoSymbol` to the appropriate acetate.
	 *
	 * Users should note that repeated calls to `add()` are, in performance terms,
	 * **much worse** than a single call to `multiAdd()`. Try to avoid repeated calls
	 * to `add()` inside a loop.
	 *
	 * @alternative
	 * @method add(loader: Loader): this
	 * Attaches the given `Loader` to the map.
	 *
	 * @alternative
	 * @method add(pin: AbstractPin): this
	 * Attaches the given pin (`HTMLPin` *et al*) to the map.
	 */
	add(symbol) {
		if (symbol instanceof AbstractPin) {
			symbol.addTo(this);
		} else {
			this.#platina.add(symbol);
		}
		return this;
	}

	/**
	 * @method multiAdd(symbols: Array of GleoSymbol): this
	 * Adds the given `GleoSymbol`s to the appropriate acetate(s).
	 * @alternative
	 * @method multiAdd(symbols: Array of AbstractPin): this
	 * Adds the given pins (`HTMLPin`s, `Balloon`s) to the map.
	 *
	 * It's also possible to call `multiAdd` with an array containing both symbols
	 * and pins.
	 */
	multiAdd(symbols) {
		this.#platina?.multiAdd(symbols.filter((s) => !(s instanceof AbstractPin)));
		symbols.filter((s) => s instanceof AbstractPin).forEach((s) => s.addTo(this));
		return this;
	}

	/**
	 * @method remove(symbol: GleoSymbol): this
	 * Removes the symbol from this map.
	 * @alternative
	 * @method remove(pin: AbstractPin): this
	 * Removes the pin from this map
	 */
	remove(symbol) {
		if (symbol instanceof AbstractPin) {
			symbol.remove();
		} else {
			this.remove.add(symbol);
		}
		return this;
	}

	/**
	 * @method multiRemove(symbols: Array of GleoSymbol): this
	 * Removes several symbols from this map.
	 * @alternative
	 * @method multiRemove(pins: Array of AbstractPin): this
	 * Removes several pins from this map.
	 *
	 * It's also possible to call `multiRemove` with an array containing both
	 * symbols and pins.
	 */
	multiRemove(symbols) {
		this.#platina?.multiRemove(symbols.filter((s) => !(s instanceof AbstractPin)));
		symbols.filter((s) => s instanceof AbstractPin).forEach((s) => s.remove());
		return this;
	}

	/**
	 * @method has(symbol: GleoSymbol): Boolean
	 * Returns `true` if this map contains the given symbol, false otherwise.
	 * @alternative
	 * @method has(symbol: Loader): Boolean
	 * Returns `true` if this map contains the given loader, false otherwise.
	 */
	has(s) {
		return this.platina?.has(s);
	}
	/**
	 * @method addAcetate(ac: Acetate): this
	 * Adds a new `Acetate` to the map.
	 *
	 * Calling this method is usually not needed, as there are default acetates.
	 */
	addAcetate(ac) {
		this.#platina?.addAcetate(ac);
		ac._map = this;
		return this;
	}
}
