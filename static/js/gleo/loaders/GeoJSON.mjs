import Loader from "./Loader.mjs";
import Stroke from "../symbols/Stroke.mjs";
import Fill from "../symbols/Fill.mjs";
import CircleStroke from "../symbols/CircleStroke.mjs";
import CircleFill from "../symbols/CircleFill.mjs";
import LngLat from "../geometry/LngLat.mjs";

function defaultPointSymbolizer(feature, geometry) {
	return [new CircleStroke(geometry), new CircleFill(geometry)];
}

function defaultLinestringSymbolizer(feature, geometry) {
	return [new Stroke(geometry)];
}

function defaultPolygonSymbolizer(feature, geometry) {
	return [new Stroke(geometry), new Fill(geometry)];
}

/**
 * @class GeoJSON
 * @inherits Loader
 * @relationship compositionOf GleoSymbol, 0..1, 0..n
 *
 * A `Loader` for requesting, parsing and symbolizing data in [GeoJSON format](https://geojson.org/)
 */
export default class GeoJSON extends Loader {
	#symbols = [];

	/**
	 * @constructor GeoJSON(json: Object, options: GeoJSON Options)
	 * Parses the data in the given JSON structure. The JSON must be conformant
	 * to the GeoJSON specification.
	 * @alternative
	 * @constructor KML(blob: Blob, options: KML Options)
	 * If given a `Blob` (which also includes `File`s), it will be parsed as GeoJSON.
	 * @alternative
	 * @constructor GeoJSON(url: URL, options: GeoJSON Options)
	 * If given a URL object, that URL will be requested, and the returned
	 * GeoJSON will be parsed.
	 * @alternative
	 * @constructor GeoJSON(url: String, options: GeoJSON Options)
	 * When given a `String`, it will be trated as an `URL`.
	 */
	constructor(
		geojson,
		{
			/**
			 * @section GeoJSON Options
			 * @option pointSymbolizer: Function = *
			 * A `Function` that defines how features with a point (or multipoint)
			 * geometry get transformed into `GleoSymbol`s. It receives the feature
			 * as its first parameter, an instance of a Gleo `Geometry` as its
			 * second parameter, and must return an array of `GleoSymbol`s.
			 *
			 * When not specified, a default implementation is used. This default
			 * implementation symbolizes point features with a `CircleFill` and
			 * a `CircleStroke` with default options.
			 *
			 * This function may be called more than once for the same feature.
			 *
			 *
			 * @option lineSymbolizer: Function = *
			 * Akin to `pointSymbolizer`, but for linestrings (and multilinestrings).
			 *
			 * When not specified, the default is to use a default `Stroke`.
			 *
			 * @option polygonSymbolizer: Function = *
			 * Akin to `pointSymbolizer`, but for polygons (and multipolygons).
			 *
			 * When not specified, the default is to use default `Stroke` and `Fill`.
			 */
			pointSymbolizer = defaultPointSymbolizer,
			lineSymbolizer = defaultLinestringSymbolizer,
			polygonSymbolizer = defaultPolygonSymbolizer,
			...opts
		} = {}
	) {
		super(opts);
		/**
		 * @property pointSymbolizer: Function
		 * The `Function` currently used for turning a feature and one of its
		 * geometries into a set of `GleoSymbol`s.
		 * Changing this function during runtime does **not** trigger a
		 * re-symbolization of the dataset.
		 * @option lineSymbolizer: Function
		 * Akin to `pointSymbolizer`, but for linestrings (and multilinestrings).
		 * @option polygonSymbolizer: Function
		 * Akin to `pointSymbolizer`, but for polygons (and multipolygons).
		 */
		this.pointSymbolizer = pointSymbolizer;
		this.lineSymbolizer = lineSymbolizer;
		this.polygonSymbolizer = polygonSymbolizer;

		if (geojson instanceof Blob) {
			geojson.text().then((json) => {
				this.#symbols = this._symbolizeFeature(JSON.parse(json));
				this.fire("symbolsadded", { symbols: this.#symbols });
				this.target?.multiAdd(this.#symbols);
			});
		} else if (geojson.type) {
			// Assuming a well-formed GeoJSON data structure was received
			this.#symbols = this._symbolizeFeature(geojson);
			this.fire("symbolsadded", { symbols: this.#symbols });
		} else {
			// Assuming URL, or url-like string
			let url = geojson instanceof URL ? geojson : new URL(geojson, document.URL);

			fetch(url)
				.then((response) => response.json())
				.then((json) => {
					this.#symbols = this._symbolizeFeature(json);
					this.fire("symbolsadded", { symbols: this.#symbols });
					this.target?.multiAdd(this.#symbols);
				});
		}
	}

	addTo(target) {
		super.addTo(target);
		return this;
	}

	_addToPlatina(platina) {
		super._addToPlatina(platina);
		this.platina.multiAdd(this.#symbols);
	}

	remove() {
		this.platina?.multiRemove(this.#symbols);
		return super.remove();
	}

	_symbolizeFeature(feature) {
		switch (feature.type) {
			case "Feature":
				return this._symbolizeFeatureGeometry(feature, feature.geometry);
			case "FeatureCollection":
				return feature.features.map(this._symbolizeFeature.bind(this)).flat();
			default:
				throw new Error(
					`Malformed GeoJSON: Expected item of type either FeatureCollection or Feature, but found ${feature.type}`
				);
		}
	}

	_symbolizeFeatureGeometry(feature, geometry) {
		if (geometry.type === "GeometryCollection") {
			return geometry.geometries
				.map((g) => this._symbolizeFeatureGeometry(feature, g))
				.flat();
		} else {
			const gleoGeometry = new LngLat(geometry.coordinates);
			switch (geometry.type) {
				case "Point":
				case "MultiPoint":
					return this.pointSymbolizer(feature, gleoGeometry);

				case "LineString":
				case "MultiLineString":
					return this.lineSymbolizer(feature, gleoGeometry);

				case "Polygon":
				case "MultiPolygon":
					return this.polygonSymbolizer(feature, gleoGeometry);

				default:
					throw new Error(
						`Malformed GeoJSON: Expected geometry of type either (Multi)Point, (Multi)LineString, (Multi)Polygon or GeometryCollection, but found ${geometry.type}`
					);
			}
		}
	}
}
