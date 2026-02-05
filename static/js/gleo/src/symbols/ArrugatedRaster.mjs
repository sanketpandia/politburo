import ConformalRaster from "./ConformalRaster.mjs";

import { project } from "../crs/projector.mjs";
import Acetate from "../acetates/Acetate.mjs";
import Arrugator from "../arrugator/arrugator.mjs";
import parseColour from "../3rd-party/css-colour-parser.mjs";

/**
 * @class AcetateArrugatedRaster
 * @inherits AcetateConformalRaster
 *
 * An `Acetate` that draws warped, reprojected (AKA "arrugated")
 * RGB(A) raster images.
 *
 * Injection of the `proj4js` dependency should be done **before** instantiating
 * any `ArrugatedRaster`s.
 */

/// BIG TODO:
/// Allow deleting ArrugatedRasters
/// Allow reprojecting (trigger full re-arrugating of all symbols)

class AcetateArrugatedRaster extends ConformalRaster.Acetate {
	#wireframeColour;
	#wireframeIndices;
	#wireframeProgram;

	/**
	 * @constructor AcetateArrugatedRaster(target: GliiFactory, opts: AcetateArrugatedRaster Options)
	 */
	constructor(
		target,
		{
			/**
			 * @section AcetateArrugatedRaster Options
			 * @option wireframeColour: undefined = undefined
			 * Disables wireframe rendering. This is the default.
			 * @option wireframeColour: Colour
			 * Enables wireframe rendering. Wireframe will have the specified
			 * solid colour. Wireframe rendering is useful for debugging but can
			 * negatively impact performance.
			 */
			wireframeColour = undefined,
			...opts
		} = {}
	) {
		super(target, { zIndex: -1900, ...opts });

		if (wireframeColour) {
			this.#wireframeColour = parseColour(wireframeColour);
			this.#wireframeIndices = new this.glii.WireframeTriangleIndices({
				type: this.glii.UNSIGNED_INT,
			});
		}

		// Redefine the UV attribute buffer since arrugated rasters will have
		// non-integer UV coordinates between 0 and 1
		this._uv = new this.glii.SingleAttribute({
			size: 1,
			growFactor: 1.2,
			usage: this.glii.STATIC_DRAW,
			glslType: "vec2",
			type: Float32Array,
			normalized: false,
		});
	}

	multiAllocate(symbols, loadedImages) {
		/// Calculates data for each raster:
		/// - The initial epsilon, and LoD
		/// - The minimum epsilon, and LoD (less than half a *raster* pixel, naïvely)
		/// Then, runs `arrugate` for each LoD

		const platinaCrs = this.platina.crs;
		if (!platinaCrs) {
			return;
		}

		let vtxCount = 0;

		const promises = symbols.map((raster, i) => {
			/// TODO: Most of this should be moved to reproject(), somehow.
			const image = loadedImages[i];
			const rasterCrs = raster.geom.crs;
			const coords = raster.geom.coords;

			// Data for the CRS coords attribute. Its values will be appended by
			// arrugator.
			const pos = [
				[coords[0], coords[1]],
				[coords[2], coords[3]],
				[coords[4], coords[5]],
				[coords[6], coords[7]],
			];

			// Data for the UV attribute. Its values will be appended by arrugator.
			// Note this defines the needed order of vertices
			const uv = [
				[0, 0],
				[0, 1],
				[1, 0],
				[1, 1],
			];

			// Data for the triangle indices. Its values will be appended by arrugator.
			const trigs = [
				[0, 1, 3],
				[0, 3, 2],
			];

			const arruga = new Arrugator(
				(coord) => project(rasterCrs.name, platinaCrs.name, coord),
				pos,
				uv,
				trigs
			);

			let arrugado;
			let minLoD, maxLoD;
			// Will store the indices data, to allocate later
			const idxs = {};
			// Will store indices allocations, one per LoD
			const baseIdxs = {};
			// Will store length of the indices per LoD, to load into symbol
			const idxLengths = {};

			if (rasterCrs.name === platinaCrs.name) {
				// Edge case: for equal CRSs, skip arrugation and use a stub instead.
				arrugado = arruga.output();
				minLoD = 0;
				maxLoD = 0;
				idxs[0] = arrugado.trigs;
			} else {
				// The minimum LoD corresponds to the scale which is equal(ish)
				// to the initial epsilon - which means that, at that scale, the
				// un-arrugated raster displays with less than a pixel of distortion.
				minLoD = Math.ceil(Math.log2(Math.sqrt(arruga.epsilon)));

				// Naïve calculation of the maximum LoD - the goal is to stop
				// calculating subdivisions that are smaller than a raster's pixel.
				/// TODO: Is there a better method to calculate the maxLoD???
				const imageSize = Math.max(image.width, image.height);
				maxLoD = Math.floor(Math.log2(Math.sqrt(arruga.epsilon) / imageSize));

				for (let i = 0; i < raster.forces; i++) {
					arruga.force();
				}

				for (let lod = minLoD; lod >= maxLoD; lod--) {
					const epsilon = Math.pow(2, lod) ** 2;

					arruga.epsilon = epsilon;

					arrugado = arruga.output();
					idxs[lod] = arrugado.trigs;
				}
			}

			const baseAttr = this._attribAllocator.allocateBlock(
				(raster.attrLength = uv.length)
			);

			for (let lod = minLoD; lod >= maxLoD; lod--) {
				/// TODO: Allocate all of the index slots at once??????????
				/// That would have the benefit of being more "compatible" in
				/// the symbol (i.e. the symbol has one single contigous block
				/// of indices), but otherwise the LoD index data ("where do
				/// each LoD start?") needs to be kept inside the symbol as well.
				const len = (idxLengths[lod] = idxs[lod].length * 3);
				baseIdxs[lod] = this._indices.allocateSlots(len);
				this._indices.set(
					baseIdxs[lod],
					idxs[lod].flat().map((i) => i + baseAttr)
				);
				this.#wireframeIndices?.set(
					baseIdxs[lod],
					idxs[lod].flat().map((i) => i + baseAttr)
				);
			}

			raster.updateRefs(this, baseAttr, baseIdxs[minLoD], minLoD, maxLoD);
			raster._idxLengths = idxLengths;
			raster._lodBaseIdxs = baseIdxs;

			this._uv.multiSet(baseAttr, uv.flat());
			this.multiSetCoords(baseAttr, arrugado.projected.flat());

			this._knownSymbols[baseAttr] = raster;

			// Unlike other acetates, ArrugatedRaster does not allocate space
			// for all symbols at once, but on a one-by-one basis
			const stridedArrays = this._getStridedArrays(
				baseAttr + raster.attrLength
				// baseIdx + totalIndices
			);

			symbols.map((raster, _i) => {
				raster._setGlobalStrides(...stridedArrays);
			});

			this._commitStridedArrays(
				baseAttr,
				raster.attrLength /*, baseIdx, totalIndices*/
			);

			// // Skip AcetateConformalRaster functionality and go directly to
			// // Acetate functionality.
			// this.fire("symbolsadded", { symbols: symbols });

			super.multiAddIds([raster], baseAttr);

			// Call grandparent
			Acetate.prototype.multiAdd.call(this, [raster]);

			// Return dump texture promise
			return raster.buildTexture(this.glii, image);
		});

		// The raster data might take time to be dumped into textures
		// (especifically for GeoTIFFs, since reading a GeoTIFF is async).
		// Therefore, re-dirty the acetate when the textures are ready.
		Promise.all(promises).then(() => (this.dirty = true));

		this.dirty = true;

		return this;
	}

	resize(x, y) {
		super.resize(x, y);

		if (this.#wireframeColour && !this.#wireframeProgram) {
			const def = this.glProgramDefinition();
			const opts = {
				...def,
				fragmentShaderMain: `gl_FragColor = uWireframeColour;`,
				uniforms: {
					...def.uniforms,
					uWireframeColour: "vec4",
				},
				indexBuffer: this.#wireframeIndices,
				blend: false,
			};
			opts.vertexShaderSource += opts.vertexShaderMain
				? `\nvoid main(){${opts.vertexShaderMain}}`
				: "";
			opts.fragmentShaderSource += opts.fragmentShaderMain
				? `\nvoid main(){${opts.fragmentShaderMain}}`
				: "";
			this.#wireframeProgram = new this.glii.WebGL1Program(opts);
			this.#wireframeProgram.setUniform(
				"uWireframeColour",
				this.#wireframeColour.map((b) => b / 255)
			);
			this._programs.addProgram(this.#wireframeProgram);
		}
		return this;
	}

	/**
	 * Redefinition of the default. Render must happen once per raster, in order
	 * to load the appropriate textures; also the current CRS scale defines
	 * which LoD to choose from, once clamped to the symbol's min/max.
	 */
	runProgram() {
		const lod = Math.floor(Math.log2(this.platina.scale));

		this._knownSymbols.forEach((r) => {
			this._programs.setTexture("uRasterTexture", r.texture);

			// Slightly confusing since minLoD > maxLoD (simpler LoDs are higher)
			const clampLoD = Math.min(Math.max(lod, r._maxLoD), r._minLoD);

			this._programs.runPartial(r._lodBaseIdxs[clampLoD], r._idxLengths[clampLoD]);
		});
	}

	// A full reprojection is easier to handle with a "remove everything, add
	// everything" approach.
	reprojectAll() {
		if (this._crs.name !== this._oldCrs.name) {
			const loadedRasters = this._knownSymbols.slice();
			this.multiDeallocate(this._knownSymbols);
			this._knownSymbols = [];
			this.multiAdd(loadedRasters);
		} else {
			super.reprojectAll();
		}
	}

	/// TODO: This only handles reprojection between CRSs with the same name
	/// (offsets).
	/// (Reprojecting between CRSs with different name requires a recalculation
	/// of **all** data for the given rasters. Doing this here leads to problems
	/// due to modifying an allocation map inside a `forEach()` call.
	reproject(start, length) {
		if (this._crs.name !== this._oldCrs.name) {
			if (Object.keys(this._oldCrs).length === 0) {
				// Happens at the first render. The data is already projected
				// in the platina's CRS during `_syncMultiAdd` (which has run earlier).
				return;
			} else {
				throw new Error(
					"Reprojecting an ArrugatedRaster is unimplemented. Sorry."
				);
			}
		} else {
			// Manual offsetting of points.
			/// TODO: Should be applied for the general case, instead of
			/// per-symbol reprojection???

			const fromOffset = this._oldCrs?.offset ?? [0, 0];
			const toOffset = this._crs?.offset ?? [0, 0];
			const offsetX = toOffset[0] - fromOffset[0];
			const offsetY = toOffset[1] - fromOffset[1];

			let coordSlice = new Float32Array(
				this._coords._byteData.buffer,
				start * 8, // Each item is 2 4-byte floats, so 8 bytes per.
				length * 2
			);

			coordSlice = coordSlice.map((xy, i) => (i % 2 ? xy - offsetY : xy - offsetX));

			this.multiSetCoords(start, coordSlice);
		}
	}

	// Custom implementation of multiDeallocate. The base implementation in
	// AcetateVertices assumes a symbol has only one indices allocation block; but
	// ArrugatedRasters have several (one per LoD).
	multiDeallocate(symbols) {
		symbols.forEach((symbol) => {
			const lods = Object.keys(symbol.idxBase).sort();
			lods.forEach((lod) => {
				this._indices.deallocateSlots(
					symbol._lodBaseIdxs[lod],
					symbol._idxLenghts[lod]
				);
			});
		});

		super.multiDeallocate(symbols);
	}
}

/**
 * @class ArrugatedRaster
 * @inherits ConformalRaster
 * @relationship dependsOn AcetateArrugatedRaster
 *
 * A warped, reprojected (AKA "arrugated") RGB(A) raster image.
 *
 * It's used just as a `ConformalRaster` would, but will work properly when being
 * reprojected - i.e. when the CRS of the raster's geometry is different from
 * the map's CRS.
 */

export default class ArrugatedRaster extends ConformalRaster {
	/// @section Static properties
	/// @property Acetate: Prototype of AcetateArrugatedRaster
	// The `Acetate` class that draws this symbol.
	static Acetate = AcetateArrugatedRaster;

	#forces = 0;

	constructor(
		geom,
		image,
		{
			/**
			 * @option forces: Number = 0
			 * The amount of times to force arrugator split for all segments.
			 * The default of zero should work for rasters with a relative small
			 * coverage. Rasters who span the whole globe and produce artefacts
			 * might arrugate better with values between 1 and 4.
			 */
			forces = 0,
			...opts
		} = {}
	) {
		super(geom, image, opts);
		this.attrLengths = {};
		this._idxLengths = {};
		this._lodBaseIdxs = {};

		this.#forces = forces;

		// Lengths are stored elsewhere. By setting this to zero,
		// the parent functionality for deallocating symbols will still work.
		this.idxLength = 0;
	}

	get forces() {
		return this.#forces;
	}

	/**
	 * @section Acetate Interface
	 * @method updateRefs(ac: AcetateConformalRaster, atb: Number, idx: Object of Number to Number, minLoD: Number, maxLoD: Number): this
	 * Internal usage only, called from `AcetateConformalRaster`.
	 *
	 * An arrugated raster has *several* index slots - one per level of detail
	 * (AKA "zoom level"). Those indices refer to the same set of vertex attributes
	 * (i.e. the vertices and their data are completely shared between LoDs).
	 *
	 * This method updates the acetate that this raster is being currently drawn
	 * on, the base vertex attribute slot**s** (`atb`), the base vertex
	 * index slot**s** (`idx`), and the minimum/maximum LoD (Level of Detail) for
	 * the data structures (each LoD maps to a CRS scale value, so the acetate
	 * can chose which set of triangles to render at a given scale).
	 */
	updateRefs(ac, atb, idx, minLoD, maxLoD /*, arrugator*/) {
		this._minLoD = minLoD;
		this._maxLoD = maxLoD;
		// this._arrugator = arrugator;

		/// TODO: Should handle idxLengths and lodBaseIdxs as well.

		return super.updateRefs(ac, atb, idx);
	}

	/// TODO!!!
	remove() {
		throw new Error("Unimplemented.");
	}
}
