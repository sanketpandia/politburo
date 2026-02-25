import css from "../dom/CSS.mjs";
import Control from "./Control.mjs";

const invSqrt2 = 0.5 / Math.sqrt(2);

css(`
.gleo-control-scalebar {
	background: #ccc;
	padding: 0.25em;
}

.gleo-control-scalebar .gleo-scale {
	border-bottom: 0.2em solid black;
	border-left: 0.2em solid black;
	border-right: 0.2em solid black;
	text-align: center;
	box-sizing: border-box;
}
`);

/**
 * @class ScaleBar
 * @inherits Control
 * An informative scale bar control.
 */
export default class ScaleBar extends Control {
	/**
	 * @constructor ScaleBar(opts: Scalebar Options)
	 */
	constructor({
		/**
		 * @section Scalebar Options
		 * @option maxSize: Number = 100
		 * Maximum size, in CSS pixels, of the scalebar line.
		 */
		maxSize = 200,
		position = "bl",
		...opts
	} = {}) {
		super({ position, ...opts });

		/// TODO: init options:
		/// - units of measurement (meters, nautical, unitless, etc)

		this._boundOnViewChange = this.onViewChange.bind(this);
		this._maxSize = maxSize;
	}

	spawnElement() {
		this.element = document.createElement("div");
		this.element.className = "gleo-control gleo-control-scalebar";

		this._scaleElement = document.createElement("div");
		this._scaleElement.className = "gleo-scale";
		this.element.appendChild(this._scaleElement);
	}

	addTo(map) {
		super.addTo(map);
		map.on("viewchanged", this._boundOnViewChange);
	}

	remove() {
		super.remove();
		this._map.off("viewchanged", this._boundOnViewChange);
	}

	onViewChange(ev) {
		const p = this._map.platina;
		const [w, h] = p.pxSize;
		const w2 = w / 2,
			h2 = h / 2;

		// The idea is to measure not the distance from a pixel to the next,
		// but rather the length of a line as long as the maximum scalebar size,
		// around the platina's center.
		// This is a best-effort approach to providing a reliable measure at
		// low scales. There'll be artifacts with specific projections at yaw 45°,
		// but hopefully won't be much of a problem.
		const offset = this._maxSize * invSqrt2;
		const geom1 = p.pxToGeom([w2 - offset, h2 - offset]);
		const geom2 = p.pxToGeom([w2 + offset, h2 + offset]);

		let pxDistance = p.crs.distance(geom1, geom2);

		let unit = "m";
		if (pxDistance >= 1e3) {
			unit = "km";
			pxDistance /= 1e3;
		}

		let clampedDistance = Math.pow(10, Math.floor(Math.log10(pxDistance)));
		if (clampedDistance * 5 < pxDistance) {
			clampedDistance *= 5;
		} else if (clampedDistance * 2 < pxDistance) {
			clampedDistance *= 2;
		}

		if (Number.isFinite(clampedDistance)) {
			this._scaleElement.innerText = `${clampedDistance}${unit}`;
			this._scaleElement.style.width =
				(this._maxSize * clampedDistance) / pxDistance + "px";
		} else {
			this._scaleElement.innerText = `N/A`;
			this._scaleElement.style.width = this._maxSize + "px";
		}
	}
}
