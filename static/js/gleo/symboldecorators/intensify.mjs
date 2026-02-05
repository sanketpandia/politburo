import { ScalarField } from "../fields/Field.mjs";
import getBlendEquationConstant from "../util/getBlendEquationConstant.mjs";

/**
 * @namespace intensify
 * @inherits Symbol Decorator
 * @relationship drawnOn ScalarField
 *
 * Takes a symbol that is normally rendered as a single solid colour (including
 * `Fill`, `Stroke`, `Circle`, etc), and makes it render into a `ScalarField`.
 * The symbol loses its `colour` option but gains an `intensity` option.
 *
 * The symbol will work similar to `HeatPoint` or `HeatStroke` in the sense
 * that it adds its intensity to the scalar field that is given colour later
 * (e.g. `HeatMapAcetate`); the symbol must be added to such a scalar field
 * instead of being added directly to the map. Unlike `HeatPoint` or `HeatStroke`
 * (which apply a linear fall-off to the intensity), the intensity an
 * intensified symbol adds to the field is constant in all its pixels.
 *
 * Might **not** work properly on symbols with more than a colour, nor or those
 * depending on a image/texture (e.g. `Sprite`, `TextLabel`)
 */

export default function intensify(base) {
	if (!("_parseColour" in base)) {
		throw new Error(
			`The symbol class to be intensified (${base.constructor.name}) doesn't seem to be one with a single solid colour`
		);
	}

	// Hack for Circle & HeadingTriangle.
	// (a similar hack would be needed for StrokeRoad; not that
	// intensify+StrokeRoad is gonna be a popular combination)
	let colourFields = 1;
	let proto = base.Acetate;
	while (proto) {
		if (proto.name === "AcetateSolidBorder") {
			colourFields = 2;
		}
		proto = proto.__proto__;
	}

	/**
	 * @miniclass Intensified Acetate (intensify)
	 * The acetate of an intensify()ed symbol gains a new constructor option.
	 */
	class IntensifiedAcetate extends base.Acetate {
		static get PostAcetate() {
			return ScalarField;
		}

		#blendEquation;
		constructor(
			target,
			{
				/**
				 * @option blendEquation: String
				 * Defines how the symbols' intensity affects the value of the
				 * scalar field. The default is `"ADD"`, which means the intensity
				 * is added to the scalar field. Other possible values are `"SUBTRACT"`,
				 * `"MIN"` and `"MAX"`.
				 */
				blendEquation = "ADD",

				...opts
			} = {}
		) {
			super(target, opts);

			// This uses the same trick as AcetateHeatStroke: modify the main
			// attribute storage (which should be an InterleavedAttributes)
			// so that the colour, which *should* always be in the 1st slot,
			// stops being a `vec4`+`Uint8Array` and becomes a `float`+`Float32Array`

			const fields = this._attrs._fields;
			fields[0] = {
				glslType: "float",
				type: Float32Array,
			};

			if (colourFields === 2) {
				fields[1] = {
					glslType: "float",
					type: Float32Array,
				};
			}

			this._attrs = new this.glii.InterleavedAttributes(
				{
					size: 1,
					growFactor: 1.2,
					usage: this.glii.STATIC_DRAW,
				},
				fields
			);

			this.#blendEquation = getBlendEquationConstant(this.glii, blendEquation);
		}

		glProgramDefinition() {
			const opts = super.glProgramDefinition();

			// General attribute name
			const search1 = /vColour\s*=\s*aColour\s*;/g;
			const replacement1 = "vColour = vec4(aColour, 0., 0., 1.);";

			// Attribute names for AcetateSolidBorder (i.e. Circle symbol)
			const search2 = /vFillColour\s*=\s*aFillColour\s*;/g;
			const replacement2 = "vFillColour = vec4(aFillColour, 0., 0., 1.);";
			const search3 = /vBorderColour\s*=\s*aBorderColour\s*;/g;
			const replacement3 = "vBorderColour = vec4(aBorderColour, 0., 0., 1.);";

			// Opacity handling becomes intensity handling
			const search4 = /gl_FragColor.a\s*\*=\s*/g;
			const replacement4 = "gl_FragColor.r *= ";

			return {
				...opts,
				attributes: {
					...opts.attributes,
					aColour: this._attrs?.getBindableAttribute(0),
				},
				varyings: {
					...opts.varyings,
					vColour: "vec4",
					vFillColour: "vec4",
					vBorderColour: "vec4", // vColour: "float"
				},
				target: this._inAcetate.framebuffer,
				blend: {
					// See notes about blend mode in AcetateHeatStroke

					/// TODO: Some option to turn on alpha on the blend equation.
					/// Normally an intensified symbol will just dump the colour
					/// into gl_FragColor, but some might benefit from enabling
					/// alpha blending (multiply intensity by alpha) if they've
					/// got a shader that manages alpha.

					equationRGB: this.#blendEquation,
					equationAlpha: this.#blendEquation,
					srcRGB: this.glii.ONE,
					srcAlpha: this.glii.ZERO,
					dstRGB: this.glii.ONE,
					dstAlpha: this.glii.ZERO,
				},
				vertexShaderMain: opts.vertexShaderMain
					.replace(search1, replacement1)
					.replace(search2, replacement2)
					.replace(search3, replacement3),
				// fragmentShaderSource: "void main() { gl_FragColor.r = 100.; }"
				fragmentShaderMain: opts.fragmentShaderMain.replace(
					search4,
					replacement4
				),
				unusedWarning: false,
			};
		}
		resize(x, y) {
			super.resize(x, y);
			this._program._target = this._inAcetate.framebuffer;
		}
	}

	/**
	 * @miniclass IntensifiedSymbol (intensify)
	 *
	 * An "intensified" symbol accepts these additional constructor options:
	 */
	class IntensifiedSymbol extends base {
		static Acetate = IntensifiedAcetate;

		constructor(
			geom,
			{
				/**
				 * @option intensity: Number = 1
				 * Intensity applied to the scalar field on all pixels corresponding
				 * to this symbol.
				 */
				intensity = 1,
				...opts
			}
		) {
			super(geom, { ...opts, colour: intensity });
		}

		static _parseColour(intensity) {
			if (Array.isArray(intensity)) {
				return null;
			}
			return [intensity];
		}
	}

	return IntensifiedSymbol;
}
