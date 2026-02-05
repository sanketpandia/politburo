import { registerActuator } from "../Map.mjs";
import Geometry from "../geometry/Geometry.mjs";
import { getMousePosition } from "../dom/Dom.mjs";
import { invert, transpose } from "../3rd-party/gl-matrix/mat3.mjs";
import { transformMat3 } from "../3rd-party/gl-matrix/vec3.mjs";

/**
 * @class WheelActuator
 * @inherits Actuator
 *
 * Mouse wheel actuator. Scrolling the mouse wheel shall zoom the map in/out.
 *
 * TODO: Fix interaction with SpanClampActuator
 */

class WheelActuator {
	#boundWheel;
	#boundPreRender;

	/**
	 * @constructor WheelActuator(map: GleoMap)
	 */
	constructor(map) {
		this.map = map;
		this.canvas = map.canvas;

		this.#boundWheel = this.#onWheel.bind(this);
		this.#boundPreRender = this.#onPreRender.bind(this);

		/**
		 * @class GleoMap
		 * @section Interaction behaviour options
		 * @option wheelPxPerZoomLog2: Number = 60
		 * How many scroll pixels mean a change in the scale by a factor of 2.
		 * Smaller values will make wheel-zooming faster, and vice versa. The
		 * default value of 60 means that one "step" on a standard mousewheel
		 * should change the scale by a factor of 2.
		 *
		 * This option depends on `WheelActuator` being loaded.
		 */
		map.wheelPxPerZoomLog2 ??= map.options.wheelPxPerZoomLog2 ?? 60;

		/**
		 * @option wheelZoomDuration: Number = 200
		 * Duration, in milliseconds, of the mousewheel zoom animation.
		 *
		 * This option depends on `WheelActuator` and `InertiaActuator` being loaded.
		 */
		map.wheelZoomDuration ??= map.options.wheelZoomDuration ?? 200;

		/**
		 * @section Interaction behaviour properties
		 * @property wheelPxPerLog2: Number
		 * Runtime value of the `wheelPxPerLog2` initialization option.
		 *
		 * Updating its value will affect future scrollwheel zoom operations.
		 * @property wheelZoomDuration: Number
		 * Runtime value of the `wheelZoomDuration` initialization option.
		 *
		 * Updating its value will affect future scrollwheel zoom operations.
		 */

		this.resetTargetScaleTimeout = undefined;
	}

	enable() {
		this.canvas.addEventListener("wheel", this.#boundWheel);
	}

	disable() {
		this.canvas.removeEventListener("wheel", this.#boundWheel);
	}

	#onWheel(ev) {
		ev.preventDefault();

		const dpr = devicePixelRatio ?? 1;
		const currentScale = this.map.scale;
		if (!this._targetScale) {
			this._targetScale = currentScale;
		}

		let [canvasX, canvasY] = getMousePosition(ev, this.map.canvas);
		this._targetCenter = this.map.center;

		const delta = getWheelDelta(ev);
		this._targetScale *= Math.pow(2, delta / this.map.wheelPxPerZoomLog2);

		const snapActuator = this.map.actuators.get("zoomsnap");
		const spanClampActuator = this.map.actuators.get("spanclamp");
		this._snappedTargetScale = this._targetScale;
		if (snapActuator && this.map.options.zoomSnap !== false) {
			this._snappedTargetScale = snapActuator.snapScale(this._targetScale);
		}
		if (spanClampActuator && this.map.options.zoomSnap !== false) {
			this._snappedTargetScale = spanClampActuator.clampScale(
				this._snappedTargetScale
			);
		}

		// Bits from GleoMap's pxToGeom
		// transform the wheel device-pixel coordinates into clipspace
		const [w, h] = this.map.platina.deviceSize;
		let clipX = (dpr * canvasX * 2) / w - 1;
		let clipY = (dpr * canvasY * -2) / h + 1;

		const scaleFactor = this._snappedTargetScale / currentScale;
		// 		const scaleFactor = Math.log2(
		// 			Math.pow(2,this._targetScale)
		// 			/
		// 			Math.pow(2,currentScale)
		// 		);

		clipX -= clipX * scaleFactor;
		clipY -= clipY * scaleFactor;

		const vec = [clipX, clipY, 1];
		const invMatrix = invert(new Array(9), this.map.platina._crsMatrix);
		transpose(invMatrix, invMatrix);
		transformMat3(vec, vec, invMatrix);

		this._targetCenter = new Geometry(this.map.center.crs, [vec[0], vec[1]], {
			wrap: false,
		});

		const boundsClampActuator = this.map.actuators.get("boundsclamp");
		if (boundsClampActuator) {
			this._targetCenter = boundsClampActuator.clampView(
				this._targetCenter,
				this._targetScale,
				this.map.platina.deviceSize
			);
		}

		// 		this._targetCenter.coords[0] += geom.coords[0];
		// 		this._targetCenter.coords[1] += geom.coords[1];
		// 		const targetY = center.coords[1] - geom.coords[1];

		///TODO: Do the change around the pointer position (as reported by the
		/// `wheel` event), instead of around the center point.
		// 		this._baseX = ev.clientX;
		// 		this._baseY = ev.clientY;

		this.map.platina.addEventListener("prerender", this.#boundPreRender);

		// The following assumes that no call to requestAnimationFrame()
		// will take more than ~400 millisecs.
		clearTimeout(this.resetTargetScaleTimeout);
		this.resetTargetScaleTimeout = setTimeout(
			this._resetTargetScale.bind(this),
			this.map.wheelZoomDuration + 200
		);
	}

	#onPreRender(ev) {
		//console.log(ev);
		// Event happens only once, so prevent running multiple times
		this.map.platina.removeEventListener("prerender", this.#boundPreRender);
		// 		const offsetX = this._lastX - this._baseX;
		// 		const offsetY = this._lastY - this._baseY;

		// 		console.log("prerender wheelzoom to", this._targetScale);

		if (this._snappedTargetScale !== this._lastSnappedScale) {
			this.map.setView({
				// this.map.center,
				center: this._targetCenter,
				scale: this._snappedTargetScale,
				duration: this.map.wheelZoomDuration,
				zoomSnap: false,
			});
		}

		this._lastSnappedScale = this._snappedTargetScale;
	}

	_resetTargetScale() {
		if (this.map.actuators.has("inertia")) {
			this.map.setView({
				scale: this._targetScale,
				zoomSnap: true,
			});
		}
		delete this._targetScale;
	}
}

// // Chrome on Win scrolls double the pixels as in other platforms (see Leaflet bug #4538),
// // and Firefox scrolls device pixels, not CSS pixels
// var wheelPxFactor =
// 	(Browser.win && Browser.chrome) ? 2 * window.devicePixelRatio :
// 	Browser.gecko ? window.devicePixelRatio : 1;

const wheelPxFactor = devicePixelRatio ?? 1;

// Aux, straight from Leaflet's DomEvent code.
function getWheelDelta(ev) {
	if (ev.deltaMode === 0) {
		// pixels
		return ev.deltaY / wheelPxFactor;
	} else if (ev.deltaMode === 1) {
		// lines
		return ev.deltaY * 10;
	} else if (ev.deltaMode === 2) {
		// pages
		return ev.deltaY * 60;
	}

	return 0;
}

registerActuator("wheel", WheelActuator, true);
