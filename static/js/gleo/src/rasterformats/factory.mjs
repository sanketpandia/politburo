import AbstractRaster from "./AbstractRaster.mjs";

// Load browser-capable formats by default
import HTMLImages from "./HTMLImages.mjs";

// Singleton store of known raster formats
export let known = [HTMLImages];

// if (window) {
// 	import("./HTMLImages.mjs");
// }

function rasterFromWrappable(obj) {
	const fit = known.find((format) => format.canWrap(obj));
	return fit && new fit(obj);
}

export default function factory(r) {
	if (r instanceof AbstractRaster) {
		return r;
	}
	const wrapped = rasterFromWrappable(r);
	if (wrapped) {
		return wrapped;
	} else if (typeof r === "string") {
		r = new URL(r, document.URL);
	} else if (!(r instanceof URL)) {
		throw new Error(
			"Bad parameter to raster factory: must be a URL or a raster/image object"
		);
	}

	const urlStr = r.toString();

	return Promise.any(known.map((format) => format.fromUrl(urlStr)));
}
