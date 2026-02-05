import GleoSymbol from "./Symbol.mjs";
import parseColour from "../3rd-party/css-colour-parser.mjs";

import AcetateVertices from "../acetates/AcetateVertices.mjs";
import { LINECAP } from "../util/pointExtrusionTypeConstants.mjs";

// 90 degrees, in radians
const Δϕ90 = Math.PI / 2;

// 150 degrees, in radians
const Δϕ150 = Math.PI / 1.2;

/**
 * @class AcetateChain
 * @inherits AcetateVertices
 *
 * An `Acetate` that draws lines as `Chain`s of overlapping 2-point segments
 *
 */
class AcetateChain extends AcetateVertices {
	/**
	 * @constructor AcetateChain(target: GliiFactory)
	 */
	constructor(target, opts) {
		super(target, { zIndex: 1500, ...opts });

		// this._indices = new this.glii.SparseIndices({
		// 	type: this.glii.UNSIGNED_INT,
		// 	drawMode: this.glii.POINTS,
		// });

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
					// RGBA Colour
					glslType: "vec4",
					type: Uint8Array,
					normalized: true,
				},
				{
					// Width, in 256ths of CSS pixels.
					// Used for fading.
					glslType: "float",
					type: Uint16Array,
					normalized: false,
				},
			]
		);

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
					// Segment length: lenght at vertex (either 0 or full),
					// and segment lenght.
					// Used for fading. The values will be interpolated in the
					// non-cap triangles of each segment.
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
				aWidth: this._attrs.getBindableAttribute(1),
				aExtrude: this._geomAttrs.getBindableAttribute(0),
				aLength: this._geomAttrs.getBindableAttribute(1),
				...opts.attributes,
			},
			uniforms: {
				uPixelSize: "vec2",
				uScale: "float",
				...opts.uniforms,
			},
			vertexShaderMain: `
				vColour = aColour;
				vLength = aLength / uScale;
				vWidth = aWidth / 512.;

				gl_Position = vec4(
					vec3(aCoords, 1.0) * uTransformMatrix
					+ vec3(aExtrude * uPixelSize, 0.0)
					, 1.0);
			`,
			varyings: {
				vColour: "vec4",
				vLength: "vec2",
				vWidth: "float", // *half* the width
				// vExterior: "float",
				// vDashArray: "vec4",
				// vAccLength: "float",
				// vMiter: "float", // Only for joins: px distance to node
			},
			fragmentShaderMain: `
				gl_FragColor = vColour;

				float position = min(vLength.x, vLength.y - vLength.x);
				float opacity = 0.5 + min(position / vWidth, 1.0) / 2.;

				gl_FragColor.a *= opacity;
			`,
			blend: {
				equationRGB: this.glii.FUNC_ADD,
				equationAlpha: this.glii.FUNC_ADD,

				srcRGB: this.glii.ONE_MINUS_DST_ALPHA,
				srcAlpha: this.glii.ONE,
				dstRGB: this.glii.DST_ALPHA,
				dstAlpha: this.glii.ONE,
			},
		};
	}

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
			this._indices.asTypedArray(maxIdx),

			// Width (for fading)
			this._attrs.asStridedArray(1, maxVtx),
		];
	}

	_getGeometryStridedArrays(maxVtx, maxIdx) {
		return [
			// CRS coords
			this._coords.asStridedArray(maxVtx),

			// Extrusion
			this._geomAttrs.asStridedArray(0, maxVtx),

			// Segment length (and relative length position)
			this._geomAttrs.asStridedArray(1),

			// Point strides
			this._getPerPointStridedArrays(maxVtx, maxIdx),

			// Segment strides
			this._getPerSegmentStridedArrays(maxVtx, maxIdx),
		];
	}

	_commitStridedArrays(baseVtx, vtxLength, baseIdx, idxLength) {
		this._attrs.commit(baseVtx, vtxLength);
		this._indices.commit(baseIdx, idxLength);
	}

	_commitGeometryStridedArrays(baseVtx, vtxLength /*, baseIdx, totalIndices*/) {
		this._geomAttrs.commit(baseVtx, vtxLength);
		this._attrs.commit(baseVtx, vtxLength);
		this._commitPerPointStridedArrays(baseVtx, vtxLength);
	}

	_getPerPointStridedArrays(_maxVtx, _maxIdx) {
		return [];
	}

	_getPerSegmentStridedArrays(maxVtx, _maxIdx) {
		return [
			// Colour
			this._attrs.asStridedArray(0, maxVtx),
		];
	}

	multiAdd(syms) {
		super.multiAdd(syms);
		super.multiAllocate(syms);

		return this;
	}

	reproject(start, length, symbols) {
		const end = start + length;

		// In most cases, it's safe to assume that relevant symbols in the same
		// attribute allocation block have their vertex attributes in a
		// compacted manner.
		// The exception is tiles: tile vertex attributes are allocated in bulk
		// (enough to fill a whole texture atlas), before actually instantiating
		// tile symbols. Tile acetates shall overload this method.

		const stridedCoords = this._coords.asStridedArray(end);
		const geomStrides = this._getGeometryStridedArrays(end);

		const relevantSymbols =
			symbols ??
			this._knownSymbols.filter((symbol, attrIdx) => {
				return attrIdx >= start && attrIdx + symbol.attrLength <= start + length;
			});

		relevantSymbols.forEach((s) => {
			const geom = s.geometry.toCRS(this._crs);
			// stridedCoords.set(geom.coords, s.attrBase);
			s._setGeometryStrides(geom, ...geomStrides);
		});

		this._coords.commit(start, length);
		this._commitGeometryStridedArrays(start, length);

		const coordData = new Float32Array(stridedCoords.buffer, start * 8, length * 2);
		super.expandBBox(coordData);
		return coordData;
	}
}

/**
 * @class Chain
 * @inherits GleoSymbol
 * @relationship drawnOn AcetetateChain
 *
 * Draws line geometries as a set of overlapping 2-point segments.
 *
 * Behaves similar to `Stroke` symbols, but handles the line joins ("corners")
 * differently: instead of calculating joins, corner points are drawn twice
 * at half the opacity.
 *
 * Compared with `Stroke`s, `Chain`s produce less graphical artefacts when
 * drawing thick, short lines. The downside is reduced fidelity for corners
 * between long segments.
 *
 */

export default class Chain extends GleoSymbol {
	static Acetate = AcetateChain;

	#colour;
	#width;
	// #dashArray;
	// #centerline;

	/**
	 * @constructor Chain(geom: Geometry, opts?: Chain Options)
	 */
	constructor(
		geom,
		{
			/**
			 * @section
			 * @aka Stroke Options
			 * @option colour: Colour = '#3388ff'
			 * The colour of the chain.
			 * @alternative
			 * @option colour: Array of Colour
			 * The colour of each segment of the chain. There must be enough elements.
			 */
			colour = "#3388ff",
			/**
			 * @option width: Number = 4
			 * The width of the chain, in CSS pixels
			 */
			width = 4,

			...opts
		} = {}
	) {
		super(geom, opts);

		this.#calcStorage();

		this.#colour = this.constructor._parseColour(colour);
		if (this.#colour === null && Array.isArray(colour)) {
			this.#colour = colour.map(this.constructor._parseColour);
		}

		this.#width = width;
	}

	#segmentCount;

	#calcStorage() {
		const segmentCount = (this.#segmentCount =
			this.geometry.coords.length / this.geometry.dimension -
			this.geometry.rings.length -
			this.geometry.hulls.length -
			1);

		// Each segment has 10 vertices and 10 triangles (30 triangle primitive indices)
		this.attrLength = segmentCount * 10;
		this.idxLength = segmentCount * 30;
	}

	_setGlobalStrides(typedIdxs, strideWidth) {
		/*
		 * Vertices connect as follows, 1 and 6 being the offset-zero points
		 * of the segment.
		 *
		 *      0---5
		 *     /|\  |\
		 *    3 | \ | 8
		 *    |\|  \|/|
		 *    | 1---6 |
		 *    |/|\  |\|
		 *    4 | \ | 9
		 *     \|  \|/
		 *      2---7
		 *
		 * (This is compatible with the LINECAP point extrusion type: line caps have
		 * the centerline at the 2nd (offset 1) vertex).
		 */

		// prettier-ignore
		const idxMap = [
			1, 0, 3,
			1, 3, 4,
			1, 4, 2,
			1, 6, 0,
			0, 6, 5,
			1, 7, 6,
			1, 2, 7,
			6, 8, 5,
			6, 9, 8,
			6, 7, 9,
		];

		let idx = this.idxBase;

		for (let i = 0; i < this.#segmentCount; i++) {
			const offset = this.attrBase + i * 10;
			typedIdxs.set(
				idxMap.map((n) => n + offset),
				idx
			);
			idx += 30;
		}

		let w = this.#width * 256;
		for (let i = 0; i < this.attrLength; i++) {
			strideWidth.set([w], this.attrBase + i);
		}
	}

	_setGeometryStrides(
		geom,
		strideCoords,
		strideExtrude,
		strideLength,
		perPointStrides,
		perSegmentStrides
	) {
		const w = this.#width / 2;

		geom.mapRings((start, end, _length, _r) => {
			for (let i = start + 1; i < end; i++) {
				const coordAx = geom.coords[(i - 1) * geom.dimension];
				const coordAy = geom.coords[(i - 1) * geom.dimension + 1];
				const coordBx = geom.coords[i * geom.dimension];
				const coordBy = geom.coords[i * geom.dimension + 1];

				const Δx = coordBx - coordAx;
				const Δy = coordBy - coordAy;
				const ϕ = Math.atan2(Δy, Δx);

				// Plus 90 degrees counter-clockwise
				const ϕ90 = ϕ + Δϕ90;
				const cosϕ90 = w * Math.cos(ϕ90);
				const sinϕ90 = w * Math.sin(ϕ90);

				// Plus 150 degrees counter-clockwise
				const ϕ150 = ϕ + Δϕ150;
				const cosϕ150 = w * Math.cos(ϕ150);
				const sinϕ150 = w * Math.sin(ϕ150);

				// Plus 210 degrees counter-clockwise
				const ϕ210 = ϕ - Δϕ150;
				const cosϕ210 = w * Math.cos(ϕ210);
				const sinϕ210 = w * Math.sin(ϕ210);

				const vtx = this.attrBase + i * 10 - 10;

				strideExtrude.set([cosϕ90, sinϕ90], vtx + 0);
				strideExtrude.set([0, 0], vtx + 1);
				strideExtrude.set([-cosϕ90, -sinϕ90], vtx + 2);
				strideExtrude.set([cosϕ150, sinϕ150], vtx + 3);
				strideExtrude.set([cosϕ210, sinϕ210], vtx + 4);

				strideExtrude.set([cosϕ90, sinϕ90], vtx + 5);
				strideExtrude.set([0, 0], vtx + 6);
				strideExtrude.set([-cosϕ90, -sinϕ90], vtx + 7);
				strideExtrude.set([-cosϕ210, -sinϕ210], vtx + 8);
				strideExtrude.set([-cosϕ150, -sinϕ150], vtx + 9);

				// prettier-ignore
				strideCoords.set([
					coordAx, coordAy,
					coordAx, coordAy,
					coordAx, coordAy,
					coordAx, coordAy,
					coordAx, coordAy,

					coordBx, coordBy,
					coordBx, coordBy,
					coordBx, coordBy,
					coordBx, coordBy,
					coordBx, coordBy,
				], vtx);

				// Length of segment
				const l = Math.sqrt(Δx * Δx + Δy * Δy);

				// Five first vertices are at position zero, five last ones
				// are at position 100% length
				for (let i = 0; i < 5; i++) {
					strideLength.set([0, l], vtx + i);
				}
				for (let i = 5; i < 10; i++) {
					strideLength.set([l, l], vtx + i);
				}

				/// TODO: Trick lengths at first and last point in the geometry
				/// ring, unless the ring loops

				this._setPerSegmentStrides(
					i - 1,
					this.attrBase + i * 10 - 10,
					10,
					geom,
					...perSegmentStrides
				); /// TODO!!!!

				this._setPerPointStrides(
					i - 1,
					LINECAP,
					this.attrBase + i * 10 - 10,
					5,
					...perPointStrides
				);
				this._setPerPointStrides(
					i,
					LINECAP,
					this.attrBase + i * 10 - 5,
					5,
					...perPointStrides
				);
			}
		});
	}

	_setPerPointStrides(_n, _pointType, _vtx, _vtxCount, _geom, ..._strides) {
		// noop
	}

	_setPerSegmentStrides(n, vtx, vtxCount, _geom, strideColour) {
		const segmentColour =
			Array.isArray(this.#colour) && Array.isArray(this.#colour[0])
				? this.#colour[n]
				: this.#colour;
		for (let i = 0; i < vtxCount; i++) {
			strideColour.set(segmentColour, vtx + i);
		}
	}

	static _parseColour = parseColour;
}
