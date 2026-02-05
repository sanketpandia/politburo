import AcetateInteractive from "./AcetateInteractive.mjs";
import { ScalarField } from "./Field.mjs";
import getBlendEquationConstant from "../util/getBlendEquationConstant.mjs";

/**
 * @class AcetateFuelPoint
 * @inherits AcetateExtrudedPoint
 * @relationship drawnOn ScalarField
 *
 * An `Acetate` to place `FuelPoint`s into a scalar field.
 *
 * The `AcetateFuelPoint` defines the "ramp factor": the speed at which the
 * field increases/decreases, in terms of field units per CRS unit.
 *
 */

export default class AcetateFuelPoint extends AcetateInteractive {
	#blendEquation;
	#radius;
	#rampFactor;

	// Absolute possible minimum and maximum intensity values, based on
	// min/max of all known points
	#min = Infinity;
	#max = -Infinity;

	/**
	 * @constructor AcetateFuelPoint(target: GliiFactory, opts: AcetateFuelPoint Options)
	 */
	constructor(
		target,
		{
			/**
			 * @option rampFactor: Number = 1
			 * Defines how fast the intensity fades out outwards, in
			 * scalar field units per CRS distance units ("units per metre").
			 *
			 * If the ramp factor is positive, then the calculated value into
			 * the field will be the minimum of the possible values; if it's
			 * positive, then maximum.
			 */
			rampFactor = 1,
			/// TODO: The sign of the ramp factor should be enough to choose
			/// the blend equation to use.

			/**
			 * @option radius: Number = 10000
			 * The radius of all FuelPoints, in CRS distance units
			 */
			radius = 10000,

			/// TODO: Refactor the whole thing so instead of a radius, a maximum
			/// value can be given. Each symbol would calculate its size based on the max value.
			/// This would, hopefully, enable early triangle depth culling and
			/// improve performance.

			...opts
		} = {}
	) {
		super(target, {
			zIndex: 2000,
			...opts,
		});

		this._attrs = new this.glii.InterleavedAttributes(
			{
				usage: this.glii.STATIC_DRAW,
				size: 1,
				growFactor: 1.2,
			},
			[
				{
					// Field intensity
					glslType: "float",
					type: Float32Array,
					normalized: false,
				},
				{
					// Distance to center (CRS distance units, AKA meters)
					glslType: "float",
					type: Float32Array,
					normalized: false,
				},
			]
		);

		this.#blendEquation = rampFactor >= 0 ? this.glii.MIN : this.glii.MAX;
		this.#radius = radius;
		this.#rampFactor = rampFactor;
	}

	/**
	 * @property PostAcetate: ScalarField
	 * Signals that this `Acetate` isn't rendered as a RGBA8 texture,
	 * but instead uses a scalar field.
	 */
	static get PostAcetate() {
		return ScalarField;
	}

	// This is the definition for the program turning HeatFuel
	// symbols into a float32 texture (AKA "scalar field")
	glProgramDefinition() {
		const opts = super.glProgramDefinition();
		return {
			...opts,
			attributes: {
				...opts.attributes,
				aIntensity: this._attrs.getBindableAttribute(0),
				aDistance: this._attrs.getBindableAttribute(1),
			},
			uniforms: {
				// uPixelSize: "vec2",
				uRampFactor: "float",
				uMin: "float",
				uMax: "float",
				uRange: "float",
				...opts.uniforms,
			},
			vertexShaderMain: `
				vIntensity = aIntensity + aDistance * uRampFactor;
				gl_Position = vec4(
					vec3(aCoords, 1.0) * uTransformMatrix
					, 1.0);

				gl_Position.z = (vIntensity - uMin) / uRange;
			`,
			varyings: {
				vIntensity: "float",
			},
			fragmentShaderMain: `gl_FragColor.r = vIntensity;`,
			// fragmentShaderMain: `gl_FragColor.r = gl_FragCoord.z;`,
			target: this._inAcetate.framebuffer,
			// depth: this.glii.LEQUAL,
			depth: this.glii.LESS,
			// depth: this.glii.GREATER,
			blend: {
				equationRGB: this.#blendEquation,
				// equationRGB: this.glii.FUNC_ADD,
				equationAlpha: this.#blendEquation,

				/**
				 * NOTE: When using blend modes that multiply the src RGB
				 * components by the scr alpha, then the fragment shader
				 * needs to set the alpha component. i.e. there's a need to
				 * set `gl_FragColor.a = 1.;`.
				 *
				 * This is counter-intuitive, since the R32F texture has no
				 * alpha component. **BUT**, the alpha component of
				 * gl_FragColor lives until the blend operation.
				 *
				 * By setting the srcRGB blend parameter to `ONE`, the output
				 * is unaffected by the alpha component.
				 */
				srcRGB: this.glii.ONE,
				srcAlpha: this.glii.ZERO,
				dstRGB: this.glii.ONE,
				dstAlpha: this.glii.ZERO,
			},
		};
	}

	glIdProgramDefinition() {
		const def = super.glIdProgramDefinition();

		return {
			...def,
			blend: {
				equationRGB: this.glii.MAX,
				equationAlpha: this.glii.MAX,

				srcRGB: this.glii.ONE,
				srcAlpha: this.glii.ONE,
				dstRGB: this.glii.ZERO,
				dstAlpha: this.glii.ZERO,
			},
		};
	}

	resize(x, y) {
		super.resize(x, y);
		this._program._target = this._inAcetate.framebuffer;
		// this._program.setUniform("uPixelSize", [2 / x, 2 / y]);
		this._programs.setUniform("uRampFactor", this.#rampFactor);
		this._programs.setUniform("uMin", this.#min);
		// this._programs.setUniform("uMax", this.#max);
		this._programs.setUniform("uRange", Math.ceil(this.#max - this.#min));
	}

	/**
	 * @property rampFactor: Number
	 * Gets or sets the current ramp factor.
	 *
	 * Note that changing the sign of the ramp factor will **not** behave properly.
	 */
	get rampFactor() {
		return this.#rampFactor;
	}

	set rampFactor(r) {
		this._programs.setUniform("uRampFactor", (this.#rampFactor = r));

		if (this._knownSymbols.length) {
			const intensities = this._knownSymbols.map((fp) => fp.intensity);

			const [minValue, maxValue] = intensities.reduce(
				(acc, val) => {
					return [
						Math.min(val, acc[0]),
						Math.max(val, acc[1] + this.#rampFactor * this.#radius),
					];
				},
				[Infinity, -Infinity]
			);

			// const minValue = Math.min(...intensities);
			this.#min = Math.min(this.#min, minValue);
			// const maxValue = Math.max(...intensities) + this.#rampFactor * this.#radius;
			this.#max = Math.max(this.#max, maxValue);
			this._programs.setUniform("uMin", this.#min);
			// this._programs.setUniform("uMax", this.#max);
			this._programs.setUniform("uRange", Math.ceil(this.#max - this.#min));
		}

		this.dirty = true;
	}

	_getStridedArrays(maxVtx, maxIdx) {
		return [
			// Field intensity
			this._attrs.asStridedArray(0, maxVtx),
			// Distance to center
			this._attrs.asStridedArray(1),
			// Triangle indices
			this._indices.asTypedArray(maxIdx),
			// Static extrusion radius (CRS distance units)
			this.#radius,
		];
	}

	/**
	 * @section
	 * @method multiAdd(fuelpoints : Array of FuelPoint): this
	 * Adds the `FuelPoint`s to the acetate.
	 */
	multiAdd(fuelpoints) {
		// Skip already added symbols
		fuelpoints = fuelpoints.filter((e) => !e._inAcetate);
		if (fuelpoints.length === 0) {
			return;
		}

		// Sort the fuelpoints by their intensity in an effort to minimize
		// fragment passes
		if (this.#blendEquation == this.glii.MIN) {
			fuelpoints = fuelpoints.sort((a, b) => a.intensity - b.intensity);
		} else if (this.#blendEquation == this.glii.MAX) {
			fuelpoints = fuelpoints.sort((a, b) => b.intensity - a.intensity);
		}

		const totalIndices = fuelpoints.reduce((acc, fp) => acc + fp.idxLength, 0);
		const totalVertices = fuelpoints.reduce((acc, fp) => acc + fp.attrLength, 0);

		let baseVtx = this._attribAllocator.allocateBlock(totalVertices);
		let baseIdx = this._indices.allocateSlots(totalIndices);
		const maxVtx = baseVtx + totalVertices;

		let stridedArrays = this._getStridedArrays(maxVtx, baseIdx + totalIndices);

		let vtxAcc = baseVtx;
		let idxAcc = baseIdx;

		fuelpoints.map((fp) => {
			fp.updateRefs(this, vtxAcc, idxAcc);
			this._knownSymbols[vtxAcc] = fp;

			fp._setGlobalStrides(...stridedArrays);

			vtxAcc += fp.attrLength;
			idxAcc += fp.idxLength;
		});

		// this._commitStridedArrays(baseVtx, totalVertices);
		this._attrs.commit(baseVtx, totalVertices);

		this._indices.commit(baseIdx, totalIndices);

		if (this._crs) {
			this.reproject(baseVtx, totalVertices, fuelpoints);
		}

		// The AcetateInteractive will assign IDs to symbol vertices.
		super.multiAddIds(fuelpoints, baseVtx);

		const intensities = fuelpoints.map((fp) => fp.intensity);

		const [minValue, maxValue] = intensities.reduce(
			(acc, val) => {
				return [
					Math.min(val, acc[0]),
					Math.max(val, acc[1] + this.#rampFactor * this.#radius),
				];
			},
			[this.#min, this.#max]
		);

		// const minValue = Math.min(...intensities);
		this.#min = Math.min(this.#min, minValue);
		// const maxValue = Math.max(...intensities) + this.#rampFactor * this.#radius;
		this.#max = Math.max(this.#max, maxValue);
		this._programs.setUniform("uMin", this.#min);
		// this._programs.setUniform("uMax", this.#max);
		this._programs.setUniform("uRange", this.#max - this.#min);

		console.log(this.#min, this.#max, this.#max - this.#min);

		this.dirty = true;

		return super.multiAdd(fuelpoints);
	}

	/**
	 * @section Internal Methods
	 * @uninheritable
	 *
	 * @method reproject(start: Number, length: Number, symbols: Array of GleoSymbol): Array of Number
	 * Dumps a new set of values to the `this._coords` attribute buffer, based on the known
	 * set of symbols added to the acetate (only those which have their attribute offsets
	 * between `start` and `start+length`. Each symbol will spawn as many
	 * coordinate `vec2`s as their `attrLength` property.
	 *
	 * Returns the data set into the attribute buffer: a plain array of coordinates
	 * in the form `[x1,y1, x2,y2, ... xn,yn]`.
	 */
	reproject(start, length, symbols) {
		let relevantSymbols =
			symbols ??
			this._knownSymbols.filter((symbol, attrIdx) => {
				return attrIdx >= start && attrIdx + symbol.attrLength <= start + length;
			});

		let addr = 0;
		const ρ = this.#radius;

		// In most cases, it's safe to assume that relevant symbols in the same
		// attribute allocation block have their vertex attributes in a
		// compacted manner.

		const coordData = new Float64Array(length * 2);

		relevantSymbols.forEach((symbol) => {
			const projected = symbol.geometry.toCRS(this.platina.crs).coords;

			// Center point
			coordData.set(projected, addr);
			addr += 2;

			let θ = 0;
			const ɛ = (Math.PI * 2) / symbol.steps;

			// for (let i = 0; i < symbol.attrLength; i++) {
			for (let i = 0; i < symbol.steps; i++) {
				/// TODO: Use a geodetic method instead of a CRS-planar method.
				/// i.e. this calculation should use meters, not EPSG:3857 units

				coordData.set(
					[projected[0] + Math.sin(θ) * ρ, projected[1] + Math.cos(θ) * ρ],
					addr
				);
				addr += 2;
				θ += ɛ;
			}
		});

		//console.log("Symbol reprojected:", coordData);
		this.multiSetCoords(start, coordData);

		this.dirty = true;

		return coordData;
	}
}
