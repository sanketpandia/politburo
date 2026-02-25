import { registerActuator } from "../Map.mjs";

/**
 * @class ZoomYawSnapActuator
 * @inherits Actuator
 *
 * Zoom & yaw snap actuator.
 *
 * Zoom snap works whenever there's raster stuff in the map
 * (`TileLoader`s, `ConformalRaster`s, etc) in the map: it will snap the scale
 * so that it matches that of the raster (when the scales are close enough)
 *
 * Yaw snap acts whenever the yaw rotation angle is too close to the target
 * angle. Its main purpose is to lock the
 */

class ZoomYawSnapActuator {
	/**
	 * @constructor ZoomYawSnapActuator(map: GleoMap)
	 */
	constructor(map) {
		this.map = map;

		/**
		 * @class GleoMap
		 * @section Interaction behaviour options
		 * @option zoomSnapFactor: Number = 0.5
		 * Whether the map's scale will snap to the native scale of raster symbols
		 * (`ConformalRaster`s and `RasterTileLoader`s) in the map.
		 *
		 * The value is the snap threshold, expressed in terms of the difference between
		 * the base-2 logarithms of the requested scale and the raster's scale.
		 *
		 * For a tile pyramid with power-of-two scales per level (i.e. the
		 * scale of a level is double the scale of the previous level and half
		 * of the next level), the default threshold value of of `0.5` will
		 * always snap between pyramid levels.
		 *
		 * This option depends on `ZoomYawSnapActuator`.
		 *
		 * @option yawSnapTarget: Number = 0
		 * The target yaw snap angle (in decimal degrees, clockwise). The yaw
		 * snap logic will only trigger when the yaw is set to a value close
		 * to this target.
		 *
		 * @option yawSnapPeriod: Number = 90
		 * When set to a finite value less than 360, allows for multiple values
		 * of the snap target, separated by this value. The default means that
		 * the yaw will snap to either `0`, `90`, `180` or `270` degrees, if the
		 * requested yaw is close to any of these values.
		 *
		 * @option yawSnapTolerance: Number = 10
		 * The maximum difference (in decimal degrees) between the requested
		 * yaw and the target yaw to trigger the snap logic.
		 *
		 * @option zoomSnapOnlyOnYawSnap: Boolean = false
		 * By default, zoom snaps occur regardless of the yaw. When this is
		 * set to `true`, zoom snaps will only happen when the yaw is snapped.
		 *
		 * @section Interaction behaviour properties
		 * @property zoonSnapFactor
		 * Runtime value for the `zoomSpanFactor` option. Updating its value will affect future zoom operations.
		 * @property yawSnapTarget
		 * Runtime value for the `yawSnapTarget` option. Updating its value will
		 * affect future yaw operations.
		 * @property yawSnapPeriod
		 * Runtime value for the `yawSnapTarget` option. Updating its value will
		 * affect future yaw operations.
		 * @property yawSnapTolerance
		 * Runtime value for the `yawSnapTarget` option. Updating its value will
		 * affect future yaw operations.
		 * @property zoomSnapOnlyOnYawSnap
		 * Runtime value for the `yawSnapTarget` option. Updating its value will
		 * affect future yaw operations.
		 */
		map.zoomSnapFactor ??= map.options.zoomSnapFactor ?? 0.5;
		map.yawSnapTarget ??= map.options.yawSpanTarget ?? 0;
		map.yawSnapPeriod ??= map.options.yawSnapPeriod ?? 90;
		map.yawSnapTolerance ??= map.options.yawSnapTolerance ?? 10;
		map.zoomSnapOnlyOnYawSnap ??= map.options.zoomSnapOnlyOnYawSnap ?? false;

		this.boundFilter = this.zoomSnapFilterSetView.bind(this);
	}

	enable() {
		this.map.registerSetViewFilter(this.boundFilter);
	}

	disable() {
		this.map.unregisterSetViewFilter(this.boundFilter);
	}

	zoomSnapFilterSetView({
		yawDegrees,
		yawRadians,
		zoomSnap = true,
		yawSnap = true,
		...opts
	}) {
		const map = this.map;
		/**
		 * @miniclass SetView Options (Platina)
		 * @option zoomSnap: Boolean = true
		 * When explicitly set to `false`, the zoom snapping logic is disabled:
		 * the scale level of the map will *not* snap to the raster data for
		 * that `setView` call.
		 * @option yawSnap: Boolean = true
		 * When explicitly set to `false`, the yaw snap logic is disabled
		 * for that `setView` call.
		 */
		if (yawDegrees === undefined && yawRadians !== undefined) {
			yawDegrees = (-yawRadians * 180) / Math.PI;
		}

		if (yawDegrees !== undefined && yawSnap) {
			let snapped = false;

			const p = map.yawSnapPeriod;
			let y = yawDegrees;
			// As per https://developer.mozilla.org/en-US/docs/Web/JavaScript/Reference/Operators/Remainder
			y = ((yawDegrees % p) + p) % p;

			if (y < map.yawSnapTolerance) {
				snapped = true;
				yawDegrees -= y;
			} else if (y + map.yawSnapTolerance > p) {
				snapped = true;
				yawDegrees += p - y;
			}

			if (!snapped && map.zoomSnapOnlyOnYawSnap) {
				return { yawDegrees, yawRadians, ...opts };
			}
		}

		if (!zoomSnap) {
			return { yawDegrees, ...opts };
		} else if (opts.scale !== undefined) {
			return {
				...opts,
				scale: this.snapScale(opts.scale),
				yawDegrees,
			};
		} else if (opts.span !== undefined) {
			const [w, h] = this.map.platina.pxSize;
			const diag = Math.sqrt(w * w + h * h);

			return {
				...opts,
				span: undefined,
				scale: this.snapScale(opts.span / diag),
				yawDegrees,
			};
		} else {
			return { yawDegrees, ...opts };
		}
	}

	/**
	 * @class ZoomYawSnapActuator
	 * @section Scale and pixel fidelity methods
	 * @method snapScale(scale: Number): Number
	 *
	 * Runs the scale snap logic on the given value: returns the nearest scale snap point,
	 * if the given scale is within the `zoomSnapFactor` tolerance.
	 */
	snapScale(scale) {
		const [w, h] = this.map.platina.pxSize;
		const diag = Math.sqrt(w * w + h * h);
		const minSpan = this.map.minSpan ?? this.map.crs.minSpan;
		const maxSpan = this.map.maxSpan ?? this.map.crs.maxSpan;

		const stops = this.map.platina.getScaleStops(this.map.platina.crs.name);

		const scaleLog = Math.log2(scale);
		// Log2 between the (so-far) closest scale and the target one.
		let closestLog2 = Infinity;
		let closestScale;

		stops.forEach((stop) => {
			const stopSpan = stop * diag;
			if (minSpan && stopSpan < minSpan) {
				return;
			}
			if (maxSpan && stopSpan > maxSpan) {
				return;
			}

			const deltaLog = Math.abs(scaleLog - Math.log2(stop));
			if (deltaLog <= this.map.zoomSnapFactor && deltaLog < closestLog2) {
				closestLog2 = deltaLog;
				closestScale = stop;
			}
		});

		return closestScale !== undefined ? closestScale : scale;
	}
}

registerActuator("zoomsnap", ZoomYawSnapActuator, true);
