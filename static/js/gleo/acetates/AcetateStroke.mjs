import AcetateInteractive from "./AcetateInteractive.mjs";

/**
 * @class AcetateStroke
 * @inherits AcetateVertices
 *
 * An `Acetate` that draws line strokes.
 */

export default class AcetateStroke extends AcetateInteractive {
	#miterLimit;

	/**
	 * @constructor AcetateStroke(glii: GliiFactory, opts: AcetateStroke Options)
	 */
	constructor(
		glii,
		{
			/** @section AcetateStroke Options
			 * @option miterLimit: Number = 10
			 * Maximum value for the extrusion factor in miter line joints.
			 * Note this is not the same behaviour as 2D miter limit, which
			 * replaces miter joins with bevel joins.
			 */
			miterLimit = 10,
			...opts
		} = {}
	) {
		super(glii, { zIndex: 3000, ...opts });

		this.#miterLimit = miterLimit;

		// Non-geometric attributes - the ones that don't change with a full
		// reprojection
		this._attrs = new this.glii.InterleavedAttributes(
			{
				size: 1,
				growFactor: 1.2,
				usage: this.glii.STATIC_DRAW,
			},
			[
				{
					// RGBA Colour
					glslType: "vec4",
					type: Uint8Array,
					normalized: true,
				},
				{
					// (Accumulated) dash array, with up to 4 elements.
					glslType: "vec4",
					type: Uint8Array,
					normalized: false,
				},
				// TODO: antialias feather (or make it an Acetate uniform)
			]
		);

		// Geometric attributes - the ones that change with a full reprojection
		this._geomAttrs = new this.glii.InterleavedAttributes(
			{
				size: 1,
				growFactor: 1.2,
				usage: this.glii.STATIC_DRAW,
			},
			[
				{
					// Vertex extrusion amount
					glslType: "vec2",
					type: Float32Array,
					normalized: false,
				},
				{
					// Distance to stroke start, in CRS units
					// Used for dashing
					glslType: "float",
					type: Float32Array,
					normalized: false,
				},
				{
					// Data for shortening extrusion in the inner vertex of
					// joins (if not an inner vertex, values are zero):
					// - Length of the shortest adjacent segment, in CRS units
					// - Ratio between half stroke width and extrusion length
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
				aColour: this._attrs.getBindableAttribute(0),
				aExtrude: this._geomAttrs.getBindableAttribute(0),
				aDashArray: this._attrs.getBindableAttribute(1),
				aAccLength: this._geomAttrs.getBindableAttribute(1),
				aInnerAdjustment: this._geomAttrs.getBindableAttribute(2),
				...opts.attributes,
			},
			uniforms: {
				uPixelSize: "vec2",
				uScale: "float",
				...opts.uniforms,
			},
			vertexShaderMain: `
				vColour = aColour;
				vDashArray = aDashArray;
				vAccLength = aAccLength / uScale;

				vec2 extrude = aExtrude;
				if (aInnerAdjustment.x != 0.) {
					// Reduce the length of extrusion on the vertices in the
					// inside of joins, if their extrusions would be larger
					// than the length of an adjacent segment; but never so
					// much that it becomes less than half the stroke width.
					float factor = clamp(
						length(aExtrude) * uScale / aInnerAdjustment.x,
						1.,
						aInnerAdjustment.y
					);
					extrude /= factor;
				}

				gl_Position = vec4(
					vec3(aCoords, 1.0) * uTransformMatrix
					+ vec3(extrude * uPixelSize, 0.0)
					, 1.0);
			`,
			varyings: {
				vColour: "vec4",
				vDashArray: "vec4",
				vAccLength: "float",
				vMiter: "float", // Only for joins: px distance to node
			},
			fragmentShaderMain: `
				float dashIdx = mod(vAccLength, vDashArray.w);
				if (dashIdx <= vDashArray.x) {
					gl_FragColor = vColour;
				} else if (dashIdx <= vDashArray.y) {
					discard;
				} else if (dashIdx <= vDashArray.z) {
					gl_FragColor = vColour;
				} else {
					discard;
				}

				// if (!gl_FrontFacing) {gl_FragColor = vec4(1., 0., 0., .5);}
				if (!gl_FrontFacing) { discard; }
			`,
		};
	}

	// The platina will call resize() on acetates when needed - besides redoing the
	// framebuffer with the new size, this needs to reset the uniform uPixelSize.
	resize(w, h) {
		super.resize(w, h);
		const dpr2 = (devicePixelRatio ?? 1) * 2;
		this._programs.setUniform("uPixelSize", [dpr2 / w, dpr2 / h]);
	}

	runProgram() {
		this._programs.setUniform("uScale", this.platina.scale);
		super.runProgram();
	}

	_getStridedArrays(maxVtx, maxIdx) {
		return [
			// Indices
			// ...super._getStridedArrays(maxVtx, maxIdx),

			// Dash
			this._attrs.asStridedArray(1, maxVtx),

			// All per-point strided arrays
			this._getPerPointStridedArrays(maxVtx, maxIdx),
		];
	}

	_commitStridedArrays(baseVtx, vtxLength, baseIdx, totalIndices) {
		// this._attrs.commit(baseVtx, vtxLength);
		return this._commitPerPointStridedArrays(
			baseVtx,
			vtxLength,
			baseIdx,
			totalIndices
		);
	}

	_getGeometryStridedArrays(maxVtx, maxIdx) {
		return [
			// Indices
			this._indices.asTypedArray(maxIdx),

			// Extrusion (offset in CSS pixels)
			this._geomAttrs.asStridedArray(0, maxVtx),

			// Distance (in CRS units, to first point of each ring)
			this._geomAttrs.asStridedArray(1),

			// Miter limit constant
			1 / this.#miterLimit,

			// per-point strides
			this._getPerPointGeomStridedArrays(maxVtx, maxIdx),
		];
	}

	_getPerPointStridedArrays(maxVtx, _maxIdx) {
		return [
			// Colour
			this._attrs.asStridedArray(0, maxVtx),
		];
	}

	_getPerPointGeomStridedArrays(_maxVtx, _maxIdx) {
		return [];
	}

	_commitGeometryStridedArrays(baseVtx, vtxLength, baseIdx, idxLength) {
		this._indices.commit(baseIdx, idxLength);
		this._geomAttrs.commit(baseVtx, vtxLength);
		this._commitPerPointGeomStridedArrays(baseVtx, vtxLength);
	}
	_commitPerPointStridedArrays(baseVtx, vtxLength) {
		this._attrs.commit(baseVtx, vtxLength);
	}
	_commitPerPointGeomStridedArrays(_baseVtx, _vtxLength) {
		// this._attrs.commit(baseVtx, vtxLength);
	}

	/**
	 * @method multiAdd(strokes: Array of Stroke): this
	 * Adds the strokes to this acetate (so they're drawn on the next refresh),
	 * using as few WebGL calls as feasible.
	 */
	multiAdd(strokes) {
		// Skip already added symbols
		strokes = strokes.filter((s) => isNaN(s.attrBase));

		// Skip strokes with zero indices or vertices, typically strokes with
		// degenerate geometries (with either zero or one points).
		// Instead of just skipping, mark them as belonging to the acetate, so
		// that they may be removed.
		strokes = strokes.filter((s) => {
			let hasData = s.idxLength > 0 && s.attrLength > 0;
			if (!hasData) {
				s._inAcetate = this;
			}
			return hasData;
		});
		if (strokes.length === 0) {
			return;
		}

		const totalIndices = strokes.reduce((acc, stroke) => acc + stroke.idxLength, 0);
		const totalVertices = strokes.reduce((acc, stroke) => acc + stroke.attrLength, 0);

		// There is a degenerate case when only Strokes with one point are
		// added, and would ask for zero indices

		let baseIdx = totalIndices > 0 ? this._indices.allocateSlots(totalIndices) : 0;
		let baseVtx = this._attribAllocator.allocateBlock(totalVertices);
		let idxAcc = baseIdx;
		let vtxAcc = baseVtx;

		let stridedArrays = this._getStridedArrays(
			baseVtx + totalVertices,
			baseIdx + totalIndices
		);
		// const strideColour = this._attrs.asStridedArray(0, baseVtx + totalVertices);
		// const strideDash = this._attrs.asStridedArray(1);

		strokes.forEach((stroke) => {
			stroke._inAcetate = this;
			stroke.attrBase = vtxAcc;
			stroke.idxBase = idxAcc;
			this._knownSymbols[vtxAcc] = stroke;

			stroke._setGlobalStrides(...stridedArrays);

			vtxAcc += stroke.attrLength;
			idxAcc += stroke.idxLength;
		});

		this._commitStridedArrays(baseVtx, totalVertices /*, baseIdx, totalIndices*/);

		if (this._crs) {
			this.reproject(baseVtx, totalVertices, strokes);
		}

		this.dirty = true;
		super.multiAddIds(strokes, baseVtx);
		return super.multiAdd(strokes);
	}

	/**
	 * @method reprojectAll(start: Number, length: Number, skipEarcut: Boolean): Array of Number
	 * As `AcetateVertices.reproject()`, but also recalculates the values for the
	 * attributes which depend on the geometry (including the extrusion amount, which
	 * depends on the linestring angle on each node) and mesh triangulation
	 * (for dextro- or levo-oriented bevel and round joins).
	 */
	reproject(start, length, strokes) {
		//console.log("stroke reproject", start.toString(16), length.toString(16));
		const relevantSymbols =
			strokes ??
			this._knownSymbols.filter((symbol, attrIdx) => {
				return attrIdx >= start && attrIdx + symbol.attrLength <= start + length;
			});

		const coordData = new Float64Array(length * 2);
		const end = start + length;
		// const strideExtrude = this._geomAttrs.asStridedArray(0, end);
		// const strideDistance = this._geomAttrs.asStridedArray(1);
		// const typedIdxs = this._indices.asTypedArray(end);
		// const miterLimit = 1 / this.#miterLimit;

		const [
			typedIdxs,
			strideExtrude,
			strideDistance,
			miterLimit,
			perPointStrides,
			...geometryStrides
		] = this._getGeometryStridedArrays(
			end,
			end // FIXME: calculate max idx, not just max vtx
		);

		let minIdx = Infinity,
			maxIdx = -Infinity,
			vtxLength = 0;

		relevantSymbols.forEach((symbol) => {
			const projectedGeom = symbol.geometry.toCRS(this._crs);
			let addr = (symbol.attrBase - start) * 2;

			projectedGeom.mapRings((start, end, length, r) => {
				if (length === 1) {
					return;
				}

				for (let v = start; v < end; v++) {
					let vtxCount;
					if (v === start) {
						// Start of a ring. Behaves as 2 vertices if the
						// ring loops (join is added at the last and closing
						// vertex instead), as endcap if not.
						vtxCount =
							(projectedGeom.loops[r] ? 2 : symbol.verticesPerEnd) +
							symbol.centerline;
					} else if (v === end - 1) {
						// End of a ring. Idem as start.
						vtxCount =
							(projectedGeom.loops[r]
								? symbol.verticesPerJoin
								: symbol.verticesPerEnd) + symbol.centerline;
					} else {
						// Not start, not end: always a line join
						vtxCount = symbol.verticesPerJoin + symbol.centerline;
					}

					const coord = projectedGeom.coords.slice(v * 2, v * 2 + 2);

					// Store the coordinate once per vertex
					for (let j = 0; j < vtxCount; j++) {
						coordData.set(coord, addr);
						addr += 2;
					}

					vtxLength += vtxCount;
				}
			});

			symbol._setGeometryStrides(
				projectedGeom,
				strideExtrude,
				strideDistance,
				miterLimit,
				perPointStrides,
				typedIdxs,
				...geometryStrides
			);

			minIdx = Math.min(minIdx, symbol.idxBase);
			maxIdx = Math.max(maxIdx, symbol.idxBase + symbol.idxLength);
		});

		this.multiSetCoords(start, coordData);

		// this._geomAttrs.commit(start, vtxLength);

		if (isFinite(minIdx)) {
			// this._indices.commit(minIdx, maxIdx - minIdx);
			this._commitGeometryStridedArrays(start, vtxLength, minIdx, maxIdx - minIdx);
		}

		return coordData;
	}

	reprojectAll() {
		this.reproject(0, this._indices._size, this._knownSymbols);
	}

	destroy() {
		this._geomAttrs.destroy();
		return super.destroy();
	}
}
