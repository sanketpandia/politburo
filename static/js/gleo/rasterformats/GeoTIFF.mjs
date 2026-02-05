import { known } from "./factory.mjs";
import AbstractRaster from "./AbstractRaster.mjs";

import {
	// Pool,
	// globals as geotiffGlobals,
	// fromBlob as tiffFromBlob,
	fromUrl as tiffFromUrl,
	// fromUrls as tiffFromUrls,
	GeoTIFFImage,
	// GeoTIFF as RawGeoTIFF,
} from "geotiff";

/**
 * @class GeoTIFF
 * An `AbstractRaster` that fits GeoTIFFs.
 *
 * When building from a GeoTIFF URL, this reads the *first* image of a GeoTIFF
 * file.
 */
export default class GeoTIFF extends AbstractRaster {
	#tiffImg;

	constructor(tiffImg) {
		super();
		this.#tiffImg = tiffImg;
	}

	static canWrap(obj) {
		return obj instanceof GeoTIFFImage;
	}

	static async fromUrl(url) {
		if (!url.match(/\.(geo)?tif?f/i)) {
			return Promise.reject();
		}
		const tiff = await tiffFromUrl(url);
		return new GeoTIFF(await tiff.getImage(0));
	}

	// Must dump the raster object into a new glii texture
	async asTexture(glii) {
		const data = await this.#tiffImg.readRasters({
			interleave: true,
		});

		let tex;
		const bands = this.bandCount;

		if (bands === 1 && data instanceof Float32Array) {
			tex = new glii.Texture({
				internalFormat: glii.gl.R32F,
				format: glii.gl.RED,
				type: glii.FLOAT,
			});
		} else if (bands === 4 && data instanceof Uint8Array) {
			tex = new glii.Texture({
				internalFormat: glii.RGBA,
				format: glii.RGBA,
				type: glii.UNSIGNED_BYTE,
			});
		} else {
			throw new Error(
				`Cannot create texture from a GeoTIFF with ${bands} bands/samples/channels, and datatype ${data.constructor.name}`
			);
		}

		return tex.texArray(this.width, this.height, data);
	}

	// Returns the width of the raster, in pixels.
	get width() {
		return this.#tiffImg.getWidth();
	}

	// Returns the height of the raster, in pixels.
	get height() {
		return this.#tiffImg.getHeight();
	}

	// Returns the number of channels/bands
	get bandCount() {
		return this.#tiffImg.getSamplesPerPixel();
	}

	// Returns the number of bits per channel/band
	get bitDepth() {
		return this.#tiffImg.getBytesPerPixel() * 8;
	}
}

known.push(GeoTIFF);
