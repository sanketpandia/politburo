import { registerActuator } from "../Map.mjs";
import Geometry from "../geometry/Geometry.mjs";
import css from "../dom/CSS.mjs";

css(`
.gleo > canvas.nopinch {
	touch-action: pan-x pan-y;
}
.gleo > canvas.nodrag.nopinch {
	touch-action: none;
}
`);

/**
 * @class PinchActuator
 * @inherits Actuator
 *
 * Pointer pinch actuator, for two-finger zoom and rotation.
 */

class PinchActuator {
	#boundDown;
	#boundUp;
	#boundMove;

	/**
	 * @constructor PinchActuator(map: GleoMap)
	 */
	constructor(map) {
		this.map = map;
		this.platina = map.platina;

		this.#boundDown = this.#onPointerDown.bind(this);
		this.#boundUp = this.#onPointerUp.bind(this);
		this.#boundMove = this.#onPointerMove.bind(this);
	}

	/**
	 * @method enable(): this
	 * Enables this actuator. This will capture `pointerdown`, `pointerup` and
	 * `pointermove` (between `pointerdown` and `pointerup`) DOM events.
	 */
	enable() {
		this.platina.canvas.classList.add("nopinch");
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
		this.platina.canvas.classList.remove("nopinch");
		this.platina.removeEventListener("pointerdown", this.#boundDown);
		this.platina.removeEventListener("pointerup", this.#boundUp);
		this.platina.removeEventListener("pointerout", this.#boundUp);
		this.platina.removeEventListener("pointermove", this.#boundMove);
		this.#downPointers = 0;
	}

	// The algorithm revolves around keeping two sets of data for each of the
	// two (active) pointers: the screen/canvas XY position (which changes
	// with each movement), and the CRS position (the coordinates of the
	// event geometry, which keep the same during the entire pinch)
	#screenPositions = []; // In device pixels
	#crsPositions = [];
	#downPointers = 0;

	// There's a need to store the CRS when the pinch started, in case the CRS
	// changes during the pinch, to make the appropriate convertions.
	// It is assumed that the CRS will not change inbetween the first and
	// second pointer events.
	#crs;

	// `true` when the pinch's yaw is still under the threshold for yaw interaction
	// (which is 10°)
	#snapYaw = true;

	// Value of map's yaw when the pinch started, in radians
	#startYaw;

	#id1;
	#id2;

	// The relative angle between the two `#crsPositions`
	#crsTheta = 0;

	// The relative offset between the two `#screenPositions`
	#crsDelta = [];
	#crsDeltaSquareLength = 0;

	#onPointerDown(ev) {
		const id = ev.pointerId;
		const dpr = devicePixelRatio ?? 1;

		this.#downPointers++;

		this.#screenPositions[id] = [dpr * ev.canvasX, dpr * ev.canvasY];
		this.#crsPositions[id] = ev.geometry.coords;

		if (this.#downPointers === 1) {
			this.#crs = ev.geometry.crs;
			this.#id1 = id;
		} else if (this.#downPointers === 2) {
			this.#id2 = id;
			this.#snapYaw = true;
			this.#startYaw = this.map.yawRadians;

			const a = this.#crsPositions[this.#id1];
			const b = this.#crsPositions[this.#id2];

			this.#crsDelta = [b[0] - a[0], b[1] - a[1]];
			this.#crsTheta = Math.atan2(this.#crsDelta[1], this.#crsDelta[0]);
			this.#crsDeltaSquareLength = this.#crsDelta[0] ** 2 + this.#crsDelta[1] ** 2;

			this.platina.addEventListener("pointermove", this.#boundMove);
		}

		// Capture the pointer, unless there's a drag actuator (assume the drag
		// actuator already captured the pointer)
		if (!this.map.actuators.get("drag")) {
			this.platina.canvas.setPointerCapture(ev.pointerId);
		}
	}

	#onPointerUp(ev) {
		const id = ev.pointerId;

		if (!this.#screenPositions[id]) {
			return;
		}

		delete this.#screenPositions[id];
		delete this.#crsPositions[id];

		this.#downPointers--;

		if (this.#downPointers === 1) {
			this.map.setView({
				scale: this.map.scale,
				zoomSnap: true,
				yawDegrees: this.map.yawDegrees,
				yawSnap: true,
			});
			this.platina.removeEventListener("pointermove", this.#boundMove);

			// Reset the values of this.#id1 and this.#id2 - making sure
			// that the pointer ID currently down is at this.#id1
			if (id === this.#id1) {
				this.#id1 = this.#id2;
			}
			this.#id2 = undefined;
		}

		// Release pointer capture; same logic as on pointerdown.
		if (!this.map.actuators.get("drag")) {
			this.platina.canvas.releasePointerCapture(ev.pointerId);
		}
	}

	#onPointerMove(ev) {
		const id = ev.pointerId;
		const dpr = devicePixelRatio ?? 1;
		this.#screenPositions[id] = [dpr * ev.canvasX, dpr * ev.canvasY];

		// Ideally I'd like to solve this with some matrix equations - given
		// the transformation matrix and the known data (CRS coordinates,
		// expected clipspace coordinates), solve for the unknown data (center,
		// scale, yaw angle).

		// But, in the end, it's simpler to do this heuristically.

		const a = this.#screenPositions[this.#id1];
		const b = this.#screenPositions[this.#id2];

		const screenDelta = [b[0] - a[0], b[1] - a[1]];
		const screenTheta = Math.atan2(screenDelta[1], screenDelta[0]);

		const theta = -this.#crsTheta - screenTheta;
		const scale = Math.sqrt(
			this.#crsDeltaSquareLength / (screenDelta[0] ** 2 + screenDelta[1] ** 2)
		);

		/// Screen vector from the 1st known point to the center
		const [w, h] = this.platina.pxSize;

		const svX = w / 2 - a[0];
		const svY = a[1] - h / 2; // Screen Y is downwards, the calculations
		// will need upwards.

		// Rotate by negative theta
		const sinTheta = Math.sin(-theta);
		const cosTheta = Math.cos(-theta);
		const srX = svX * cosTheta - svY * sinTheta;
		const srY = svX * sinTheta + svY * cosTheta;

		// Multiply by scale - units are now CRS' units
		const crsDeltaX = srX * scale;
		const crsDeltaY = srY * scale;

		// Add the CRS delta to the 1st point's CRS coordinates
		const [crsX, crsY] = this.#crsPositions[this.#id1];
		const centerX = crsX + crsDeltaX;
		const centerY = crsY + crsDeltaY;

		if (Math.abs(theta - this.#startYaw) > 0.175) {
			// If yaw change greater than 0.175 radians (≃ π/18 = 10 degrees),
			// stop snapping yaw.
			this.#snapYaw = false;
		}

		this.map.setView({
			center: new Geometry(this.#crs, [centerX, centerY]),
			scale,
			yawRadians: this.#snapYaw ? this.#startYaw : theta,
			zoomSnap: false,
			yawSnap: false,
			duration: 0,
		});
	}
}

registerActuator("pinch", PinchActuator, true);
