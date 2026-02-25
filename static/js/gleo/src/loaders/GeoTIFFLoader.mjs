import Loader from "./Loader.mjs";
import BaseCRS from "../crs/BaseCRS.mjs";
import ArrugatedRaster from "../symbols/ArrugatedRaster.mjs";
import Geometry from "../geometry/Geometry.mjs";
import HeatMap from "../fields/HeatMap.mjs";
import { ScalarField } from "../fields/Field.mjs";

import { fromUrl as tiffFromUrl, GeoTIFF } from "geotiff";

// Ensure that ArrugatedRaster supports loading from GeoTIFF
import "../rasterformats/GeoTIFF.mjs";

/**
 * @class GeoTIFFLoader
 * @inherits Loader
 *
 * Loads a single GeoTIFF file (from a URL), automatically handling everything
 * needed for a default visualization of that GeoTIFF.
 *
 * In particular, this handles metadata for the CRS, the min/max values and number
 * of samples per pixel. It then provides a `ArrugatedRaster`, possibly paired
 * to a `HeatMap` to provide greyscale rendering.
 *
 * Note this works for single GeoTIFFs with *one* image inside. It will not
 * work for CloudOptimized GeoTIFFs (COGs) or the like.
 *
 * Any options not recognized by gleo will be passed on to the GeoTIFF.js library
 * when requesting a GeoTIFF from a URL.
 *
 * @example
 *
 * ```
 * new GeoTIFFLoader('file.tif').addTo(map);
 *
 * new GeoTIFFLoader('otherfile.tif', {
 * 	allowFullFile: true,	// geotiff.js-specific option
 * 	headers: {}, 	// geotiff.js-specific option
 * 	maxRanges: 0	// geotiff.js-specific option
 * }).addTo(map);*
 * ```
 */

export default class GeoTIFFLoader extends Loader {
	#url;
	#tiff;
	#arrugator;
	#wrapper; // ScalarField (etc) when needed for non-RGBA TIFFs

	#geotiffjsOptions = {};

	/**
	 * @constructor GeoTIFFLoader(url: String, opts?: GeoTIFFLoader Options)
	 * Loads the GeoTIFF from the given URL.
	 * @alternative
	 * @constructor GeoTIFFLoader(geotiff: GeoTIFF, opts?: GeoTIFFLoader Options)
	 * Loads the GeoTIFF from a `geotiff.js` instance
	 */
	constructor(
		url,
		{
			/**
			 * TODO:
			 * section GeoTIFFLoader Options
			 * option wireframeColour: Colour = undefined
			 * As the homonymous option in `AcetateArrugatedRaster`. Setting this
			 * to an actual colour will display the arrugator tesselation as wireframe.
			 */
			// wireframeColour = undefined,

			...opts
		} = {}
	) {
		super(opts);
		this.#url = url;
		this.#geotiffjsOptions = opts;
	}

	addTo(target) {
		// console.log("Adding GeoTIFFLoader to", target.constructor.name, target);

		super.addTo(target);
		const arrugator = this.#buildArrugatedRaster(this.#url);

		arrugator.then((arr) => {
			if (target instanceof ScalarField) {
				const acetate = new ArrugatedRaster.Acetate(target);
				arr.addTo(acetate);
			} else if (this.#wrapper) {
				const acetate = new ArrugatedRaster.Acetate(this.#wrapper);
				arr.addTo(acetate);
			} else {
				arr.addTo(target);
			}
		});
	}

	async #buildArrugatedRaster(url) {
		const tiff =
			url instanceof GeoTIFF ? url : await tiffFromUrl(url, this.#geotiffjsOptions);

		// console.log(`Geotiff has ${await tiff.getImageCount()} images`);

		this.#tiff = await tiff.getImage(0);

		const crsCode = "EPSG:" + this.#tiff.getGeoKeys().ProjectedCSTypeGeoKey;

		const crs = await BaseCRS.guessFromCode(crsCode);

		/// TODO: double-check this. Is there another way to fetch the corners?
		/// How are GeoTIFFs with slanted or rotated worldfiles work????
		const [minX, minY, maxX, maxY] = this.#tiff.getBoundingBox();

		this.#arrugator = new ArrugatedRaster(
			new Geometry(crs, [
				[minX, maxY],
				[minX, minY],
				[maxX, maxY],
				[maxX, minY],
			]),
			this.#tiff,
			{
				// ...opts
			}
		);

		const samples = this.#tiff.getSamplesPerPixel();

		// See geotiff.js/src/globals.js for explanation of values
		const interpretation = this.#tiff.getFileDirectory().PhotometricInterpretation;

		if (samples === 1) {
			const minValue = this.#tiff.getFileDirectory().SMinSampleValue;
			const maxValue = this.#tiff.getFileDirectory().SMaxSampleValue;
			// const nodata = this.#tiff.getGDALNoData();

			if (interpretation === 0) {
				// "White is zero"
				this.#wrapper = new HeatMap(this.platina, {
					stops: {
						[minValue - 0.001]: [0, 0, 0, 0],
						[minValue]: "white",
						[maxValue]: "black",
						[maxValue + 0.001]: [0, 0, 0, 0],
					},
				});
			} else if (interpretation === 1) {
				// "Black is zero"
				this.#wrapper = new HeatMap(this.platina, {
					stops: {
						[minValue - 0.001]: [0, 0, 0, 0],
						[minValue]: "black",
						[maxValue]: "white",
						[maxValue + 0.001]: [0, 0, 0, 0],
					},
				});
			} else if (interpretation === 3) {
				throw new Error("Cannot (yet) display pallette GeoTIFFs");
			} else {
				throw new Error(
					`Unknown/invalid photogrammetric interpretation for 1-band raster: ${interpretation}`
				);
			}

			// Explicitly create an ArrugatedRasterAcetate and add it to the heatmap
			// const acetate = new ArrugatedRaster.Acetate(this.#wrapper);
			// this.#arrugator.addTo(acetate);
		} else if (samples === 3 || samples === 4) {
			// no-op, assume RGB.
			// this.#arrugator.addTo(this.target);
			// this.#wrapper = this.#arrugator;
		} else {
			throw new Error(
				`Cannot (yet) display GeoTIFF with ${samples} samples per pixel`
			);
		}

		return this.#arrugator;
	}
}
