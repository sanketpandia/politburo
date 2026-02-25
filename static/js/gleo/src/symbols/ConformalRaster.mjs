import GleoSymbol from "./Symbol.mjs";
import RawGeometry from "../geometry/RawGeometry.mjs";
import { factory } from "../geometry/DefaultGeometry.mjs";
import rasterFactory from "../rasterformats/factory.mjs";
// import imagePromise, {imageToTexture} from "../util/imagePromise.mjs";

import AcetateInteractive from "../acetates/AcetateInteractive.mjs";
import Acetate from "../acetates/Acetate.mjs";

/**
 * @class AcetateConformalRaster
 * @inherits AcetateVertices
 *
 * An `Acetate` that draws rectangular conformal (i.e. matching the display CRS)
 * RGB(A) raster images.
 *
 * Only the four corners of the raster are reprojected when displaying in a
 * different CRS, and pixels are linearly interpolated. For proper raster
 * reprojection, leverage Arrugator instead.
 *
 */

class AcetateConformalRaster extends AcetateInteractive {
	static get PostAcetate() {
		// Allow AcetateConformalRaster to render into both RGBA and
		// scalar fields. When working on a scalar field, only the first
		// channel will be used.
		return Acetate;
	}

	constructor(target, opts) {
		super(target, { zIndex: -2000, ...opts });

		this._uv = new this.glii.SingleAttribute({
			size: 1,
			growFactor: 1.2,
			usage: this.glii.STATIC_DRAW,
			glslType: "vec2",
			type: Uint8Array,
			normalized: false,
		});
	}

	glProgramDefinition() {
		const opts = super.glProgramDefinition();
		return {
			...opts,
			attributes: {
				...opts.attributes,
				aUV: this._uv,
			},
			textures: {
				uRasterTexture: undefined,
			},
			vertexShaderMain: `
				vUV = aUV;
				gl_Position = vec4(
					vec3(aCoords, 1.0) * uTransformMatrix, 1.0);
			`,
			varyings: { vUV: "vec2" },
			fragmentShaderMain: `
				gl_FragColor = texture2D(uRasterTexture, vUV);
				// gl_FragColor.r = 1.0;
			`,
		};
	}

	/**
	 * @method multiAdd(rasters: Array of ConformalRaster): this
	 * Adds the conformal rasters to this acetate (so they're drawn on the next refresh).
	 *
	 * Note this call can be asynchronous - if any of the rasters' images has not loaded
	 * yet, that will delay this whole call until all of the images have loaded.
	 */
	multiAdd(symbols) {
		Promise.all(symbols.map((s) => s.raster))
			.then((loadedRasters) => this.multiAllocate(symbols, loadedRasters))
			.catch((err) => {
				/**
				 * @event rastererror: Event
				 * Fired when some of the rasters to be added to this acetate have failed
				 * to load their image.
				 */
				throw err;
			});

		return this;
	}

	multiAllocate(symbols, loadedRasters) {
		// Skip already added symbols
		symbols = symbols.filter((r) => !r._inAcetate);
		const l = symbols.length;
		if (l === 0) return;

		// These are constant for this particular case, but they could be
		// fetched from the symbol's `attrLength` and `idxLenght` instead.
		const totalIndices = l * 6;
		const totalVertices = l * 4;

		let baseIdx = this._indices.allocateSlots(totalIndices);
		let baseVtx = this._attribAllocator.allocateBlock(totalVertices);
		let idxAcc = baseIdx;
		let vtxAcc = baseVtx;

		// U-V texture coordinates for all symbols. Usual 0-1 corners per group of 4 vertices.
		// prettier-ignore
		this._uv.multiSet(
			baseVtx,
			symbols.map(() => [
				0, 0,
				1, 0,
				1, 1,
				0, 1
			]).flat()
		);

		// Build up two trigs per raster.
		this._indices.set(
			baseIdx,
			symbols
				.map((r, i) => {
					const base = i * 4;

					// prettier-ignore
					return [
						base   , base+1, base+2,
						base+2 , base+3, base  ,
					]
				})
				.flat()
		);

		const stridedArrays = this._getStridedArrays(
			baseVtx + totalVertices
			// baseIdx + totalIndices
		);

		// Set up data for each of the symbols. This will trigger texturification
		// of each symbol's image.
		const promises = symbols.map((s, i) => {
			s.updateRefs(this, baseVtx + i * 4, baseIdx + i * 6);
			s._setGlobalStrides(...stridedArrays);

			this._knownSymbols[baseVtx + i * 4] = s;

			return s.buildTexture(this.glii, loadedRasters[i]);
		});

		if (this._crs) {
			this.reproject(baseVtx, totalVertices);
		}

		// The raster data might take time to be dumped into textures
		// (especifically for GeoTIFFs, since reading a GeoTIFF is async).
		// Therefore, re-dirty the acetate when the textures are ready.
		Promise.all(promises).then(() => (this.dirty = true));

		this._commitStridedArrays(baseVtx, totalVertices /*, baseIdx, totalIndices*/);

		this.dirty = true;

		super.multiAddIds(symbols, baseVtx);
		return super.multiAdd(symbols);
	}

	/**
	 * Redefinition of the default. Render must happen once per raster, in order
	 * to load the appropriate textures.
	 */
	runProgram() {
		this._knownSymbols.forEach((r, i) => {
			// console.log("Rendering conformal raster", i, r);
			this._programs.setTexture("uRasterTexture", r.texture);

			// "6" is constant here, but coudl also be fetched from `r.idxLength`.
			this._programs.runPartial(r.idxBase, r.idxLength);
		});
	}

	_getStridedArrays(_maxVtx, _maxIdx) {
		return [];
	}

	_commitStridedArrays(_baseVtx, _vtxLength /*, baseIdx, totalIndices*/) {
		// noop
	}

	_getGeometryStridedArrays() {
		return [];
	}

	_commitGeometryStridedArrays(_baseVtx, _vtxCount, _baseIdx, _idxCount) {
		// noop
	}
}


/**
 * @class ConformalRaster
 * @inherits GleoSymbol
 * @relationship drawnOn AcetateConformalRaster
 *
 * A rectangular, conformal (i.e. matching the display CRS) RGB(A) raster image.
 *
 * If the `Geometry` of the raster has a different CRS than the map, consider
 * using `ArrugatedRaster` instead.
 */
export default class ConformalRaster extends GleoSymbol {
	/// @section Static properties
	/// @property Acetate: Prototype of AcetateConformalRaster
	// The `Acetate` class that draws this symbol.
	static Acetate = AcetateConformalRaster;

	// static assertGeometry(geom) {
	// 	if (!(geom instanceof RawGeometry)) {
	// 		throw new Error("ConformalRaster Geometry is not a valid.");
	// 	}
	// 	if (geom.coords.length / geom.dimension !== 4) {
	// 		throw new Error("ConformalRaster Geometry must have exactly 4 coordinates.");
	// 	}
	// }

	/**
	 * @section
	 * A `ConformalRaster` is created from a `Geometry` and a RGB(A) image. The
	 * image can be either:
	 * - An instance of `HTMLImageElement`, which must be fully loaded
	 * - A `Promise` to such a `HTMLImageElement` instance
	 * - A `String` containing the URL for the image
	 * In the last two cases, the addition of the `ConformalRaster` to the
	 * platina or map will be delayed until the image has been loaded.
	 *
	 * The `Geometry` must have one ring of 4 elements. The image is assumed
	 * to be quadrangular (rectangular or quasi-rectangular), and each coordinate
	 * is mapped to the four "corners" of the image.
	 *
	 * The 4 points of the geometry **must** be sorted as follows:
	 * - Upper-Left corner of the raster
	 * - Upper-right idem
	 * - Lower-right idem
	 * - Lower-left idem
	 *
	 * @constructor ConformalRaster(coords: Geometry, raster: HTMLImageElement)
	 * Instantiate with an already-loaded image
	 * @alternative
	 * @constructor ConformalRaster(coords: Geometry, raster: Promise to HTMLImageElement)
	 * Instantiate with a `Promise` to a raster
	 * @alternative
	 * @constructor ConformalRaster(coords: Geometry, raster: String)
	 * Instantiate with the URL of a raster
	 */
	constructor(geom, raster, opts = {}) {
		super(geom, opts);
		// this.constructor.assertGeometry(this.geometry);

		/**
		 * @property raster: Promise to AbstractRaster
		 * A `Promise` to the raster image (when instantiated with a `Promise`
		 * or a `String` containing a URL)
		 * @alternative
		 * @property image: AbstractRaster
		 * The raster image itself (when instantiated with a fully-loaded
		 * `HTMLImageElement` or `GeoTIFF` or the like)
		 */
		this.raster = raster instanceof Promise ? raster : rasterFactory(raster);

		/**
		 * @option interpolate: Boolean = false
		 * Whether to use bilinear pixel interpolation or not.
		 *
		 * In other words: `false` means pixellated, `true` means smoother.
		 */
		this._interpolate = !!opts.interpolate;

		// A conformal raster is always represented as a quad: 4 vertices, 2 triangle
		// primitives of 3 slots each.
		this.attrLength = 4;
		this.idxLength = 6;
	}

	/**
	 * @method setGeometry(geom: Geometry): this
	 * Moves this conformal raster symbol to a new bounding box, as given by `geom`.
	 *
	 * The given geometry must have one ring of 4 elements.
	 */
	setGeometry(geom) {
		geom = factory(geom);
		assertGeometry(geom);

		this.geom = geom;
		if (this._inAcetate) {
			this._inAcetate.reproject(this.attrBase, this.attrLength);
			if (this._inAcetate._map) {
				this._inAcetate.dirty = true;
			}
		}
	}

	/**
	 * @method buildTexture(glii: Glii, raster: AbstractRaster): Promise of this
	 * Internal usage only, called from `AcetateConformalRaster` and only
	 * once the image promise has been resolved. Builds the texture given
	 * the Glii/WebGL context. Texture dumping might be async.
	 */
	async buildTexture(glii, raster) {
		/**
		 * @property texture: Texture
		 * The Glii texture containing this raster's RGB(A) image. Is initialized
		 * by an acetate, after this symbol has been added to it.
		 */

		this.raster = raster;
		this.texture = await raster.asTexture(glii);
		const texFilter = this._interpolate ? glii.LINEAR : glii.NEAREST;
		this.texture.setParameters(
			texFilter,
			texFilter,
			glii.CLAMP_TO_EDGE,
			glii.CLAMP_TO_EDGE
		);
		return this;
	}

	/**
	 * @method remove():this
	 * Removes this symbol from its containing `Acetate` (and, therefore, from the
	 * containing `GleoMap`). Cleans up resources dedicated to the GL texture.
	 */
	remove() {
		super.remove();
		this.texture.destroy && this.texture.destroy(); // So Glii cleans up GL resources
		delete this.texture;
	}

	_setGlobalStrides() {
		// noop
	}
	_setGeometryStrides() {
		// noop
	}
	_setPerPointStrides(_n, _pointType, _vtx, _vtxCount, _geom, ..._strides) {
		// noop
	}
}
