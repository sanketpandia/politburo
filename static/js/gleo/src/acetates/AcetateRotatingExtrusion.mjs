import AcetateSolidExtrusion from "./AcetateSolidExtrusion.mjs";

/**
 * @class AcetateRotatingExtrusion
 * @inherits AcetateSolidExtrusion
 *
 * An `Acetate` for rendering solid extrusions that rotate around the
 * point geometry.
 *
 */

export default class AcetateRotatingExtrusion extends AcetateSolidExtrusion {
	constructor(target, opts) {
		super(target, { zIndex: 2800, opts });

		this._attrs = new this.glii.InterleavedAttributes(
			{
				usage: this.glii.STATIC_DRAW,
				size: 1,
				growFactor: 1.2,
			},
			[
				{
					// Dynamic extrusion amount
					glslType: "vec2",
					type: Float32Array,
					normalized: false,
				},
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
				{
					// Rotation speed (revolutions per second)
					glslType: "float",
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
				aRotateExtrude: this._attrs.getBindableAttribute(0),
				aColour: this._attrs.getBindableAttribute(1),
				aFeather: this._attrs.getBindableAttribute(2),
				aSpeed: this._attrs.getBindableAttribute(3),
			},
			uniforms: {
				uNow: "float",
				...opts.uniforms,
			},
			vertexShaderMain: `
				vColour = aColour;
				vFeather = aFeather;
				float angle = aSpeed * uNow / (1000. / 3.14159);
				float sinA = sin(angle);
				float cosA = cos(angle);
				mat2 rotationMatrix = mat2(sinA, -cosA, cosA, sinA);
				gl_Position = vec4(
					vec3(aCoords, 1.0) * uTransformMatrix +
					vec3(
						((rotationMatrix * aRotateExtrude) + aExtrude)
						* uPixelSize, 0.0)
					, 1.0);
			`,
		};
	}

	_getStridedArrays(maxVtx, maxIdx) {
		return [
			// Static extrusion (offset)
			this._extrusions.asStridedArray(maxVtx),
			// Rotating extrusion
			this._attrs.asStridedArray(0, maxVtx),
			// Colour
			this._attrs.asStridedArray(1),
			// Feather
			this._attrs.asStridedArray(2),
			// Speed
			this._attrs.asStridedArray(3),
			// Triangle indices
			this._indices.asTypedArray(maxIdx),
			// Feather constant
			this.feather,
		];
	}

	redraw() {
		this._programs.setUniform("uNow", performance.now());
		return super.redraw.apply(this, arguments);
	}

	// An animated Acetate is always dirty, meaning it wants to render at every
	// frame.
	get dirty() {
		return true;
	}
	set dirty(_) {}
}
