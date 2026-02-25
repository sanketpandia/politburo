/**
 * @namespace fadeIntervalify
 * @inherits Symbol Decorator
 *
 * As `intervalify`, but the symbol's opacity depends on the position of the
 * symbol's interval relative to the acetate's interval limits (the default `intervalify`
 * just hides symbols whose interval is outside the acetate's interval).
 *
 *
 * @example
 *
 * A typical use case is filtering data by a time interval while fading out data
 * points that towards one side of the interval.
 *
 * ```
 * import fadeIntervalify from 'gleo/src/symbols/fadeIntervalify.mjs';
 *
 * const FadeSprite = fadeIntervalify(Sprite, "within");	// "Intervalified" `Sprite` class
 *
 * const sprite1 = new FadeSprite(geom1, {
 * 	intervalStart: Date.parse("1975-05-30"),
 * 	intervalEnd: Date.parse("1975-06-02"),
 * 	...spriteOptions
 * };
 *
 * const myIntervalAcetate = new IntervalSprite.Acetate(map);
 * myIntervalAcetate.multiAdd([sprite1]);
 *
 * // If a sprite falls near the start of the acetate's interval, make it transparent
 * myIntervalAcetate.opacityStart = 0;
 * // If a sprite falls near the end of the acetate's interval, make it opaque
 * myIntervalAcetate.opacityEnd = 1;
 *
 * myIntervalAcetate.intervalStart = Date.parse("1970-01-01");
 * myIntervalAcetate.intervalEnd = Date.parse("1980-01-01");
 * ```
 *
 * @miniclass FadeIntervalified GleoSymbol (fadeIntervalify)
 * @section
 * The return value of the `fadeIntervalify` function is a `GleoSymbol` class. It
 * will behave as the base class used, but with two extra constructor options:
 *
 * @option intervalStart: Number
 * The start (minimum value) of the symbol's numeric interval.
 * @option intervalEnd: Number
 * The end (maximum value) of the symbol's numeric interval.
 *
 * @miniclass FadeIntervalified Acetate (fadeIntervalify)
 * @section
 *
 * Interval-enabled symbols need their own acetate to be drawn into - the
 * example code above shows how to access it from an intervalified symbol
 * class, and how to instantiate and add it to a map/platina.
 *
 * An intervalified `Acetate` gains two new read/write properties. Setting
 * a new value to any of these will trigger a re-render.
 *
 * @property intervalStart: Number
 * The start (minimum value) of the symbol's numeric interval. Any symbols
 * in this acetate with an interval that falls outside the acetate's own
 * interval will **not** be drawn.
 * @property intervalEnd: Number
 * The end (maximum value) of the symbol's numeric interval.
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
 * `mode` can take two values:
 *
 * - When `mode` is `within`, then a symbol will be shown if its interval is
 *   completely within the acetate's interval. This is the default.
 * - When `mode` is `intersect`, then a symbol will be shown if its interval
 *   intersects with the acetate's interval.
 *
 */

export default function intervalify(base, mode = "within") {
	class IntervalifiedAcetate extends base.Acetate {
		constructor(
			target,
			{ intervalStart = -Infinity, intervalEnd = Infinity, ...opts } = {}
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

			// // Ideally, this mix-in would redefine `multiAdd`, and inside
			// // there call `super.multiAdd(symbols)`. But, since symbol allocation
			// // can be async (e.g. in `Sprite`s), the reliable way to fetch
			// // allocated symbols is to listen to the "symbolsadded" event.
			// this.on("symbolsadded", this.#onSymbolsAdded.bind(this));
		}

		glProgramDefinition() {
			const opts = super.glProgramDefinition();

			return {
				...opts,
				attributes: {
					aInterval: this._intervalAttr,
					...opts.attributes,
				},
				uniforms: {
					uInterval: "vec2",
					...opts.uniforms,
				},
				vertexShaderMain: mode === "intersect" ? `
				if (aInterval.x > uInterval.x || aInterval.y < uInterval.y) {
					${opts.vertexShaderMain}
				}` : `
				if (aInterval.x > uInterval.x && aInterval.y < uInterval.y) {
					${opts.vertexShaderMain}
				}`
			};
		}

		// #onSymbolsAdded(ev) {
		// 	const symbols = ev.detail.symbols;
		//
		// 	// This makes the ASSUMPTION that the parent functionality kept the
		// 	// order of the symbols intact, and that the attribute allocation
		// 	// runs from the `attrBase` of the first symbol to the
		// 	// `attrBase`+`attrLength` of the last symbol.
		//
		// 	const attrBase = symbols[0].attrBase;
		//
		// 	const intervals = symbols
		// 		.map((s) =>
		// 			new Array(s.attrLength).fill([s.intervalStart, s.intervalEnd])
		// 		)
		// 		.flat(2);
		//
		// 	this._intervalAttr.multiSet(attrBase, intervals);
		//
		// 	return this;
		// }

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
		#dirtyIntervals = false;
		get intervalStart() {
			return this.#intervalStart;
		}
		get intervalEnd() {
			return this.#intervalEnd;
		}
		set intervalStart(i) {
			this.#intervalStart = i;
			this.dirty = this.#dirtyIntervals = true;
		}
		set intervalEnd(i) {
			this.#intervalEnd = i;
			this.dirty = this.#dirtyIntervals = true;
		}
		redraw() {
			if (this.#dirtyIntervals) {
				this._programs.setUniform("uInterval", [
					this.#intervalStart,
					this.#intervalEnd,
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
