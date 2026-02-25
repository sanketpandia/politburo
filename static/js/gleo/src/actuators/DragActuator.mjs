import { registerActuator } from "../Map.mjs";
import Geometry from "../geometry/Geometry.mjs";
import css from "../dom/CSS.mjs";
import { getMousePosition } from "../dom/Dom.mjs";

css(`
.gleo > canvas.nodrag {
	touch-action: pinch-zoom;
}
`);

/**
 * @class DragActuator
 * @inherits Actuator
 *
 * Pointer drag actuator. This includes mouse drag, one-touch drag and box zoom.
 *
 * Dragging a pointer through the map canvas shall drag the map around.
 */

/// TODO: Hack this actuator so that the DOM position of the `GleoMap`'s `<canvas>`
/// is moved on `_onPointerMove`, then reset to zero on `_onPreRender`. This
/// *should* prevent some of the movement lag when dragging.

/// TODO: Disable the drag actuator if there's a second `pointerdown` event
/// without a `pointerup` first - meaning there's two (or more) fingers/pens
/// touching the surface.

class DragActuator {
	#boundDown;
	#boundUp;
	#boundMove;

	#modifier;

	#box;

	#downPointers = new Set();

	/**
	 * @constructor DragActuator(map: GleoMap)
	 */
	constructor(map) {
		this.map = map;
		this.platina = map.platina;

		/**
		 * @class GleoMap
		 * @section Interaction behaviour options
		 * @option boxZoomModifier: String = "shift"
		 * One of `"shift"`, `"control"`, `"alt"` or `"meta"`. Defines the
		 * modifier key that must be pressed during a map drag so it performs
		 * a box zoom instead.
		 * @alternative
		 * @option boxZoomModifier: Boolean
		 * Explicitly set to `false` to disable box zooming.
		 */

		this.#boundDown = this.#onPointerDown.bind(this);
		this.#boundUp = this.#onPointerUp.bind(this);
		this.#boundMove = this.#onPointerMove.bind(this);

		switch (map.boxZoomModifier) {
			case "control":
				this.#modifier = "ctrlKey";
				break;
			case "alt":
				this.#modifier = "altKey";
				break;
			case "meta":
				this.#modifier = "metaKey";
				break;
			case false:
				this.#modifier = false;
				break;
			case "shift":
			default:
				this.#modifier = "shiftKey";
		}

		if (this.#modifier) {
			this.#box = document.createElement("div");
			this.#box.style.border = "2px dotted #38f";
			this.#box.style.background = "rgba(255,255,255,0.5)";
			this.#box.style.position = "absolute";
			this.#box.style.pointerEvents = "none";
		}
	}

	/**
	 * @method enable(): this
	 * Enables this actuator. This will capture `pointerdown`, `pointerup` and
	 * `pointermove` (between `pointerdown` and `pointerup`) DOM events.
	 */
	enable() {
		this.platina.canvas.classList.add("nodrag");
		this.platina.addEventListener("pointerdown", this.#boundDown);
		this.platina.addEventListener("pointerup", this.#boundUp);
		this.platina.addEventListener("pointerout", this.#boundUp);
	}

	/**
	 * @method disable(): this
	 * Disables this actuator. Stops capturing `pointerdown`, `pointermove`, `pointerup`
	 * DOM events.
	 */
	disable() {
		this.platina.canvas.classList.remove("nodrag");
		this.platina.removeEventListener("pointerdown", this.#boundDown);
		this.platina.removeEventListener("pointerup", this.#boundUp);
		this.platina.removeEventListener("pointerout", this.#boundUp);
		this.platina.removeEventListener("pointermove", this.#boundMove);
	}

	#boxZooming = false;
	#baseX;
	#baseY;
	#lastX;
	#lastY;
	#sizeX;
	#sizeY;
	#onPointerDown(ev) {
		this.#downPointers.add(ev.pointerId);
		[this.#baseX, this.#baseY] = getMousePosition(ev, this.map.canvas);

		if (this.#downPointers.size === 1) {
			this.platina.addEventListener("pointermove", this.#boundMove);
		} else {
			this.platina.removeEventListener("pointermove", this.#boundMove);
		}
		this.platina.canvas.setPointerCapture(ev.pointerId);

		if (this.#modifier && ev[this.#modifier]) {
			this.#boxZooming = true;
			this.#box.style.left = this.#baseX + "px";
			this.#box.style.top = this.#baseY + "px";
			this.#box.style.width = "0px";
			this.#box.style.height = "0px";
			this.map.container.appendChild(this.#box);
		} else {
			this.#boxZooming = false;
		}
	}

	#onPointerUp(ev) {
		if (!this.#downPointers.has(ev.pointerId)) {
			// This happens when the browser fires both a `pointerout` and a
			// `pointerup` event. The second one shall be ignored.
			// Else, the #downPointers counter would go into the negatives.
			return;
		}

		this.platina.canvas.releasePointerCapture(ev.pointerId);

		this.#downPointers.delete(ev.pointerId);

		this.platina.removeEventListener("pointermove", this.#boundMove);

		if (this.#downPointers.size === 0 && this.#boxZooming) {
			this.map.container.removeChild(this.#box);

			const corner1 = this.platina.pxToGeom([this.#baseX, this.#baseY], false);
			const corner2 = this.platina.pxToGeom(
				[this.#baseX + this.#sizeX, this.#baseY + this.#sizeY],
				false
			);

			const [x1, y1, x2, y2] = [corner1.coords, corner2.coords].flat();
			const box = [
				Math.min(x1, x2),
				Math.min(y1, y2),
				Math.max(x1, x2),
				Math.max(y1, y2),
			];

			this.map.fitBounds(box);

			this.#boxZooming = false;
		}
	}

	#onPointerMove(ev) {
		[this.#lastX, this.#lastY] = getMousePosition(ev, this.map.canvas);

		if (this.#boxZooming) {
			this.#sizeX = this.#lastX - this.#baseX;
			this.#sizeY = this.#lastY - this.#baseY;

			this.#box.style.left =
				Math.min(this.#baseX, this.#baseX + this.#sizeX) + "px";
			this.#box.style.top = Math.min(this.#baseY, this.#baseY + this.#sizeY) + "px";
			this.#box.style.width = Math.abs(this.#sizeX) + "px";
			this.#box.style.height = Math.abs(this.#sizeY) + "px";
		} else {
			const pxX = this.#lastX - this.#baseX;
			const pxY = this.#lastY - this.#baseY;

			const scale = this.map.scale;
			const yaw = this.map.yawRadians;
			const cosYaw = Math.cos(yaw);
			const sinYaw = Math.sin(yaw);

			const crsX = (pxX * cosYaw - pxY * sinYaw) * scale;
			const crsY = (pxX * sinYaw + pxY * cosYaw) * scale;

			const center = this.map.center;
			this.map.setView({
				center: new Geometry(center.crs, [
					center.coords[0] - crsX,
					center.coords[1] + crsY,
				]),
				duration: 0,
			});

			this.#baseX = this.#lastX;
			this.#baseY = this.#lastY;
		}
	}
}

registerActuator("drag", DragActuator, true);
