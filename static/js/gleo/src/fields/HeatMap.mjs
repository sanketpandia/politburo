import { ScalarField } from "./Field.mjs";
import parseColour from "../3rd-party/css-colour-parser.mjs";
import glslFloatify from "../util/glslFloatify.mjs";
import glslVecNify from "../util/glslVecNify.mjs";

/**
 * @class HeatMap
 * @inherits ScalarField
 *
 * A `ScalarField` to draw heat maps, with a custom colour ramp.
 *
 * Since the colour ramp is configurable, this `ScalarField` must be manually
 * instantiated, unlike most `Acetate`s.
 *
 * While it's possible to add `HeatPoint`s directly to a `Platina`,
 * doing so (before instantiating a `HeatMap`) would use the
 * defaults and draw those `HeatPoint`s in a black-and-white `GreyscaleField`.
 * It is strongly advised to instantiate a `HeatMap` before placing
 * any `HeatPoint`s.
 *
 * Very similar to `GreyscaleField`, but with multiple colour stops in the
 * colour ramp (instead of two colours for min/max values).
 *
 * @example
 *
 * ```js
 * const heatmap = new HeatMap(map, {
 * 	stops: {
 * 		0: [255,0,0,0],
 * 		10: [255,0,0,255],
 * 		100: [255,255,0,255],
 * 		200: [0,255,0,255],
 * 		500: [0,255,255,255]
 * 	},
 * });
 *
 * new HeatPoint(geom, {radius: 80, intensity: 500}).addTo(heatmap);
 *
 * ```
 */

export default class HeatMap extends ScalarField {
	#stops;

	/**
	 * @constructor HeatMap(glii: GliiFactory, opts?: AcetateQuadBin Options)
	 *
	 */
	constructor(
		glii,
		{
			/**
			 * @option stops: Object of Number to Colour
			 * A map of intensities to `Colour`s that defines how to
			 * colourize the heatmap. A pixel with an intensity of a stop
			 * will exactly get that colour; any other colours will be
			 * linearly interpolated.
			 *
			 * The first key must always be zero, and the keys must be
			 * ordered in strictly ascending order.
			 *
			 * Default is:
			 * ```
			 * {
			 * 	0: [255,0,0,0]        	// Transparent red
			 *  10: [255,0,0,255]     	// Red
			 *  100: [255,255,0,255]  	// Yellow
			 *  1000: [0,255,0,255]   	// Green
			 *  10000: [0,255,255,255]	// Cyan
			 * }
			 * ```
			 */
			stops = {
				0: [0, 0, 255, 0],
				10: [0, 0, 255, 255],
				100: [0, 255, 255, 255],
				1000: [0, 255, 0, 255],
				10000: [255, 255, 0, 255],
			},
			...opts
		} = {}
	) {
		super(glii, {
			zIndex: 2000,
			...opts,
		});

		this.#stops = stops;
	}

	/// @property stops: Object of Number to Colour
	/// The colour ramp, as the homonymous option.
	///
	/// It can be updated, recompiling the WebGL shader in the process.
	get stops() {
		return this.#stops;
	}

	set stops(s) {
		this.#stops = s;
		this.rebuildShaderProgram();
	}

	// Returns the definition for the GL program that turns the float32 texture
	// into a RGBA8 texture
	glProgramDefinition() {
		/// TODO: Refactor the GLSL program: unroll the loop, and
		/// have the colours as constants

		let intensities = [];
		let colours = [];

		Object.entries(this.#stops)
			.map(([intensity, colour]) => [Number(intensity), parseColour(colour)])
			.sort(([a, _], [b, __]) => a - b)
			.forEach(([intensity, colour]) => {
				intensities.push(intensity);
				colours.push(colour);
			});

		// const intensities = Object.keys(this.#stops).map(n=>Number(n));
		// const colours = Object.values(this.#stops).map((c) => parseColour(c));
		const stopCount = colours.length;
		// NOTE: GLSL array constructors are avaialble in GLSL 3.00 (WebGL2),
		// would allow to specify the consts outside main(), and would look like
		// const float intensities[${stopCount}] = float[${stopCount}](${intensities.join(',')});
		const intensitiesInit = intensities
			.map((i, j) => `intensities[${j}] = float(${glslFloatify(i)});`)
			.join("\n");
		const coloursInit = colours
			.map((c, j) => `colours[${j}] = ${glslVecNify(c.map((b) => b / 255))};`)
			.join("\n");

		const opts = super.glProgramDefinition();
		return {
			...opts,
			fragmentShaderMain: `
				float intensities[${stopCount}];
				vec4 colours[${stopCount}];
				${intensitiesInit}
				${coloursInit}

				float value = texture2D(uField, vUV).x;

				//if (value <= 0.0) {discard;}

				gl_FragColor = colours[0];

				for (int i=1; i< ${stopCount}; i++) {
					gl_FragColor = mix(
						gl_FragColor,
						colours[i],
						smoothstep(intensities[i-1], intensities[i], value)
					);
				}
			`,
		};
	}
}
