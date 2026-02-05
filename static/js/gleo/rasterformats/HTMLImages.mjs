// import { known } from './factory.mjs';
import AbstractRaster from "./AbstractRaster.mjs";

/// TODO: Detect if we're running in a browser. If not, export a dummy raster format
/// that never fits.

/**
 * @class HTMLImages
 * An `AbstractRaster` format that fits images natively understood by the
 * browser, including PNG, JPG, WebP, etc.
 *
 */
export default class HTMLImages extends AbstractRaster {
	#img;

	constructor(img) {
		super();
		this.#img = img;
	}

	static canWrap(obj) {
		return (
			obj instanceof HTMLImageElement ||
			obj instanceof HTMLCanvasElement ||
			obj instanceof ImageData ||
			obj instanceof ImageData
		);
	}

	static async fromUrl(url) {
		// Explicitly fail GeoTIFFs without trying to load them
		if (url.match(/\.(geo)?tif?f/i)) {
			return Promise.reject();
		}

		return await new Promise((res, rej) => {
			const img = new Image();
			img.addEventListener("load", (ev) => res(new HTMLImages(ev.target)));
			img.addEventListener("error", rej);
			img.addEventListener("abort", rej);
			img.crossOrigin = true;
			img.src = url;
		});
	}

	// Must dump the raster object into a new glii texture
	asTexture(glii) {
		return new glii.Texture({}).texImage2D(this.#img);
	}

	// Returns the width of the raster, in pixels.
	get width() {
		return this.#img.naturalWidth;
	}

	// Returns the height of the raster, in pixels.
	get height() {
		return this.#img.naturalHeight;
	}

	// Returns the number of channels/bands
	get bandCount() {
		return 4; // RGBA
	}

	// Returns the number of bits per channel/band
	get bitDepth() {
		return 8;
	}
}

// known.push(HTMLImages);
