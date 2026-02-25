import ExtrudedPoint from "../symbols/ExtrudedPoint.mjs";

/**
 * @namespace screenedgify
 * @inherits Symbol Decorator
 * @relationship associated ExtrudedPoint
 *
 * Modifies an `ExtrudedPoint` class (e.g. `Sprite`s, `CircleFill`s, etc) so that
 * they are always visible in the viewport, sticking to the edge when they should
 * be outside.
 *
 */

export default function bouncify(base) {
	if (!base instanceof ExtrudedPoint) {
		throw new Error(
			"The 'screenedgify' symbol decorator can only be applied to extruded points"
		);
	}

	class ScreenEdgifiedAcetate extends base.Acetate {
		#margin;

		constructor(
			target,
			{
				/**
				 * @option margin: Array of Number = [0,0,0,0]
				 * The margin for the symbols, in CSS pixels, in
				 * `[top, right, bottom, left]` form.
				 */
				margin = [0, 0, 0, 0],

				...opts
			} = {}
		) {
			// Hack the acetate's bbox so that it's drawn even if all symbols
			// seem to be outside of the viewport
			super(target, opts);

			this.#margin = margin;
			this.bbox.expandXY(-Infinity, -Infinity);
			this.bbox.expandXY(Infinity, Infinity);
		}

		glProgramDefinition() {
			const opts = super.glProgramDefinition();

			if (opts.vertexShaderMain.includes("vec3(aCoords, 1.0) * uTransformMatrix")) {
				// Clamp the position *before* applying the extrusion

				const clampFn = `vec3 clampPosition(vec3 pos) {
					return vec3(
						clamp( pos.x, uScreenEdgeMargin.w, uScreenEdgeMargin.y),
						clamp( pos.y, uScreenEdgeMargin.z, uScreenEdgeMargin.x),
						pos.z
					);
				}`;

				return {
					...opts,
					vertexShaderSource: opts.vertexShaderSource + clampFn,
					vertexShaderMain: opts.vertexShaderMain.replace(
						"vec3(aCoords, 1.0) * uTransformMatrix",
						"clampPosition(vec3(aCoords, 1.0) * uTransformMatrix)"
					),
					uniforms: {
						uScreenEdgeMargin: "vec4",
						...opts.uniforms,
					},
				};
			} else {
				// Fallback, may squish symbols near the edge.
				return {
					...opts,
					vertexShaderMain:
						opts.vertexShaderMain +
						`
					gl_Position = vec4(
						clamp(gl_Position.x, uScreenEdgeMargin.w, uScreenEdgeMargin.y),
						clamp(gl_Position.y, uScreenEdgeMargin.z, uScreenEdgeMargin.x),
						gl_Position.zw);
					`,
					uniforms: {
						uScreenEdgeMargin: "vec4",
						...opts.uniforms,
					},
				};
			}
		}

		resize(x, y) {
			super.resize(x, y);

			this._programs.setUniform("uScreenEdgeMargin", [
				+1 - this.#margin[0] / y, // top
				+1 - this.#margin[1] / x, // right
				-1 + this.#margin[2] / y, // bottom
				// reprojectAll() also resets the bounding box, so reapply the hack.
				-1 + this.#margin[3] / x, // left
			]);

			return this;
		}

		reprojectAll() {
			super.reprojectAll();
			this.bbox.expandXY(-Infinity, -Infinity);
			this.bbox.expandXY(Infinity, Infinity);
		}
	}

	/**
	 * @miniclass ScreenEdgifiedSymbol (bouncify)
	 *
	 * A "screenedgified" symbol accepts these additional constructor options:
	 */

	return class ScreenEdgifiedSymbol extends base {
		static Acetate = ScreenEdgifiedAcetate;
	};
}
