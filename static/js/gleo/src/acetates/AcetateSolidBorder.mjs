import AcetateExtrudedPoint from "./AcetateExtrudedPoint.mjs";

/**
 * @class AcetateSolidBorder
 * @inherits AcetateExtrudedPoint
 *
 * An `Acetate` for solid extrusions with two colours: fill and border.
 *
 * Used in `HeadingTriangle` and `Circle`.
 *
 */
export default class AcetateSolidBorder extends AcetateExtrudedPoint {
	constructor(target, opts) {
		super(target, { zIndex: 2500, opts });

		this._attrs = new this.glii.InterleavedAttributes(
			{
				usage: this.glii.STATIC_DRAW,
				size: 1,
				growFactor: 1.2,
			},
			[
				{
					// Fill RGBA colour
					glslType: "vec4",
					type: Uint8Array,
					normalized: true,
				},
				{
					// Border RGBA colour
					glslType: "vec4",
					type: Uint8Array,
					normalized: true,
				},
				{
					// Border width and feather width (in CSS pixels)
					glslType: "vec2",
					type: Float32Array,
					normalized: false,
				},
				{
					// Per-vertex distance from farthest edge. In a HeadingTriangle,
					// each vertex will have values like N-0-0, 0-N-0, 0-0-N.
					// In a Circle, all values are the same.
					// Units should be CSS pixels.
					glslType: "vec3",
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
				aFillColour: this._attrs.getBindableAttribute(0),
				aBorderColour: this._attrs.getBindableAttribute(1),

				// Per-triangle border and feather width
				aBorder: this._attrs.getBindableAttribute(2),

				// Per-vertex distance to edge
				aEdge: this._attrs.getBindableAttribute(3),

				...opts.attributes,
			},
			uniforms: {
				uPixelSize: "vec2",
				...opts.uniforms,
			},
			vertexShaderMain: `
				vFillColour = aFillColour;
				vBorderColour = aBorderColour;
				vBorder = aBorder;
				vEdge = aEdge;

				gl_Position = vec4(
						vec3(aCoords, 1.0) * uTransformMatrix +
						vec3(aExtrude * uPixelSize, 0.0)
						, 1.0);

			`,
			varyings: {
				vFillColour: "vec4",
				vBorderColour: "vec4",
				vBorder: "vec2",
				vEdge: "vec3",
			},
			fragmentShaderMain: `
				float edgeDistance = min(min(vEdge.x, vEdge.y), vEdge.z);

				if (edgeDistance < vBorder.x) {
					gl_FragColor = vBorderColour;
					gl_FragColor.a *= min(1., edgeDistance / vBorder.y);
				} else {
					// gl_FragColor = vFillColour /** edgeDistance / 16.*/;
					gl_FragColor = mix(vBorderColour, vFillColour, min(edgeDistance - vBorder.y / vBorder.x, 1.0));
				}

				// gl_FragColor.rgb = vEdge / 16.;
				// gl_FragColor.a = 1.;
			`,
		};
	}

	_getStridedArrays(maxVtx, maxIdx) {
		return [
			// Extrusion
			this._extrusions.asStridedArray(maxVtx),
			// Fill colour
			this._attrs.asStridedArray(0, maxVtx),
			// Border colour
			this._attrs.asStridedArray(1),
			// Border+feather
			this._attrs.asStridedArray(2),
			// Distance to edge
			this._attrs.asStridedArray(3),
			// Triangle indices
			this._indices.asTypedArray(maxIdx),
		];
	}

	// The map will call resize() on acetates when needed - besides redoing the
	// framebuffer with the new size, this needs to reset the uniform uPixelSize.
	resize(w, h) {
		super.resize(w, h);
		const dpr2 = (devicePixelRatio ?? 1) * 2;
		this._programs.setUniform("uPixelSize", [dpr2 / w, dpr2 / h]);
	}
}
