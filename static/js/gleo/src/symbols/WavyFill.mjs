import Fill from "./Fill.mjs";
import parseColour from "../3rd-party/css-colour-parser.mjs";

/**
 * @class AcetateWavyFill
 * @inherits AcetateVertices
 *
 * An animated `Acetate` for `WavyFill`s.
 */
class AcetateWavyFill extends Fill.Acetate {
	/**
	 * @constructor AcetateWavyFill(target: GliiFactory)
	 */
	constructor(target, opts = {}) {
		super(target, opts);

		// Could be done as a SingleAttribute, but is a InterleavedAttributes for
		// compatibility with the `intensify` decorator.

		this._attrs = new this.glii.InterleavedAttributes(
			{
				size: 1,
				growFactor: 1.2,
				usage: this.glii.STATIC_DRAW,
			},
			[
				{
					// colour one
					glslType: "vec4",
					type: Uint8Array,
					normalized: true,
				},
				{
					// colour two
					glslType: "vec4",
					type: Uint8Array,
					normalized: true,
				},
				{
					// Height of both bands + wave height proportion
					glslType: "vec2",
					type: Float32Array,
					normalized: false,
				},
			]
		);
	}

	glProgramDefinition() {
		const opts = super.glProgramDefinition();
		return {
			...opts,
			attributes: {
				...opts.attributes,
				aColour1: this._attrs.getBindableAttribute(0),
				aColour2: this._attrs.getBindableAttribute(1),
				aWaveHeight: this._attrs.getBindableAttribute(2),
			},
			vertexShaderMain: `
				vColour1 = aColour1;
				vColour2 = aColour2;
				vWaveHeight = aWaveHeight;
				gl_Position = vec4( vec3(aCoords, 1.0) * uTransformMatrix, 1.0);
			`,
			varyings: {
				vColour1: "vec4",
				vColour2: "vec4",
				vWaveHeight: "vec2",
			},
			fragmentShaderMain: `

			float height = gl_FragCoord.y / vWaveHeight.x;
			float sinTime1 = sin(gl_FragCoord.x / 10. + uTime) * vWaveHeight.y;
			float sinTime2 = sin(gl_FragCoord.x / -10. + uTime) * vWaveHeight.y;
			float alpha = fract( height + sinTime1 );
			float beta = fract(height + sinTime2 + .5);

			if ( alpha >  beta) {
				gl_FragColor = vColour1;
			} else {
				gl_FragColor = vColour2;
			}
			`,
			uniforms: {
				// uNow: "float",
				uTime: "float",
				...opts.uniforms,
			},
		};
	}

	_getStridedArrays(maxVtx, _maxIdx) {
		return [
			// Indices
			//...super._getStridedArrays(maxVtx, maxIdx),

			// Colour 1
			this._attrs.asStridedArray(0, maxVtx),

			// Colour 2
			this._attrs.asStridedArray(1, maxVtx),

			// Band/Wave heights
			this._attrs.asStridedArray(2, maxVtx),
		];
	}

	redraw() {
		// this._programs.setUniform("uNow", performance.now());
		this._programs.setUniform("uTime", performance.now() / 1000);
		return super.redraw.apply(this, arguments);
	}

	// Animated acetates are always dirty
	get dirty() {
		return super.dirty || this._knownSymbols.length > 0;
	}
	set dirty(d) {
		return (super.dirty = d);
	}
}

/**
 * @class WavyFill
 * @inherits Fill
 * @relationship drawnOn AcetateWavyFill
 *
 * Animated polygon fill with a wavy animation, meant for water features.
 */
export default class WavyFill extends Fill {
	/// @section Static properties
	/// @property Acetate: Prototype of AcetateWavyFill
	// The `Acetate` class that draws this symbol.
	static Acetate = AcetateWavyFill;

	#colour1;
	#colour2;
	#bandsHeight;
	#waveHeight;

	/**
	 * @section
	 * @constructor WavyFill(geom: Geometry, opts?: WavyFill Options)
	 */
	constructor(
		geom,
		{
			/**
			 * @section
			 * @aka WavyFill Options
			 * @option colour1: Colour = '#3388ffc0'
			 * The first colour of the fill symbol.
			 * @option colour2: Colour = '#2266ffc0'
			 * The second colour of the fill symbol.
			 */
			colour1 = [0x33, 0x88, 0xff, 0xc0],

			colour2 = [0x22, 0x66, 0xff, 0xc0],

			/**
			 * @option bandsHeight: Number = 40
			 * The height (in CSS pixels) of the two colour bands
			 * @option waveHeight: Number = 0.25
			 * The height of the tip of a wave (relative to its bottom point),
			 * as a percentage of `bandsHeight`.
			 */
			bandsHeight = 40,
			waveHeight = 0.25,

			...opts
		} = {}
	) {
		// Length of each linestring
		//this._lengths = linestrings.map((ls) => ls.length);
		super(geom, opts);

		// Amount of vertex attribute slots needed
		// Attribute slots is *half* of the lenght of the [x1,y2, ...xn,xy] flat array
		this.attrLength = this.geom.coords.length / this.geom.dimension;

		// Amount of index slots needed (calc'd by earcut)
		//this.idxLength = (this.attrLength - this._lengths.length) * 2;

		this.#colour1 = this.constructor._parseColour(colour1);
		this.#colour2 = this.constructor._parseColour(colour2);
		this.#bandsHeight = bandsHeight;
		this.#waveHeight = waveHeight;
		if (this.#colour1 === null || this.#colour2 === null) {
			throw new Error("Invalid colours specified for WavyFill.");
		}
	}

	_setGlobalStrides(strideColour1, strideColour2, strideWaveHeight) {
		const attrMax = this.attrBase + this.attrLength;
		for (let i = this.attrBase; i < attrMax; i++) {
			strideColour1.set(this.#colour1, i);
			strideColour2.set(this.#colour2, i);
			strideWaveHeight.set([this.#bandsHeight, this.#waveHeight / 2], i);
		}
		return this;
	}

	// Can be overriden by subclasses or the `intensify` decorator
	static _parseColour = parseColour;
}
