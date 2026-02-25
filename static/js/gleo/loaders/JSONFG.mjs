import GeoJSON from "./GeoJSON.mjs";
import Geometry from "../geometry/Geometry.mjs";
import epsg4326 from "../crs/epsg4326.mjs";

import { getCRS as getKnownCRS } from "../crs/knownCRSs.mjs";

// Returns a `BaseCRS` given the value of the `coordRefSys` property in
// the JSON-FG structure
function getCRS(def, fallback) {
	if (typeof def === "string") {
		return getKnownCRS(def);
	} else if (def?.href) {
		return getKnownCRS(def.href);
	} else {
		return fallback;
	}
}

/**
 * @class JSONFG
 * @inherits GeoJSON
 * @relationship compositionOf GleoSymbol, 0..1, 0..n
 * @relationship dependsOn knownCRSs
 *
 * A `Loader` for requesting, parsing and symbolizing data in "OGC Features and
 * Geometries JSON" format (AKA "OGC JSON-FG"). JSON-FG is OGC's proposal for
 * extending GeoJSON.
 *
 * This Gleo implementation is built according to the *draft* specification as
 * of 2022-09, from https://docs.ogc.org/DRAFTS/21-045.html .
 *
 * GeoJSON files can only contain data in latitude-longitude coordinates, but
 * JSON-FG allows for other coordinate systems. This Gleo implementation
 * handles this extra CRS information, so that geometries work as they should.
 *
 * Due to technical restrictions, any CRSs referred to in the data **must**
 * already have a corresponding Gleo `BaseCRS` defined *elsewhere*.
 */
export default class JSONFG extends GeoJSON {
	/**
	 * @constructor JSONFG(json: Object, options: JSONFG Options)
	 * Parses the data in the given JSON structure. The JSON must be conformant
	 * to the JSON-FG specification.
	 * @alternative
	 * @constructor JSONFG(url: URL, options: JSONFG Options)
	 * If given a URL object, that URL will be requested, and the returned
	 * JSON-FG will be parsed.
	 * @alternative
	 * @constructor JSONFG(url: String, options: JSONFG Options)
	 * When given a `String`, it will be trated as an `URL`.
	 */
	constructor(jsonfg, opts) {
		super(jsonfg, opts);
	}

	_symbolizeFeature(feature, crs) {
		// This rewrites GeoJSON's `_symbolizeFeature`, adding support for the
		// `place` property, which takes priority over `geometry`.
		const featCrs = getCRS(feature.coordRefSys, crs);

		switch (feature.type) {
			case "Feature":
				if (feature.geometry) {
					return this._symbolizeFeatureGeometry(feature, feature.geometry);
				} else if (feature.place) {
					return this._symbolizeFeaturePlace(feature, feature.place, featCrs);
				} else {
					throw new Error("JSON-FG Feature has neither geometry of place.");
				}
			case "FeatureCollection":
				return feature.features
					.map((feat) => this._symbolizeFeature(feat, featCrs))
					.flat();
			default:
				throw new Error(
					`Malformed JSON-FG: Expected item of type either FeatureCollection or Feature, but found ${feature.type}`
				);
		}
	}

	_symbolizeFeaturePlace(feature, place, crs) {
		const geomCrs = getCRS(place.coordRefSys, crs);

		if (place.type === "GeometryCollection") {
			return place.geometries
				.map((g) => this._symbolizeFeaturePlace(feature, g, geomCrs))
				.flat();
		} else {
			const gleoGeometry = new Geometry(geomCrs, place.coordinates);
			switch (place.type) {
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
						`Unsupported/malformed JSON-FG: Expected place of type either (Multi)Point, (Multi)LineString, (Multi)Polygon or GeometryCollection, but found ${place.type}`
					);
			}
		}
	}
}
