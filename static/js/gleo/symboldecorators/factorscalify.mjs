/**
 * @namespace factorscalify
 * @inherits Symbol Decorator
 *
 * Applies a scale factor to symbols depending on the map's scale.
 *
 * Typical use cases include `Stroke`d lines that must appear thinner when
 * zoomed out, but thicker when zoomed in. The decorator takes in a map
 * of map scale values to symbol scale factors.
 *
 * This decorator can be applied to:
 * - `ExtrudedPoint`s (including `CircleFill`, `Sprite`, etc)
 * - `Stroke`s and similar linestring symbols (`StrokeRoad`, `Chain`, `HeatStroke`, etc)
 *
 *
 * @example
 *
 * ```js
 * const ScaledStroke = factorscalify(Stroke, {
 * 	// At a map scale of 1000 CRS units per CSS pixel, halve the size of the symbols
 * 	1000: 0.5,
 *
 * 	// At a map scale of 1 CRS unit per CSS pixel, use the symbol's original size
 * 	1: 1
 * });
 * ```
 *
 */

export default function factorscalify(base, scaleMap = { 1: 1 }) {
	// Sort the map scales
	const sortedScaleFactorPairs = Object.entries(scaleMap).sort((a, b) => b[0] - a[0]);

	const logScaleFactorPairs = sortedScaleFactorPairs.map(([scale, factor]) => [
		Math.log2(scale),
		factor,
	]);

	console.log(logScaleFactorPairs);

	const l = logScaleFactorPairs.length - 1;

	function factorForMapScale(scale) {
		const logScale = Math.log2(scale);
		if (logScale > logScaleFactorPairs[0][0]) {
			return logScaleFactorPairs[0][1];
		}

		for (let i = 0; i < l; i++) {
			const lower = logScaleFactorPairs[i + 1][0];
			if (logScale > lower) {
				const upper = logScaleFactorPairs[i][0];
				const span = upper - lower;
				const pct = (logScale - lower) / span;

				const minFactor = logScaleFactorPairs[i + 1][1];
				const maxFactor = logScaleFactorPairs[i][1];

				return maxFactor * pct + minFactor * (1 - pct);
			}
		}

		return logScaleFactorPairs[l][1];
	}

	class FactorScalifiedAcetate extends base.Acetate {
		glProgramDefinition() {
			const opts = super.glProgramDefinition();
			const regexpExtrude = /(\W)aExtrude(\W)/g;
			const replacementExtrude = function replacement(_, pre, post) {
				return `${pre}(aExtrude * uScaleFactor)${post}`;
			};

			return {
				...opts,
				vertexShaderMain: opts.vertexShaderMain.replace(
					regexpExtrude,
					replacementExtrude
				),
				uniforms: {
					uScaleFactor: "float",
					...opts.uniforms,
				},
			};
		}

		redraw() {
			// this._programs.setUniform("uNow", performance.now());
			this._programs.setUniform(
				"uScaleFactor",
				factorForMapScale(this.platina.scale)
			);

			return super.redraw.apply(this, arguments);
		}
	}

	return class FactorScalifiedSymbol extends base {
		static Acetate = FactorScalifiedAcetate;
	};
}
