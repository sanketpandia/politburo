// import Stroke from "../symbols/Stroke.mjs";
// import Hair from "../symbols/Hair.mjs";
import ProtobufVectorTileLoader from "./ProtobufVectorTileLoader.mjs";
import Loader from "./Loader.mjs";
import create3857Pyramid from "../geometry/Pyramid3857.mjs";
// import parseColour from "../3rd-party/css-colour-parser.mjs";
import MercatorTiles from "./MercatorTiles.mjs";
import RasterTileLoader from "./RasterTileLoader.mjs";
import abortableImagePromise from "../util/abortableImagePromise.mjs";
import template from "../util/templateStr.mjs";
import tileJsonToPyramid from "../geometry/TileJsonPyramid.mjs";
import TileEvent from "../dom/TileEvent.mjs";

import fillSymbolizer from "./vectorstylesheet/fillSymbolizer.mjs";
import lineSymbolizer from "./vectorstylesheet/lineSymbolizer.mjs";
import getFilterFunc from "./vectorstylesheet/filter.mjs";

/**
 * @class VectorStylesheetLoader
 * @inherits Loader
 * @relationship aggregationOf RasterTileLoader, 0..1, 0..n
 * @relationship aggregationOf ProtobufVectorTileLoader, 0..1, 0..n
 *
 * Should read a JSON document containing a Mapbox GL JS Stylesheet, and
 * spawn a ProtobufVectorTileLoader (plus a RasterTileLoader if there is
 * aerial imagery specified in the stylesheet)
 *
 * See https://docs.mapbox.com/mapbox-gl-js/style-spec/
 *
 */

export default class VectorStylesheetLoader extends Loader {
	// An array containing one loader per data source.
	// These can be `RasterTileLoader`s, `ProtobufVectorTileLoader`s
	#subloaders = [];

	#backgroundColour;

	#boundDispatchEvent;

	/**
	 * @section
	 * A `VectorStylesheetLoader` takes the URL of the JSON stylesheet as its
	 * only constructor parameter.
	 *
	 * @constructor VectorStylesheetLoader(url: URL)
	 * @alternative
	 * @constructor VectorStylesheetLoader(url: String)
	 */
	constructor(
		url,
		{
			/**
			 * @option interactive: Boolean = false
			 * Whether the `GleoSymbol`s spawned bythis loader shall be
			 * `interactive` themselves (or not).
			 */
			interactive = false,
			...opts
		} = {}
	) {
		super();

		this.#boundDispatchEvent = function proxyEvent(ev) {
			this.dispatchEvent(
				new TileEvent(ev.type, {
					tileLevel: ev.tileLevel,
					tileX: ev.tileX,
					tileY: ev.tileY,
					tile: ev.tile,
					error: ev.error,
				})
			);
		}.bind(this);

		url = new URL(url, document.url);
		fetch(url)
			.then((res) => res.json())
			.then((stylesheet) => {
				const themeFuncs = {};

				// Each "theme" (or "layer" in mapbox/maplibre parlance) shall
				// spawn a lambda-function to return `GleoSymbol`s from a
				// feature from that theme.
				stylesheet.layers.forEach((theme) => {
					let themeFunc;
					if (theme.type == "fill" && theme.paint["fill-color"]) {
						/// TODO: Handle fill patterns

						try {
							themeFunc = fillSymbolizer(theme, interactive);
						} catch (ex) {
							console.info(ex, theme);
						}
					} else if (theme.type == "line" && theme.paint["line-color"]) {
						/// TODO: handle line patterns

						try {
							themeFunc = lineSymbolizer(theme, interactive);
						} catch (ex) {
							console.info(ex, theme);
						}
					} else if (theme.type === "background") {
						if (typeof theme.paint["background-color"] === "string") {
							this.#backgroundColour = theme.paint["background-color"];
						} else if (theme.paint["background-color"].stops) {
							this.#backgroundColour =
								theme.paint["background-color"].stops[0][1];
						}
					}

					if (themeFunc && theme.filter) {
						const booleanFilter = getFilterFunc(theme.filter, themeFunc);
						themeFunc = (function filterClosure(fn) {
							return function filter(geom, attrs) {
								return booleanFilter(geom, attrs) ? fn(geom, attrs) : [];
							};
						})(themeFunc);
					}

					if (themeFunc) {
						const source = theme["source-layer"];
						if (themeFuncs[source]) {
							themeFuncs[source].push(themeFunc);
						} else {
							themeFuncs[source] = [themeFunc];
						}
					}
				});

				// Flatten themeFuncs
				Object.entries(themeFuncs).forEach(([themeName, funcs]) => {
					themeFuncs[themeName] = function flattenSymbols(geom, attrs) {
						return funcs.map((f) => f(geom, attrs)).flat();
					};
				});

				function stylesheetSymbolizer(themeName, geom, attrs) {
					if (attrs.class === "street" && attrs.type === "residential") {
						console.log(attrs);
					}
					if (themeFuncs[themeName]) {
						return themeFuncs[themeName](geom, attrs);
					} else {
						return [];
					}
				}

				Object.entries(stylesheet.sources).forEach(
					async ([sourceName, source]) => {
						//console.log(sourceName, source);

						let loader;

						if (source.attribution && !source.url && !source.tiles) {
							// Attribution-only source - set the loader's attribution
							// to it, and hope that there's only one such source.
							this.attribution = source.attribution;
						} else if (source.type === "raster" && source.tiles) {
							const tilesTemplate = source.tiles[0];

							loader = new MercatorTiles(tilesTemplate, {
								minZoom: source.tiles.minZoom || 0,
								maxZoom: source.tiles.maxZoom || 15,
								tileSize: source.tiles.tileSize || 256,
								attribution: source.attribution || opts.attribution,
								...opts,
							});
						} else if (source.type === "raster" && source.url) {
							const { pyramid, tiles } = await tileJsonToPyramid(
								source.url,
								source.tileSize
							);

							const templateStr = tiles[0];

							loader = new RasterTileLoader(
								pyramid,
								function fetchImage(z, x, y, controller) {
									return abortableImagePromise(
										template(templateStr, { x, y, z }),
										controller
									);
								},
								{
									tileResX: source.tileSize || 256,
									tileResY: source.tileSize || 256,
									//tileSize: source.tileSize || 256,
									attribution: source.attribution || opts.attribution,
									...opts,
								}
							);
						} else if (source.type === "vector" && source.tiles) {
							const pyramid = create3857Pyramid(
								source.tiles.minZoom || 0,
								source.tiles.maxZoom || 14,
								source.tiles.tileSize || 256
							);

							loader = new ProtobufVectorTileLoader(
								pyramid,
								source.tiles[0],
								stylesheetSymbolizer,
								{
									attribution: source.attribution || opts.attribution,
									...opts,
								}
							);
						} else if (source.type === "vector" && source.url) {
							const { pyramid, tiles } = await tileJsonToPyramid(
								source.url
							);

							loader = new ProtobufVectorTileLoader(
								pyramid,
								tiles[0],
								stylesheetSymbolizer,
								{
									attribution: source.attribution || opts.attribution,
									...opts,
								}
							);
						}

						if (loader) {
							this.#subloaders.push(loader);
							if (this.platina) {
								this.#addSubloader(loader);
							}
						}
						if (this.#backgroundColour) {
							this.platina.backgroundColour = this.#backgroundColour;
						}
					}
				);
			})
			.catch((err) => {
				throw err;
			});
	}

	addTo(target) {
		super.addTo(target);
		this.#subloaders.forEach(this.#addSubloader.bind(this));
		if (this.#backgroundColour) {
			this.platina.backgroundColour = this.#backgroundColour;
		}
		return this;
	}

	remove() {
		super.remove();
		this.#subloaders.forEach((subloader) => {
			for (let evName of ["tileload", "tileerror", "tileprune"]) {
				subloader.removeEventListener(evName, this.#boundDispatchEvent);
			}
			subloader.remove();
		});
		/// TODO: There are no capabilities to reset the background colour
	}

	#addSubloader(subloader) {
		subloader.addTo(this.platina);
		for (let evName of ["tileload", "tileerror", "tileprune", "tileout"]) {
			subloader.addEventListener(evName, this.#boundDispatchEvent);
		}
	}
}
