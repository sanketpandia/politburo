import { PMTiles } from "pmtiles";

import VectorTile from "../3rd-party/vector-tile/vectortile.mjs";
import Protobuf from "../3rd-party/pbf/pbf.mjs";

import GenericVectorTileLoader from "./GenericVectorTileLoader.mjs";
import Loader from "./Loader.mjs";
// import template from "../util/templateStr.mjs";
import loadProtobufferRawGeometry from "../util/loadProtobufferRawGeometry.mjs";

import create3857Pyramid from "../geometry/Pyramid3857.mjs";

/**
 * @class ProtoMapsLoader
 * @inherits Loader
 *
 * Loader for ProtoMaps vector tiles. See https://protomaps.com/ .
 *
 * It assumes a EPSG:3857 pyramid.
 */

export default class ProtoMapsLoader extends Loader {
	#symbolizers = {};

	#defaultSymbolizer;

	#onEachFeature;
	// #fetchOptionsForTile;

	#pmtiles; // The PMTiles instance

	// A GenericVectorTileLoader, init'd once the pmtiles metadata is loaded.
	#tileLoader;

	/**
	 * @constructor ProtobufVectorTileLoader(pyramid: TilePyramid, templateStr: String, symbolyzer: Function, opts?: ProtobufVectorTileLoader options)
	 */
	constructor(
		source,
		{
			/**
			 * @option symbolizers: Obect of String to Function
			 *
			 * A key-value map of symbolizer callback functions. The keys
			 * must be the names of the themes in the tileset (e.g. `"roads"` or
			 * `"natural"`), the values must be symbolizer functions that must
			 * return an array of zero or more `GleoSymbol`s.
			 */
			symbolizers = {},

			/**
			 * @option defaultSymbolizer: Function
			 *
			 * A callback symbolizer function used whenever there is no matching entry in
			 * `symbolizers`. It must return an array of zero or more `GleoSymbol`s.
			 */
			defaultSymbolizer = undefined,

			/**
			 * @option tileSize: Number = 256
			 * The expected size of the tiles, in CSS pixels
			 */
			tileSize = 256,

			...opts
		} = {}
	) {
		// super(pyramid, this.#tileFn.bind(this), opts);

		super();

		this.#pmtiles = new PMTiles(source);
		this.#symbolizers = symbolizers;
		this.#defaultSymbolizer = defaultSymbolizer ?? function () {};

		this.#tileLoader = this.#pmtiles.getMetadata().then((metadata) => {
			// Get min/max zoom level from vector_layers metadata
			let minZoom = Infinity;
			let maxZoom = -Infinity;

			console.log(metadata);

			metadata.vector_layers.forEach((l) => {
				minZoom = Math.min(minZoom, l.minzoom);
				maxZoom = Math.max(maxZoom, l.maxzoom);
			});

			const pyramid = create3857Pyramid(minZoom, maxZoom, tileSize);

			return new GenericVectorTileLoader(pyramid, this.#tileFn.bind(this));
		});

		this.#tileLoader.then(console.log);

		// super(pyramid, undefined, opts);
		// this._tileFn = this.#tileFn;
		// this.#symbolizer = symbolizer;
		// this.#templateStr = templateStr;
		//
		// this.#onEachFeature = onEachFeature;
		// this.#fetchOptionsForTile = fetchOptionsForTile;
	}

	/**
	 * @property tileLoader: Promise to AbstractTileLoader
	 * Resolves to the underlying tile loader, once the metadata for the
	 * PMTiles source has been loaded.
	 */
	get tileLoader() {
		return this.#tileLoader;
	}

	addTo(target) {
		/// FIXME: cover edge case of adding then immediately removing a ProtoMapsLoader
		this.#tileLoader.then((tileloader) => tileloader.addTo(target));
		super.addTo(target);
	}

	remove() {
		this.#tileLoader.then((tileloader) => tileloader.remove());
		super.remove();
	}

	async #tileFn(z, x, y, controller) {
		const pmtile = await this.#pmtiles.getZxy(z, x, y, controller.signal);

		// Parse the mapbox-style protobuffer vector tile
		const tile = new VectorTile(new Protobuf(pmtile.data));

		const pyramid = (await this.#tileLoader).pyramid;
		const bbox = pyramid.tileCoordsToBbox(z, [x, y]);

		const symbols = Object.entries(tile.layers)
			.map(([themeName, theme]) => {
				const extent = theme.extent;
				const themeSymbols = [];
				const symbolizer =
					this.#symbolizers[themeName] ?? this.#defaultSymbolizer;
				const numericZoom = +z;

				// console.log("tileFn", z, x, y, themeName, theme.length);

				if (symbolizer) {
					for (let i = 0; i < theme.length; i++) {
						const feat = theme.feature(i);
						// const coords = feat.loadGeometry();
						// const geom = this.#normalizeGeom(extent, bbox, coords, feat.type);
						const geom = loadProtobufferRawGeometry(
							pyramid.crs,
							feat,
							bbox,
							extent
						);

						const geomType =
							feat.type === 1
								? "Point"
								: feat.type === 2
								? "LineString"
								: feat.type === 3
								? "Polygon"
								: "Unknown";

						const symbols = symbolizer(geom, {
							$type: geomType,
							"geometry-type": geomType,
							$zoom: numericZoom,
							...feat.properties,
						});

						this.#onEachFeature?.(symbols, geom, themeName, feat.properties);

						themeSymbols[i] = symbols;
					}
				}
				return themeSymbols.flat();
			})
			.flat();

		return symbols;
	}
}
