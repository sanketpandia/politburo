import GleoSymbol from "./Symbol.mjs";
import { getMousePosition } from "../dom/Dom.mjs";

/**
 * @class ExtrudedPoint
 * @inherits GleoSymbol
 *
 * Abstract class, containing functionality common to symbols displayed as
 * triangles whose vertices are extruded from a central point, which
 * corresponds to the point `Geometry` of the symbol.
 *
 * i.e. `Sprite`, `CircleFill`, `CircleStroke`, and `Pie` so far.
 */

export default class ExtrudedPoint extends GleoSymbol {
	constructor(
		geom,
		{
			/**
			 * @option draggable: Boolean = false
			 * When set to `true`, the symbol can be dragged around with the pointer,
			 * and will fire `dragstart`/`drag`/`dragend` events.
			 */
			draggable = false,
			interactive = true,

			/**
			 * @option offset: Array of Number = [0,0]
			 * The amount of CSS pixels that the symbol will be offset from its
			 * geometry. The amount is up-right in `[x, y]` or `[up, right]` form.
			 */
			offset = [0, 0],

			...opts
		} = {}
	) {
		super(geom, { ...opts, interactive });

		this.#offset = offset;

		this._assertGeom(this.geometry);

		this.draggable = draggable;
	}

	#offset;

	get geometry() {
		return super.geometry;
	}
	set geometry(geom) {
		super.geometry = geom;
		if (this._inAcetate && this.attrBase !== undefined) {
			this._inAcetate.reproject(this.attrBase, this.attrLength);
			if ("dirty" in this._inAcetate) {
				this._inAcetate.dirty = true;
			}
		}
	}

	/**
	 * @property offset: Array of Number
	 * Getter/setter for the symbol's offset, as per the homonymous option.
	 */
	get offset() {
		return this.#offset;
	}
	set offset(offset) {
		this.#offset = offset;
		if (this._inAcetate && this.attrBase !== undefined) {
			const strideExtrusion = this._inAcetate._extrusions.asStridedArray();
			this._setStridedExtrusion(strideExtrusion);
			this._inAcetate._extrusions.commit(this.attrBase, this.attrLength);
			this._inAcetate.dirty = true;
		}
	}

	// Asserts that the geometry passed is a point geometry.
	_assertGeom(geom) {
		if (geom.coords.length !== geom.dimension) {
			/// TODO: Treat multi-point geometries as multipoints. See
			/// https://gitlab.com/IvanSanchez/gleo/-/issues/37
			throw new Error(
				"Geometry passed to ExtrudedPoint symbol is not a single point (is a line or polygon instead)."
			);
		}
	}

	#draggable = false;

	/**
	 * @section Subclass interface
	 * @uninheritable
	 * @method _setStridedExtrusion(strideExtrusion: StridedTypedArray): undefined
	 * Must be implemented by subclasses; must set values into the given
	 * `StridedTypedArray`.
	 *
	 * @method _setGlobalStrides(*): undefined
	 * Must be implemented by subclasses. Will receive a variable number of
	 * arguments depending on the Acetate implementaion: strided attribute arrays,
	 * strided index buffer, and constants. The implementation must fill the
	 * strided arrays appropriately.
	 */

	/**
	 * @section
	 * @property draggable: Boolean
	 * Whether the symbol can be dragged with pointer events. By (re)setting its
	 * value, dragging is enabled or disabled.
	 */
	/// TODO: Dragging behaviour with multipoint geometries is very tricky (for
	/// one, all instances of the symbol would share the same internal ID for
	/// pointer event target detection).
	/// For now, the dragging logic assumes single point geometry.
	get draggable() {
		return this.#draggable;
	}

	set draggable(d) {
		d = !!d;
		if (d === this.#draggable) {
			return;
		}
		if (d) {
			// enable
			this.#boundOnPointerDown ??= this.#onPointerDown.bind(this);
			this.#boundOnPointerMove ??= this.#onPointerMove.bind(this);
			this.#boundOnPointerUp ??= this.#onPointerUp.bind(this);

			this.on("pointerdown", this.#boundOnPointerDown);
			this.on("pointerup", this.#boundOnPointerUp);
			this.on("pointercancel", this.#boundOnPointerUp);
		} else {
			// disable
			this.off("pointerdown", this.#boundOnPointerDown);
			// this.off('pointermove', this.#boundOnPointerMove);
			this.off("pointerup", this.#boundOnPointerUp);
			this.off("pointercancel", this.#boundOnPointerUp);
		}

		this.#draggable = d;
	}

	#boundOnPointerDown;
	#boundOnPointerMove;
	#boundOnPointerUp;

	#pxDelta; // Delta between the symbol's geometry and the initial pointer event

	#onPointerDown(ev) {
		this.on("pointermove", this.#boundOnPointerMove);
		this.setPointerCapture(ev.pointerId);

		// Calculate the delta between the pointer hit point and the
		// symbol's geometry
		const geomPx = this._inAcetate._platina.geomToPx(this.geom);
		const evPx = getMousePosition(ev);
		this.#pxDelta = [evPx[0] - geomPx[0], evPx[1] - geomPx[1]];

		/**
		 * @section
		 * @event dragstart
		 * Fired whenever the user starts dragging a draggable `ExtrudedPoint`.
		 */
		this.fire("dragstart");

		ev.stopPropagation();
	}
	#onPointerMove(ev) {
		ev.stopPropagation();
		ev.preventDefault();

		const evPx = getMousePosition(ev);
		const geomPx = [evPx[0] - this.#pxDelta[0], evPx[1] - this.#pxDelta[1]];
		this.geometry = this._inAcetate._platina.pxToGeom(geomPx);
		/**
		 * @event drag
		 * Fired whenever the user keeps dragging a draggable `ExtrudedPoint`.
		 */
		this.fire("drag");
	}
	#onPointerUp(ev) {
		this.off("pointermove", this.#boundOnPointerMove);
		ev.stopPropagation();
		/**
		 * @event dragend
		 * Fired whenever the user stops dragging a draggable `ExtrudedPoint`.
		 */
		this.fire("dragend");
	}
}
