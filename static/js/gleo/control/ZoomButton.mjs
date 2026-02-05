import Button from "./Button.mjs";

/**
 * @class ZoomButton
 * @inherits Button
 * Common "Zoom In"/"Zoom Out" button functionality.
 */

export default class ZoomButton extends Button {
	constructor(opts) {
		super(opts);

		this._boundOnPointerDown = this._onPointerDown.bind(this);
		this._boundOnPointerUp = this._onPointerUp.bind(this);
		this._boundOnFrame = this._onFrame.bind(this);
		this.animFrame = undefined;
		this.lastTimestamp = undefined;
		this.initialScale = undefined;
	}

	addTo(map) {
		super.addTo(map);
		this.on("pointerdown", this._boundOnPointerDown);
		this.on("pointerup", this._boundOnPointerUp);
		this.on("pointercancel", this._boundOnPointerUp);
		// 		this.on('pointerleave', this._boundOnPointerUp);
	}

	remove() {
		this.off("pointerdown", this._boundOnPointerDown);
		this.off("pointerup", this._boundOnPointerUp);
		this.off("pointercancel", this._boundOnPointerUp);
		// 		this.off('pointerleave', this._boundOnPointerUp);
	}

	_onPointerDown(ev) {
		this.lastTimestamp = performance.now();
		this.initialScale = this._map.scale;
		this.animFrame = window.requestAnimationFrame(this._boundOnFrame);
		this.element.setPointerCapture(ev.pointerId);
	}
	_onPointerUp(ev) {
		const millisecs = Math.max(performance.now() - this.lastTimestamp, 500);
		const factor = Math.pow(this.scaleFactorPerSecond, millisecs / 1000);
		this._map.setView({
			scale: this.initialScale * factor,
			duration: Math.max(1000 - millisecs, 100),
			center: this._map.center,
		});
		window.cancelAnimationFrame(this.animFrame);
		this.element.releasePointerCapture(ev.pointerId);
	}
	_onFrame() {
		const now = performance.now();
		const secs = (now - this.lastTimestamp) / 1000;

		const factor = Math.pow(this.scaleFactorPerSecond, secs);

		this._map.setView({
			scale: this.initialScale * factor,
			duration: 100,
			zoomSnap: false,
		});
		this.animFrame = window.requestAnimationFrame(this._boundOnFrame);
	}
}
