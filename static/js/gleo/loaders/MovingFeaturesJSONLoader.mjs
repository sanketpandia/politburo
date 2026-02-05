import Loader from "./Loader.mjs";


import LngLat from "../geometry/LngLat.mjs";



/**
 * @class MovingFeaturesJSON
 * @inherits Loader
 *
 * A `Loader for requesting, parsing and symbolizing data in
 * [Moving Features JSON](https://github.com/opengeospatial/mf-json) standard format.
 *
 *
 */

export default class MovingFeaturesJSON extends Loader {
	#symbols = [];

	/**
	 * @constructor MovingFeaturesJSON(json: Object, options: MovingFeaturesJSON Options)
	 * Parses the data in the given JSON structure. The JSON must be conformant
	 * to the MovingFeaturesJSON specification.
	 * @alternative
	 * @constructor MovingFeaturesJSON(blob: Blob, options: MovingFeaturesJSON Options)
	 * If given a `Blob` (which also includes `File`s), it will be parsed as MovingFeaturesJSON.
	 * @alternative
	 * @constructor MovingFeaturesJSON(url: URL, options: MovingFeaturesJSON Options)
	 * If given a URL object, that URL will be requested, and the returned
	 * MovingFeaturesJSON will be parsed.
	 * @alternative
	 * @constructor MovingFeaturesJSON(url: String, options: MovingFeaturesJSON Options)
	 * When given a `String`, it will be trated as an `URL`.
	 */
	constructor(
		mfjson,
		{
			/**
			 * @section MovingFeaturesJSON Options
			 * @option movingPointSymbolizer: Function = *
			 * A `Function` that defines how features with a Moving Point
			 * temporal geometry get transformed into `GleoSymbol`s.
			 *
			 * Must return an array of (zero or more) `GleoSymbol`s.
			 *
			 * When not specified, a default implementation is used. This default
			 * implementation symbolizes point features with a `CircleFill` and
			 * a `CircleStroke` with default options.
			 *
			 */
			movingPointSymbolizer,

			...opts
		} = {}
	) {
		super(opts);

		this.movingPointSymbolizer = movingPointSymbolizer;


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


	// As per the spec:
	// An instant object is only a JSON string encoded by ISO 8601 field-based formats using Z or the number of milliseconds since midnight (00:00 a.m.) on January 1, 1970, in UTC
	// This returns the milliseconds since unix epoch.
	_parseDatetime(datetime) {
		if (typeof datetime === "Number") {return datetime;}
		if (typeof datetime === "String") {return Date.parse(datetime);}
	}


	_setMinMaxTimestamp(feature) {
		let minmax = this._getMinMaxTimestamp(feature);
		this.minTimestamp = minmax[0];
		this.maxTimestamp = minmax[1];
	}

	// Returns an array of the form [min, max] with the mininum/maximum timestamps
	// for that feature.
	_getMinMaxTimestamp(feature) {
		switch (feature.type) {
			case "Feature":
				return [
					this._parseDatetime(feature.datetimes[0]),
					this._parseDatetime(feature.datetimes[feature.datetimes.length -1])
				];
			case "FeatureCollection":
				let min = Infinity;
				let max = -Infinity;
				for (f of feature.features) {
					const [minF, maxF] = this._getMinMaxTimestamp(f);
					min = Math.min(min, minF);
					max = Math.max(min, maxF);
				}
				return [min, max];
			default:
				throw new Error(
					`Malformed GeoJSON: Expected item of type either FeatureCollection or Feature, but found ${feature.type}`
				);
		}
	}


	_symbolizeFeature(feature) {
		switch (feature.type) {
			case "Feature":
				return this._symbolizeFeatureGeometry(feature, feature.temporalGeometry);
			case "FeatureCollection":
				return feature.features.map(this._symbolizeFeature.bind(this)).flat();
			default:
				throw new Error(
					`Malformed GeoJSON: Expected item of type either FeatureCollection or Feature, but found ${feature.type}`
				);
		}
	}

	_symbolizeFeatureGeometry(feature, temporalGeometry) {
		// if (geometry.type === "GeometryCollection") {
		// 	return geometry.geometries
		// 		.map((g) => this._symbolizeFeatureGeometry(feature, g))
		// 		.flat();
		// } else {
			const gleoGeometry = new LngLat(temporalGeometry.coordinates);

			const mcoords = temporalGeometry.datetimes.map(d=>
				this._parseDatetime(d) - this.minTimestamp
			);

			switch (temporalGeometry.type) {
				case "MovingPoint":
					return this.movingPointSymbolizer(feature, gleoGeometry, mcoords);

				default:
					throw new Error(
						`Unsupported temporal geometry type (expected 'MovingPoint' but found '${geometry.type}')`
					);
			}
		// }
	}

}


