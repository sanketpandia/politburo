/**
 * @class AbstractRaster
 *
 * Intended for internal use only.
 *
 * Abstract wrapper over image/raster files/datasets. Implements a common wrapper
 * over `HTMLImageElement` (native to the browser), GeoTIFFs (not native to the
 * browser).
 *
 * This class worries *only* about the raster data and how to fit it into a Glii
 * `Texture`. It does **not** worry abour the geographical component.
 */
export default class AbstractRaster {
	/// @function canWrap(obj: Object): Boolean
	/// Must return `true`if the object can be wrapped into this class
	static canWrap(obj) {
		return false;
	}

	/// @function fromUrl(url: String)
	/// Must return (a Promise to) an instance of this class. The promise might fail
	/// if the URL is not a raster of this type.
	static async fromUrl(url) {
		return false;
	}

	/// @method asTexture(glii: GliiFactory): Texture
	/// Dumps the raster object into a new glii texture
	asTexture(glii) {
		return false;
	}

	/// @property width
	/// The width of the raster, in pixels. Read-only.
	get width() {
		return 0;
	}

	/// @property height
	// Returns the height of the raster, in pixels.
	get height() {
		return 0;
	}

	/// @property bandCount
	// Returns the number of channels/bands
	get bandCount() {
		return 0;
	}

	/// @property bitDepth
	// Returns the number of bits per channel/band
	get bitDepth() {
		return 0;
	}
}
