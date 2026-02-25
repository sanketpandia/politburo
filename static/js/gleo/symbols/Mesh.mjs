import AcetateInteractive from "../acetates/AcetateInteractive.mjs";
import GleoSymbol from "./Symbol.mjs";
import parseColour from "../3rd-party/css-colour-parser.mjs";

/**
 * @class AcetateMesh
 * @inherits AcetateVertices
 *
 * An `Acetate` that draws a simple (single-colour) fill for `Mesh`es.
 *
 */

class AcetateMesh extends AcetateInteractive {
	/**
	 * @constructor AcetateMesh(target: GliiFactory)
	 */
	constructor(target, opts) {
		super(target, { zIndex: 1500, ...opts });

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
					glslType: "vec4",
					type: Uint8Array,
					normalized: true,
				},
			]
		);
	}

	// Pretty much the same shader as AcetateFill
	glProgramDefinition() {
		const opts = super.glProgramDefinition();
		return {
			...opts,
			attributes: {
				...opts.attributes,
				aColour: this._attrs.getBindableAttribute(0),
			},
			vertexShaderMain: `
				vColour = aColour;
				gl_Position = vec4(
					vec3(aCoords, 1.0) * uTransformMatrix, 1.0);
			`,
			varyings: { vColour: "vec4" },
			fragmentShaderMain: `gl_FragColor = vColour;`,
		};
	}

	/**
	 * @method multiAdd(meshes: Array of Mesh): this
	 * Adds the meshes to this acetate (so they're drawn on the next refresh),
	 * using as few WebGL calls as feasible.
	 */
	multiAdd(meshes) {
		// Skip already added symbols
		meshes = meshes.filter((f) => !f._inAcetate);
		if (meshes.length === 0) {
			return;
		}

		const totalVertices = meshes.reduce((acc, mesh) => acc + mesh.attrLength, 0);
		const totalIdxs = meshes.reduce((acc, mesh) => acc + mesh.idxLength, 0);
		let baseVtx = this._attribAllocator.allocateBlock(totalVertices);
		let baseIdx = this._indices.allocateSlots(totalIdxs);
		let vtxAcc = baseVtx;
		let idxAcc = baseIdx;
		const maxVtx = baseVtx + totalVertices;
		const stridedArrays = this._getStridedArrays(
			baseVtx + totalVertices
			// baseIdx + totalIndices
		);

		const stridedIdxs = this._indices.asTypedArray(baseIdx + totalIdxs);

		meshes.forEach((mesh) => {
			mesh.updateRefs(this, vtxAcc, idxAcc);
			this._knownSymbols[vtxAcc] = mesh;

			mesh._setGlobalStrides(...stridedArrays);
			stridedIdxs.set(mesh.triangles.map((n) => n + idxAcc));

			vtxAcc += mesh.attrLength;
			idxAcc += mesh.idxLength;
		});

		this._commitStridedArrays(baseVtx, totalVertices);
		this._indices.commit(baseIdx, totalIdxs);

		if (!this._crs) {
			// Fill symbols have been added before setting a CRS. The CRS of the first
			// Fill symbol shall be used temporarily.
			this._oldCrs = this._crs = meshes[0].geom.crs;
		}

		const coordData = this.reproject(baseVtx, totalVertices, meshes);

		super.multiAddIds(meshes, baseVtx);

		return super.multiAdd(meshes);
	}

	_getStridedArrays(maxVtx, _maxIdx) {
		return [
			// Colour
			this._attrs.asStridedArray(0, maxVtx),
		];
	}

	_commitStridedArrays(baseVtx, vtxLength /*, baseIdx, totalIndices*/) {
		this._attrs.commit(baseVtx, vtxLength);
	}

	_getGeometryStridedArrays() {
		return [];
	}

	_commitGeometryStridedArrays(_baseVtx, _vtxCount, _baseIdx, _idxCount) {
		// noop
	}
}

/**
 * @class Mesh
 * @inherits GleoSymbol
 * @relationship drawnOn AcetateMesh
 *
 * Displays a mesh of connected points, each of them with a RGB(A) colour,
 * performing linear interpolation.
 *
 * This is a mesh in the geospatial sense of the word (see e.g.
 * [MDAL](https://www.mdal.xyz/)). It is **not** a mesh in the 3D computer
 * graphics sense of the word (since 3D graphics usually imply that a mesh
 * includes shaders or materials, see e.g.
 * [a threeJS mesh](https://threejs.org/docs/#api/en/objects/Mesh)). In
 * particular, all Gleo `Mesh`es are rendered using the same shader.
 *
 * If your mesh data does not contain RGB(A) values for each point, consider
 * using a symbol decorator such as `intensify`.
 *
 * @example
 *
 * ```
 * const mesh = new Mesh(
 * 	// The geometry shall be interpreted like a multipoint; rings are ignored.
 * 	[
 * 		[5, 5],
 * 		[7, 3],
 * 		[6, 8],
 * 		[10, 12],
 * 	],
 *
 * 	// The colours are assigned to the points in the geometry on a one-to-one basis
 * 	['red', 'blue', 'green', 'yellow'],
 *
 * 	// The triangles are defined in a single array. Each set of 3 point indices
 * 	// (0-indexed, relative to the geometry) defines a triangle.
 * 	[0, 1, 2,    0, 1, 3],
 *
 * 	// Obviously accepts options from GleoSymbol
 * 	{
 * 		interactive: true,
 * 		attribution: "FooBar"
 * 	}
 * ).addTo(gleoMap);
 * ```
 *
 */

export default class Mesh extends GleoSymbol {
	/// @section Static properties
	/// @property Acetate: Prototype of AcetateMesh
	// The `Acetate` class that draws this symbol.
	static Acetate = AcetateMesh;

	#values;
	#triangles;

	/**
	 * @constructor Mesh(geom: Geometry, values: Array of Colour, triangles: array of Number, opts?: Mesh Options)
	 */
	constructor(
		geom,
		{
			/**
			 * @option values: Array of Colour
			 * The colours for the points in the mesh, one value per point.
			 */
			values = [],
			...opts
		} = {},
		triangles
	) {
		super(geom, opts);

		this.#values = values.map(this.constructor._parseColour).flat();

		this.#triangles = triangles;

		this.attrLength = this.geom.coords.length / this.geom.dimension;
		this.idxLength = this.#triangles.length;
	}

	get triangles() {
		return this.#triangles;
	}
	get values() {
		return this.#values;
	}

	// Can be overriden by subclasses or the `intensify` decorator
	static _parseColour = parseColour;

	_setGlobalStrides(stridedColour, ..._strides) {
		stridedColour.set(this.values, this.attrBase);
	}

	_setGeometryStrides() {
		/* noop */
	}
	_setPerPointStrides(_n, _pointType, _vtx, _vtxCount, _geom, ..._strides) {
		// Noop
	}
}
