import Loader from "./Loader.mjs";
import abortableImagePromise from "../util/abortableImagePromise.mjs";
import Geometry from "../geometry/Geometry.mjs";
import ConformalRaster from "../symbols/ConformalRaster.mjs";

/**
 * @class ConformalOGCAPIMaps
 * @inherits Loader
 * @relationship compositionOf ConformalRaster
 *
 * A `Loader` that displays conformal rasters from a OGC API Maps service, as
 * described in:
 * - https://docs.ogc.org/DRAFTS/20-058.html
 * - https://opengeospatial.github.io/architecture-dwg/api-maps/index.html
 *
 * The constructor needs the base URL of the OGC API endpoint. The `ConformalOGCAPIMaps`
 * loader will request the metadata for that endpoint (available collections,
 * formats, CRSs, etc).
 *
 * This is an **untiled** client implementation: every viewport change
 * translates into a new image request to the API.
 *
 */
export default class ConformalOGCAPIMaps extends Loader {
	#baseURL; // URL of the API endpoint
	#collection; // Promise to the metadata of the collection in use
	#imageURL; // Promise to the base URL of the image
	#attribution; // String of HTML attribution from the metadata

	/**
	 * @constructor ConformalOGCAPIMaps(url: String, opts: ConformalOGCAPIMaps Options)
	 *
	 * Instantiates a `ConformalOGCAPIMaps`, given the base URL for the API endpoint,
	 * and a string containing the ID of a "collection".
	 *
	 */
	constructor(url, { collection, ...opts } = {}) {
		/**
		 * @section
		 * @aka ConformalOGCAPIMaps Options
		 * @option collection: String
		 * The ID of the collection to use. Note this is que unique `id`, and **not**
		 * the human-readable `title` of the collection.
		 * @option imageFormat: String
		 * The MIME type for the image format to request, e.g. `image/png` or
		 * `image/jpeg`.
		 * If ommitted, the first format listed in the collection metadata will be used.
		 * @option debounceTime: Number = 500
		 * Time, in milliseconds, to debounce the HTTP(S) requests. This is a
		 * preventative measure against overloading the OGC API server.
		 * @option transparent: Boolean
		 * Whether or not to request transparent images.
		 * @option interpolate: Boolean = false
		 * Akin to the `interpolate` option of `ConformalRaster`
		 */

		super(opts);

		this._boundOnViewChange = this._onViewChange.bind(this);
		this._boundReloadImage = this._reloadImage.bind(this);

		// Strip the trailing slash, if there's one.
		this.#baseURL = url.replace(/\/?$/, "");

		// First, fetch the metadata for the collection
		// The collection metadata is common for all OGC APIs related to that
		// collection - e.g. raster maps, raster tiles, raw vector features,
		// are all listed in the collection's metadata.
		const collectionsURL = new URL(this.#baseURL + "/collections", document.URL);
		collectionsURL.searchParams.set("f", "json");
		this.#collection = fetch(collectionsURL)
			.then((response) => response.json())
			.then((json) => {
				console.log(json.collections);
				const matches = json.collections.filter((c) => c.id === collection);
				if (matches.length === 0) {
					throw new Error(
						`Collection '${collection}' is not available from OGC API endpoint ${
							this.#baseURL
						}`
					);
				} else if (matches.length > 1) {
					throw new Error(
						`Collection '${collection}' has a duplicate definition from OGC API endpoint ${
							this.#baseURL
						}`
					);
				}
				this.#attribution = matches[0].attribution;
				return matches[0];
			});

		/// TODO: Fetch the attribution HTML string, store it properly.

		// Then, choose an appropriate image endpoint for the collection,
		// based on the image format.
		this.#imageURL = this.#collection
			.then((col) => {
				// Filter links: only interested in the ones conforming to the
				// OGC API Maps spec.
				const links = col.links.filter(
					(l) => l.rel === "http://www.opengis.net/def/rel/ogc/1.0/map"
				);
				if (!opts.imageFormat) {
					// On no specified image format (png/jpeg), use the first one from
					// the collection metadata
					return links[0];
				} else {
					const formatLinks = links.filter((l) => l.type === opts.imageFormat);
					if (formatLinks.length === 0) {
						throw new Error(
							`OGC API: Maps for collection ${collection} are not available in image format ${opts.imageFormat}`
						);
					} else if (formatLinks.length > 1) {
						throw new Error(
							`OGC API: Maps for collection ${collection} have multiple endpoints for image format ${opts.imageFormat}`
						);
					}
					return formatLinks[0];
				}
			})
			.then((link) => {
				return new URL(link.href, this.#baseURL);
			});

		this.#imageURL.catch((ex) => {
			throw new Error(`Could not get details from OGC API because: ${ex}`);
		});

		this.options = opts;
	}

	addTo(map) {
		super.addTo(map);

		this.platina.on("viewchanged", this._boundOnViewChange);
		return this;
	}

	remove() {
		this.platina.off("viewchanged", this._boundOnViewChange);

		if (this._conformalRaster) {
			this._conformalRaster.remove();
		}
		super.remove();
		return this;
	}

	_onViewChange(ev) {
		if (this._abortController && !this._abortController.signal.aborted) {
			this._abortController.abort();
		}
		clearTimeout(this._timeout);
		this._timeout = setTimeout(
			this._boundReloadImage,
			this.options.debounceTime || 250
		);
	}

	async _reloadImage() {
		const mapBBox = this.platina.bbox;

		const crs = this.platina.center.crs;

		/// TODO: Clamp viewport bbox by collection bbox
		/// Is there even a collection bbox???
		const { minX, maxX, minY, maxY } = mapBBox;
		const geom = new Geometry(
			crs,
			[
				[minX, maxY],
				[maxX, maxY],
				[maxX, minY],
				[minX, minY],
			],
			{ wrap: false }
		);

		let url = await this.#imageURL;

		const [baseMinX, baseMinY] = crs.offsetToBase([minX, minY]);
		const [baseMaxX, baseMaxY] = crs.offsetToBase([maxX, maxY]);
		const [w, h] = this.platina.pxSize;

		const params = url.searchParams;

		/// TODO: The short CRS names ("EPSG:4326") migth not be supported
		/// by the server, which might only rely on the links
		/// ("http://www.opengis.net/def/crs/EPSG/0/4326") to the GML
		/// definitions in the collection metadata.
		params.set("crs", crs.name);
		params.set("bbox-crs", crs.name);

		if (crs.flipAxes) {
			params.set("bbox", `${baseMinY},${baseMinX},${baseMaxY},${baseMaxX}`);
		} else {
			params.set("bbox", `${baseMinX},${baseMinY},${baseMaxX},${baseMaxY}`);
		}

		params.set("width", w);
		params.set("height", h);
		// 		params.set("styles", this.options.style.join(","));
		params.set("transparent", !!this.options.transparent);

		//console.log(url.toString());

		this._abortController = new AbortController();
		this._image = abortableImagePromise(url.toString(), this._abortController);

		this._image.then((img) => {
			if (!this._conformalRaster) {
				this._conformalRaster = new ConformalRaster(geom, img, {
					interpolate: this.options.interpolate,
					attribution: this.#attribution,
				}).addTo(this.platina);
			} else {
				this._conformalRaster.setGeometry(geom);
				this._conformalRaster.texture.texImage2D(img);
			}
		});
	}
}
