import Loader from "./Loader.mjs";
import abortableImagePromise from "../util/abortableImagePromise.mjs";
import Geometry from "../geometry/Geometry.mjs";
import ConformalRaster from "../symbols/ConformalRaster.mjs";

const XMLparser = new window.DOMParser();
const parseXML = function (str) {
	return XMLparser.parseFromString(str, "text/xml");
};

/**
 * @class ConformalWMS
 * @inherits Loader
 * @relationship compositionOf ConformalRaster, 0..1, 1..1
 *
 * A `Loader` that displays conformal rasters from a WMS service.
 *
 * The constructor needs the base URL of the WMS. The `ConformalWMS` loader
 * will perform a `getCapabilities` query. The raster will only by displayed if
 * the CRS of the map is one of the CRSs available from the WMS.
 *
 * This is an **untiled** WMS client implementation: every viewport change
 * translates into a new image request to the WMS.
 *
 */
export default class ConformalWMS extends Loader {
	/**
	 * @constructor ConformalWMS(url: String, opts: ConformalWMS Options)
	 *
	 * Instantiates a `ConformalWMS`, given the base URL for the WMS, and a
	 * string containing the name(s) of one or more thematic "layer"(s). These
	 * "layer" names must be present in the WMS's capabilities, and must be
	 * available in the CRS of the containing map.
	 *
	 */
	constructor(url, { wmsVersion = "1.3.0", ...opts } = {}) {
		/**
		 * @section
		 * @aka ConformalWMS Options
		 * @option layer: String
		 * The name of the thematic layer to request, as defined per the WMS capabilities
		 * @alternative
		 * @option layer: Array of String
		 * The name**s** of the thematic layer**s** to request.
		 * @option style: String
		 * A style for the thematic layer, as defined per the WMS capabilities
		 * @alternative
		 * @option style: Array of String
		 * The style**s** for the thematic layer**s**. There must be a one-to-one equivalence.
		 * @option imageFormat: String
		 * The MIME type for the image format to request, e.g. `image/png` or
		 * `image/png`.
		 * If ommitted, the first format listed in the WMS capabilities will be used.
		 * @option transparent: Boolean
		 * Whether or not to request transparent images.
		 * @option debounceTime: Number = 500
		 * Time, in milliseconds, to debounce the WMS requests. This is a
		 * preventative measure against overloading the WMS server.
		 * @option wmsVersion: String = "1.3.0"
		 * The version of the WMS protocol to use
		 * @option interpolate: Boolean = false
		 * Akin to the `interpolate` option of `ConformalRaster`
		 */

		super(opts);

		this._wmsVersion = wmsVersion;

		this._boundOnViewChange = this._onViewChange.bind(this);
		this._boundReloadImage = this._reloadImage.bind(this);

		this._capsUrl = new URL(url, document.URL);
		this._capsUrl.searchParams.set("service", "WMS");
		this._capsUrl.searchParams.set("version", wmsVersion);
		this._capsUrl.searchParams.set("request", "getCapabilities");
		this._capabilities = fetch(this._capsUrl)
			.then((response) => response.text())
			.then(parseXML)
			.then((capabilities) => {
				return capabilities.documentElement;
			});

		this._getMapCaps = this._capabilities.then((caps) => {
			const getMapCaps = caps.querySelector("Capability Request GetMap");
			const formats = Array.from(getMapCaps.querySelectorAll("Format")).map(
				(f) => f.textContent
			);
			const url = getMapCaps
				.querySelector("HTTP > Get > OnlineResource")
				.getAttribute("xlink:href");

			return {
				formats,
				url: url ? new URL(url, document.URL) : this._capsUrl,
			};
		});

		this._themeCaps = this._capabilities.then((caps) => {
			const themeCaps = {};

			caps.querySelectorAll("Capability Layer > Name").forEach((nameNode) => {
				const name = nameNode.textContent;
				const CRSs = [];
				const styles = [];
				let attributionURL;
				let attributionText;

				// In WMS 1.3.0, <Layer> elements can be nested
				let node = nameNode.parentNode;
				while (node.nodeName === "Layer") {
					for (let i = 0, l = node.children.length; i < l; i++) {
						let child = node.children.item(i);
						switch (child.nodeName) {
							case "CRS":
								CRSs.push(child.textContent);
								break;
							// case "BoundingBox": // TODO
							case "Style":
								styles.push(child.querySelector("Name").textContent);
								break;
							case "Attribution":
								if (!attributionURL) {
									attributionURL = child
										.querySelector("OnlineResource")
										.getAttribute("xlink:href");
									attributionText =
										child.querySelector("Title").textContent;
								}
						}
					}
					node = node.parentNode;
				}
				themeCaps[name] = {
					CRSs,
					styles,
					attributionText,
					attributionURL,
					// bboxes: /* optional, I guess? One per CRS. */,
					// legend: /* optional */,
				};
			});
			return themeCaps;
		});

		// this._themeNames.then(ns=>console.log("Available theme names", ns))

		this._getMapCaps.then((caps) => console.log("GetMap capabilities:", caps));
		this._themeCaps.then((caps) => console.log("Theme capabilities:", caps));

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

		/// TODO: Clamp viewport bbox by thematic layer bbox
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

		let [getMapCaps, themeCaps] = await Promise.all([
			this._getMapCaps,
			this._themeCaps,
		]);

		if (!(this.options.layer instanceof Array)) {
			this.options.layer = [this.options.layer];
		}
		if (!(this.options.style instanceof Array)) {
			this.options.style = [this.options.style];
		}

		this.options.layer.forEach((l) => {
			if (!themeCaps[l]) {
				throw new Error(`WMS does not offer thematic layer ${l}`);
			}
		});

		if (!this.options.layer.every((l) => themeCaps[l].CRSs.includes(crs.name))) {
			throw new Error(
				`WMS does not implement CRS ${crs.name} (in the requested layer(s))`
			);
		}

		const [baseMinX, baseMinY] = crs.offsetToBase([minX, minY]);
		const [baseMaxX, baseMaxY] = crs.offsetToBase([maxX, maxY]);
		// 		const [baseMinX, baseMinY] = crs.offsetFromBase([minX, minY]);
		// 		const [baseMaxX, baseMaxY] = crs.offsetFromBase([maxX, maxY]);
		const [w, h] = this.platina.pxSize;
		const url = getMapCaps.url;
		const params = url.searchParams;

		if (this._wmsVersion === "1.3.0" && crs.flipAxes) {
			params.set("bbox", `${baseMinY},${baseMinX},${baseMaxY},${baseMaxX}`);
		} else {
			params.set("bbox", `${baseMinX},${baseMinY},${baseMaxX},${baseMaxY}`);
		}

		params.set("request", "GetMap");
		params.set("version", this._wmsVersion);
		params.set("crs", crs.name);
		params.set("width", w);
		params.set("height", h);
		params.set("layers", this.options.layer.join(","));
		params.set("styles", this.options.style.join(","));
		params.set("format", this.options.imageFormat || getMapCaps.formats[0]);
		params.set("transparent", !!this.options.transparent);

		this._abortController = new AbortController();
		this._image = new abortableImagePromise(url.toString(), this._abortController);

		this._image.then((img) => {
			if (!this._conformalRaster) {
				this._conformalRaster = new ConformalRaster(geom, img, {
					interpolate: this.options.interpolate,
				}).addTo(this.platina);
			} else {
				this._conformalRaster.setGeometry(geom);
				this._conformalRaster.texture.texImage2D(img);
			}
		});
	}
}
