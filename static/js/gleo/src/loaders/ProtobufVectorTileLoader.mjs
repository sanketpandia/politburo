import VectorTile from "../3rd-party/vector-tile/vectortile.mjs";
import Protobuf from "../3rd-party/pbf/pbf.mjs";

import GenericVectorTileLoader from "./GenericVectorTileLoader.mjs";
import template from "../util/templateStr.mjs";
import loadProtobufferRawGeometry from "../util/loadProtobufferRawGeometry.mjs";

/**
 * @class ProtobufVectorTileLoader
 * @inherits GenericVectorTileLoader
 *
 * Loader for protobuffer (`.pbf`) vector tiles. Also known as "MVT"
 * (mapbox/maplibre vector tiles).
 *
 * Requires:
 * * A `TilePyramid`
 * * A tile URL template (as `RasterTileLoader`), and
 * * A function that takes a vector feature (theme/"layer", geometry and attributes)
 * and returns an array of `GleoSymbol`s.
 *
 * @example
 *
 * ```
 * let vectorTiles = new ProtobufVectorTileLoader(
 * 	pyramid,
 * 	"https://api.maptiler.com/tiles/v3-openmaptiles/{z}/{x}/{y}.pbf?key=API_KEY_GOES_HERE",
 * 	function (themeName, geom, attrs) {
 * 		if (themeName === "water") {
 * 			return [ new Fill(geom, {
 * 				colour: [0, 0, 128, 128],
 * 				interactive: true,
 * 			})];
 * 		} else {
 * 			return [];
 * 		}
 * 	}, {
 * 		attribution: "MapTiler, OpenStreetMap"
 * 	}
 * ).addTo(gleoMap);
 * ```
 *
 */

/// TODO: Allow for arbitrary options in template string, leaflet-style????

export default class ProtobufVectorTileLoader extends GenericVectorTileLoader {
	#symbolizer;
	#templateStr;

	#onEachFeature;
	#fetchOptionsForTile;

	/**
	 * @constructor ProtobufVectorTileLoader(pyramid: TilePyramid, templateStr: String, symbolyzer: Function, opts?: ProtobufVectorTileLoader options)
	 */
	constructor(
		pyramid,
		templateStr,
		symbolizer,
		{
			/**
			 * @option onEachFeature: Function
			 * Callback function that will be called just after each feature has
			 * been symbolized.
			 * The callback function will receive `(symbols, geometry, themeName, attributes)`
			 * as parameters. The callback will not be called if a vector tile
			 * feature was filtered out or otherwise was symbolized to zero symbols.
			 */
			onEachFeature = undefined,

			/**
			 * @option fetchOptionsForTile: Function = undefined
			 * Optional callback function for supplying custom fetch options, given
			 * the tile coordinates (`level`, `x` and `y`).
			 *
			 * The return value of this callback must be a set of fetch options,
			 * as per the `options` parameter in https://developer.mozilla.org/en-US/docs/Web/API/fetch .
			 *
			 */
			fetchOptionsForTile = undefined,
			...opts
		} = {}
	) {
		// super(pyramid, this.#tileFn.bind(this), opts);
		super(pyramid, undefined, opts);
		this._tileFn = this.#tileFn;
		this.#symbolizer = symbolizer;
		this.#templateStr = templateStr;

		this.#onEachFeature = onEachFeature;
		this.#fetchOptionsForTile = fetchOptionsForTile;
	}

	#tileFn(z, x, y, controller) {
		const headers =
			this.#fetchOptionsForTile === undefined
				? {}
				: this.#fetchOptionsForTile(z, x, y);

		return fetch(template(this.#templateStr, { x, y, z }), {
			...headers,
			signal: controller.signal,
		}).then(async (res) => {
			const tile = new VectorTile(new Protobuf(await res.arrayBuffer()));
			const bbox = this.pyramid.tileCoordsToBbox(z, [x, y]);

			const numericZoom = Number(z);
			// A tile has themes (landuse/roads/built-up/etc), which
			// VectorTile calls "layers".

			const symbols = Object.entries(tile.layers)
				.map(([themeName, theme]) => {
					const extent = theme.extent;
					const themeSymbols = [];

					for (let i = 0; i < theme.length; i++) {
						const feat = theme.feature(i);
						// const coords = feat.loadGeometry();
						// const geom = this.#normalizeGeom(extent, bbox, coords, feat.type);
						const geom = loadProtobufferRawGeometry(
							this.pyramid.crs,
							feat,
							bbox,
							extent
						);

						const geomType =
							feat.type === 1
								? "Point"
								: feat.type === 2
								? "LineString"
								: "Polygon";

						const symbols = this.#symbolizer(themeName, geom, {
							$type: geomType,
							"geometry-type": geomType,
							$zoom: numericZoom,
							...feat.properties,
						});

						this.#onEachFeature?.(symbols, geom, themeName, feat.properties);

						themeSymbols[i] = symbols;
					}
					return themeSymbols.flat();
				})
				.flat();

			return symbols;
		});
	}
}
