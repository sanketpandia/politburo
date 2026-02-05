/**
 * @namespace intervalify
 * @inherits Symbol Decorator
 *
 * Converts a `GleoSymbol` into a symbol that can be filtered by a value in an interval.
 *
 * Each symbol gains two properties for the interval start and end values; the
 * corresponding `Acetate` gains interval start and end as well. A symbol will
 * be rendered only if its intervall falls within (or intersects with) the
 * acetate's interval.
 *
 * The acetate's interval can be efficiently reset at runtime, allowing dynamic
 * filtering of the symbols.
 *
 * @example
 *
 * A typical use case is filtering data by a time interval. This will take
 * the `Sprite` class as a base and add intervals to it.
 *
 * ```
 * import intervalify from 'gleo/src/symbols/intervalify.mjs';
 *
 * const IntervalSprite = intervalify(Sprite, "within");	// "Intervalified" `Sprite` class
 * ```
 *
 * Then, instantiate these `GleoSymbol`s as desired. The interval values must be
 * `Number`s, so unix timestamps are used. Beware of using
 * [`Date.parse()`](https://developer.mozilla.org/en-US/docs/Web/JavaScript/Reference/Global_Objects/Date/parse)
 * with anything other than [date-time strings formatted in ISO 8601](https://tc39.es/ecma262/#sec-date-time-string-format)
 * (`YYYY-MM-DD` or `YYYY-MM-DDTHH:mm:ss.sssZ`).
 *
 * ```
 * const sprite1 = new IntervalSprite(geom1, {
 * 	intervalStart: Date.parse("1950-05-30"),
 * 	intervalEnd: Date.parse("1995-11-20"),
 * 	...spriteOptions
 * };
 * const sprite2 = new IntervalSprite(geom2, {
 * 	intervalStart: Date.parse("1995-11-21"),
 * 	intervalEnd: Infinity,
 * 	...spriteOptions
 * };
 * ```
 *
 * These symbols need to be drawn in their own `Acetate` (and this `Acetate` needs
 * to be created linked to a `GleoMap` or a `Platina`). Optionally pass a mode
 * ("within"/"intersect") and starting values for the interval to be shown:
 *
 * ```
 * const myIntervalAcetate = new IntervalSprite.Acetate(map, {mode = "within"});
 *
 * myIntervalAcetate.multiAdd([sprite1, sprite2]);
 * ```
 *
 * Finally, set the `intervalStart` and `intervalEnd` properties of the
 * `Acetate` in order to dynamically filter which symbols are drawn:
 *
 * ```
 * myIntervalAcetate.intervalStart = Date.parse("1975-05-30");
 * myIntervalAcetate.intervalEnd = Date.parse("1975-05-30");
 * ```
 *
 * @miniclass Intervalified GleoSymbol (intervalify)
 * @section
 * The return value of the `intervalify` function is a `GleoSymbol` class. It
 * will behave as the base class used, but with two extra constructor options:
 *
 * @option intervalStart: Number
 * The start (minimum value) of the symbol's numeric interval.
 * @option intervalEnd: Number
 * The end (maximum value) of the symbol's numeric interval.
 *
 * @miniclass Intervalified Acetate (intervalify)
 * @section
 *
 * Interval-enabled symbols need their own acetate to be drawn into - the
 * example code above shows how to access it from an intervalified symbol
 * class, and how to instantiate and add it to a map/platina.
 *
 * An intervalified `Acetate` gains two new read/write properties. Setting
 * a new value to any of these will trigger a re-render.
 *
 * @option intervalStart: Number = -Infinity
 * The initial value for the acetate's interval start.
 *
 * @option intervalEnd: Number = Infinity
 * The initial value for the acetate's interval end.
 *
 * @option opacityStart: Number = 1
 * The initial value for opacity of symbols whose interval is next to the
 * acetate's interval start.
 *
 * @option opacityEnd: Number = 1
 * The initial value for opacity of symbols whose interval is next to the
 * acetate's interval end.
 *
 * @option mode: String = "within"
 * Controls the overlap behaviour between the symbols' intervals and the acetate's
 * intervals. Can take two values:
 *
 * - When `mode` is `within`, then a symbol will be shown if its interval is
 *   completely within the acetate's interval. This is the default.
 * - When `mode` is `intersect`, then a symbol will be shown if its interval
 *   intersects with the acetate's interval.
 *
 * @property intervalStart: Number
 * The start (minimum value) of the acetates's numeric interval. Can be updated
 * at runtime.
 * @property intervalEnd: Number
 * The end (maximum value) of the acetate's numeric interval. Can be updated
 * at runtime.
 *
 * @property opacityStart: Number
 * The opacity of symbols whose interval is next to the acetate's interval start.
 * Can be updated at runtime.
 * @property opacityEnd: Number
 * The opacity of symbols whose interval is next to the acetate's interval start.
 * Can be updated at runtime.
 *
 * @namespace intervalify
 * @function intervalify(base: GleoSymbol, mode: String): GleoSymbol
 *
 * "Intervalifies" the given `GleoSymbol` class, i.e. returns a new `GleoSymbol`
 * class that behaves like the original, but can contain information about a
 * numeric interval. The corresponing `Acetate` also defines a numeric interval,
 * and will filter out symbols outside whose interval is outside of the acetate's
 * own interval.
 *
 */

export default function intervalify(base) {
	class IntervalifiedAcetate extends base.Acetate {
		#mode;

		constructor(
			target,
			{
				intervalStart = -Infinity,
				intervalEnd = Infinity,
				opacityStart = 1,
				opacityEnd = 1,
				mode = "within",
				...opts
			} = {}
		) {
			super(target, opts);

			this._intervalAttr = new this.glii.SingleAttribute({
				usage: this.glii.STATIC_DRAW,
				size: 1,
				growFactor: 1.2,

				// Start value in `.x`, end value in `.y`
				glslType: "vec2",
				type: Float32Array,
				normalized: false,
			});

			this.intervalStart = intervalStart;
			this.intervalEnd = intervalEnd;

			this.opacityStart = opacityStart;
			this.opacityEnd = opacityEnd;

			this.#mode = mode;
		}

		glProgramDefinition() {
			const opts = super.glProgramDefinition();

			// See https://scicomp.stackexchange.com/questions/26258/the-easiest-way-to-find-intersection-of-two-intervals
			const condition =
				this.#mode === "intersect"
					? "aInterval.y >= uInterval.x && uInterval.y >= aInterval.x"
					: "aInterval.x > uInterval.x && aInterval.y < uInterval.y";

			return {
				...opts,
				attributes: {
					aInterval: this._intervalAttr,
					...opts.attributes,
				},
				uniforms: {
					uInterval: "vec3", // Start value in `.x`, end value in `.y`, delta in `.z`
					uIntervalOpacity: "vec2",
					...opts.uniforms,
				},
				varyings: {
					vIntervalOpacity: "float",
					...opts.varyings,
				},
				vertexShaderMain: `
				if (${condition}) {
					if (uInterval.z != 0.0) {
						float intervalMidPosition =
							(((aInterval.x + aInterval.y) / 2.0) - uInterval.x) / uInterval.z;

						vIntervalOpacity = mix(uIntervalOpacity.x, uIntervalOpacity.y, intervalMidPosition);
					} else {
						vIntervalOpacity = 1.0;
					}

					${opts.vertexShaderMain}
				}`,
				fragmentShaderMain: `${opts.fragmentShaderMain}
				gl_FragColor.a *= vIntervalOpacity;
				`,
			};
		}

		_getStridedArrays(maxVtx, maxIdx) {
			return [
				// min/max interval values for vertex
				this._intervalAttr.asStridedArray(maxVtx),
				// Parent strided arrays
				...super._getStridedArrays(maxVtx, maxIdx),
			];
		}

		_commitStridedArrays(baseVtx, vtxCount) {
			this._intervalAttr.commit(baseVtx, vtxCount);
			return super._commitStridedArrays(baseVtx, vtxCount);
		}

		#intervalStart;
		#intervalEnd;
		#opacityStart;
		#opacityEnd;
		#dirtyIntervals = false;
		get intervalStart() {
			return this.#intervalStart;
		}
		get intervalEnd() {
			return this.#intervalEnd;
		}
		get opacityStart() {
			return this.#opacityStart;
		}
		get opacityEnd() {
			return this.#opacityEnd;
		}
		set intervalStart(i) {
			this.#intervalStart = i;
			this.dirty = this.#dirtyIntervals = true;
		}
		set intervalEnd(i) {
			this.#intervalEnd = i;
			this.dirty = this.#dirtyIntervals = true;
		}
		set opacityStart(i) {
			this.#opacityStart = i;
			this.dirty = this.#dirtyIntervals = true;
		}
		set opacityEnd(i) {
			this.#opacityEnd = i;
			this.dirty = this.#dirtyIntervals = true;
		}
		redraw() {
			if (this.#dirtyIntervals) {
				this._programs.setUniform("uInterval", [
					this.#intervalStart,
					this.#intervalEnd,
					this.#intervalEnd - this.#intervalStart,
				]);
				this._programs.setUniform("uIntervalOpacity", [
					this.#opacityStart,
					this.#opacityEnd,
				]);
				this.#dirtyIntervals = false;
			}
			return super.redraw.apply(this, arguments);
		}
	}

	class IntervalifiedSymbol extends base {
		static Acetate = IntervalifiedAcetate;

		constructor(
			geom,
			{ intervalStart = undefined, intervalEnd = undefined, ...opts } = {}
		) {
			super(geom, opts);
			this.intervalStart = intervalStart;
			this.intervalEnd = intervalEnd;
		}

		_setGlobalStrides(strideInterval, ...strides) {
			for (let i = this.attrBase, t = this.attrBase + this.attrLength; i < t; i++) {
				strideInterval.set([this.intervalStart, this.intervalEnd], i);
			}

			return super._setGlobalStrides(...strides);
		}
	}

	return IntervalifiedSymbol;
}
