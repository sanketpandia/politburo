import AcetateExtrudedPoint from "./AcetateExtrudedPoint.mjs";

/**
 * @class AcetateSolidExtrusion
 * @inherits AcetateExtrudedPoint
 *
 * An `Acetate` for rendering solid colors on extrusions of point geometries;
 * this is common for `Pie`, `CircleFill` and `CircleStroke` symbols.
 */

export default class AcetateSolidExtrusion extends AcetateExtrudedPoint {
	/**
	 * @constructor AcetateSolidExtrusion(target: Platina, opts?: AcetateSolidExtrusion Options)
	 */
	constructor(
		target,
		{
			/// @option feather: Number = 1.5
			/// The feather distance (in CSS pixels)
			feather = 1.5,

			...opts
		} = {}
	) {
		super(target, { zIndex: 2500, ...opts });

		this._attrs = new this.glii.InterleavedAttributes(
			{
				usage: this.glii.STATIC_DRAW,
				size: 1,
				growFactor: 1.2,
			},
			[
				{
					// RGBA colour
					glslType: "vec4",
					type: Uint8Array,
					normalized: true,
				},
				{
					// Feathering value (extrusion distance) plus feather limit
					// (max absolute value of extrusion), as 1/256ths of CSS pixel
					glslType: "vec2",
					type: Int16Array,
					normalized: false,
				},
			]
		);

		this.#feather = feather;
	}

	// Width of feathering, in pixels
	#feather = 0.5;

	/**
	 * @property feather: Number
	 * Read-only getter for the value given to the `feather` option at instantiation time.
	 */
	get feather() {
		return this.#feather;
	}

	glProgramDefinition() {
		const opts = super.glProgramDefinition();
		return {
			...opts,
			attributes: {
				aColour: this._attrs.getBindableAttribute(0),
				aFeather: this._attrs.getBindableAttribute(1),
				...opts.attributes,
			},
			uniforms: {
				uPixelSize: "vec2",
				uFeatherAmount: "float",
				...opts.uniforms,
			},
			vertexShaderMain: `
				vColour = aColour;
				vFeather = aFeather;
				gl_Position = vec4(
					vec3(aCoords, 1.0) * uTransformMatrix +
					vec3(aExtrude * uPixelSize, 0.0)
					, 1.0);
			`,
			varyings: { vColour: "vec4", vFeather: "vec2" },
			fragmentShaderMain: `
				gl_FragColor = vColour;
				float alpha = smoothstep(
					vFeather.y,
					vFeather.y - uFeatherAmount,
					abs(vFeather.x)
				);
				gl_FragColor.a *= alpha;
			`,
		};
	}

	glIdProgramDefinition() {
		const opts = super.glIdProgramDefinition();
		return {
			...opts,
			fragmentShaderMain: `
				if (vColour.a > 0.0) {
					${opts.fragmentShaderMain};
				} else {
					discard;
				}
			`,
		};
	}

	_getStridedArrays(maxVtx, maxIdx) {
		return [
			// Extrusion
			this._extrusions.asStridedArray(maxVtx),
			// Colour
			this._attrs.asStridedArray(0, maxVtx),
			// Feather
			this._attrs.asStridedArray(1),
			// Triangle indices
			this._indices.asTypedArray(maxIdx),
			// // Feather constant
			// this.#feather,
		];
	}

	_commitStridedArrays(baseVtx, vtxCount, baseIdx, idxCount) {
		this._extrusions.commit(baseVtx, vtxCount);
		this._attrs.commit(baseVtx, vtxCount);
		this._indices.commit(baseIdx, idxCount);
	}

	// The map will call resize() on acetates when needed - besides redoing the
	// framebuffer with the new size, this needs to reset the uniform uPixelSize.
	resize(w, h) {
		super.resize(w, h);
		const dpr2 = (devicePixelRatio ?? 1) * 2;
		this._programs.setUniform("uPixelSize", [dpr2 / w, dpr2 / h]);
		// 		this._programs.setUniform("uFeatherAmount", .5 * 256);	// Half a pixel
		this._programs.setUniform("uFeatherAmount", this.#feather * 256);
	}
}
