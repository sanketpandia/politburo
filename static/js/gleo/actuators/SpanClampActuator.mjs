import { registerActuator } from "../Map.mjs";

/**
 * @class SpanClampActuator
 * @inherits Actuator
 *
 * Span clamping actuator. Forces the values of the scale so that the span
 * is within the CRS's `minSpan`/`maxSpan` limits.
 *
 * Akin to an enforcer of `minZoom`/`maxZoom` (albeit Gleo doesn't have the
 * concept of min/max zoom).
 */

class SpanClampActuator {
	/**
	 * @constructor SpanClampActuator(map: GleoMap)
	 */
	constructor(map) {
		this.map = map;
		this.#boundFilter = this.spanClampFilterSetView.bind(this);

		/**
		 * @class GleoMap
		 * @section Interaction behaviour options
		 * @option minSpan: Number
		 * The minimum length, **in CRS units**, of the map "span". The user won't
		 * be able to zoom in so that the lenght of the diagonal is less than
		 * this value.
		 *
		 * This option depends on `SpanClampActuator` being loaded.
		 * @alternative
		 * @option minSpan: undefined = undefined
		 * Setting `minSpan` to `undefined` (or any falsy value) will make the
		 * `SpanClampActuator` use the CRS's `minSpan` default instead.
		 *
		 * This is the default.
		 * @option maxSpan: Number
		 * Akin to `minSpan`: prevents the user from zooming out so that the length
		 * of the diagonal is larger than this number.
		 * @alternative
		 * @option minSpan: undefined = undefined
		 * Akin to `minSpan`: then falsy, uses the CRS's `maxSpan` instead.
		 *
		 * This is the default.
		 * @section Interaction behaviour properties
		 * @property minSpan
		 * Runtime value for the `minSpan` initialization option.
		 *
		 * Updating its value will affect future zoom operations.
		 * @property maxSpan
		 * Akin to the `minSpan` property.
		 */

		map.minSpan ??= map.options.minSpan;
		map.maxSpan ??= map.options.maxSpan;
	}

	#boundFilter;

	enable() {
		this.map.registerSetViewFilter(this.#boundFilter);
	}

	disable() {
		this.map.unregisterSetViewFilter(this.#boundFilter);
	}

	spanClampFilterSetView({ scale, span, ...opts }) {
		if (scale === undefined && span === undefined) {
			return opts;
		}

		if (span === undefined) {
			// Using scale only
			return {
				...opts,
				scale: this.clampScale(scale),
			};
		} else {
			// Using span only
			return {
				...opts,
				span: this.clampSpan(span),
			};
		}
	}

	clampSpan(span) {
		const crs = this.map.platina.crs;
		const minSpan = this.map.minSpan ?? crs.minSpan;
		const maxSpan = this.map.maxSpan ?? crs.maxSpan;
		return Math.max(minSpan, Math.min(maxSpan, span));
	}

	clampScale(scale) {
		const crs = this.map.platina.crs;
		const minSpan = this.map.minSpan ?? crs.minSpan;
		const maxSpan = this.map.maxSpan ?? crs.maxSpan;

		const [w, h] = this.map.platina.pxSize;
		const diag = Math.sqrt(w * w + h * h);

		const span = scale * diag;
		if (span > maxSpan) {
			return maxSpan / diag;
		}
		if (span < minSpan) {
			return minSpan / diag;
		}
		return scale;
	}
}

registerActuator("spanclamp", SpanClampActuator, true);
