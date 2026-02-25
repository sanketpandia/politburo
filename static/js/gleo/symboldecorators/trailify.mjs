/**
 * @namespace trailify
 * @inherits mValuifyLine
 *
 * Applies only to line symbols (either `Hair` or `Stroke`). The symbol gains
 * a m-value option, like in `trajectorify`. The acetate can then set a lower
 * and upper threshold for that m-value.
 *
 * The symbol will be drawn only when the (interpolated) m-value of a pixel
 * falls within the two thresholds of the acetate. The opacity of each pixel
 * depends on how close the m-value is to the upper threshold.
 */

import mValuifyLine from "./mValuifyLine.mjs";

export default function trailify(base) {
	const mValuifiedBase = mValuifyLine(base);

	// By default, the trail will fade out on the alpha component...
	let component = "a";
	if (mValuifiedBase.Acetate.PostAcetate?.name === "ScalarField") {
		// Except if the symbol is a HeatChain/HeatStroke/etc; in this case,
		// fade out the red (first) channel
		component = "r";
	} else if (mValuifiedBase.Acetate.PostAcetate?.name === "VectorField") {
		// Or two components for vector fields
		component = "rg";
	}

	/**
	 * @miniclass Trailified Acetate (trailify)
	 */
	class TrailifiedAcetate extends mValuifiedBase.Acetate {
		#opacityStart;
		#opacityEnd;

		constructor(target, { opacityStart = 0, opacityEnd = 1, ...opts } = {}) {
			/**
			 * @option opacityStart: Number = 0
			 * The initial value for trailified symbols near the acetate's interval start,
			 * i.e. the opacity at the "tail" of a symbol.
			 *
			 * @option opacityEnd: Number = 1
			 * The initial value for trailified symbols near the acetate's interval end,
			 * i.e. the opacity at the "head" of a symbol.
			 */
			super(target, opts);

			this.opacityStart = opacityStart;
			this.opacityEnd = opacityEnd;
			this.on("programlinked", () => this.#updateOpacity());
		}

		glProgramDefinition() {
			const opts = super.glProgramDefinition();

			return {
				...opts,
				uniforms: {
					...opts.uniforms,
					uTrailOpacity: "vec2", // Lower opacity, higher opacity
				},
				fragmentShaderMain: `${opts.fragmentShaderMain}
					if (vMCoord > (uMThreshold.x + uMThreshold.y)) {
						discard;
					} else if (vMCoord < uMThreshold.x) {
						discard;
					} else {
						gl_FragColor.${component} *= mix(uTrailOpacity.x, uTrailOpacity.y,
							(vMCoord - uMThreshold.x) / uMThreshold.y
						);
					}
				`,
			};
		}

		/**
		 * @property opacityStart
		 * The opacity for trailified symbols near the lower m-value threshold,
		 * i.e. the symbol's "tail". Can be updated at runtime.
		 * @property opacityEnd
		 * The opacity for trailified symbols near the higher m-value threshold,
		 * i.e. the symbol's "head". Can be updated at runtime.
		 */
		get opacityStart() {
			return this.#opacityStart;
		}
		get opacityEnd() {
			return this.#opacityEnd;
		}
		set opacityStart(i) {
			this.#opacityStart = i;
			this.#updateOpacity();
		}
		set opacityEnd(i) {
			this.#opacityEnd = i;
			this.#updateOpacity();
		}

		#updateOpacity() {
			this._programs.setUniform("uTrailOpacity", [
				this.#opacityStart,
				this.#opacityEnd,
			]);
			this.dirty = true;
		}
	}

	/**
	 * @miniclass Trailified GleoSymbol (trailify)
	 *
	 * A "trailified" symbol accepts this additional constructor options:
	 */
	return class trailifiedSymbol extends mValuifiedBase {
		static Acetate = TrailifiedAcetate;
	};
}
