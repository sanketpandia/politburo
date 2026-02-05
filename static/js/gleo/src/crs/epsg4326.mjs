import BaseCRS from "./BaseCRS.mjs";
import { registerCRS } from "./knownCRSs.mjs";

/**
 * @namespace epsg4326
 * @inherits BaseCRS
 *
 * A EPSG:4326 CRS - aka "latitude-longitude".
 *
 * Note that `epsg4326` works as a Singleton pattern - it's already an instance, so
 * do **not** call `new epsg4326()`.
 *
 * @example
 *
 * ```
 * import epsg4326 from 'gleo/src/crs/epsg4326.mjs';
 * import Geometry from 'gleo/src/geometry/Geometry.mjs';
 *
 * let myPoint = new Geometry(epsg4326, [5, 9]);
 * ```
 *
 */

const rad = Math.PI / 180;
const R = 6371000;

const epsg4326 = new BaseCRS("EPSG:4326", {
	wrapPeriodX: 360,
	distance: function haversineDistance(p1, p2) {
		// Haversine formula for great-circle distance. Based on an implementation by
		// Jussi Mattas (https://github.com/jussimattas / https://github.com/gitjuba)
		// See https://github.com/Leaflet/Leaflet/pull/5935
		const lat1 = p1.coords[1] * rad,
			lat2 = p2.coords[1] * rad,
			sinDLat = Math.sin(((p2.coords[1] - p1.coords[1]) * rad) / 2),
			sinDLon = Math.sin(((p2.coords[0] - p1.coords[0]) * rad) / 2),
			a = sinDLat * sinDLat + Math.cos(lat1) * Math.cos(lat2) * sinDLon * sinDLon,
			c = 2 * Math.atan2(Math.sqrt(a), Math.sqrt(1 - a));
		return R * c;
	},
	ogcUri: "http://www.opengis.net/def/crs/EPSG/0/4326",
	flipAxes: true,
	minSpan: 1e-6, // circa 0.1m at equator
	maxSpan: 720,
	viewableBounds: [-Infinity, -90, Infinity, 90],
});

export default epsg4326;

// OGC URI Alias
/// TODO: axis order???!!!
registerCRS(epsg4326, "http://www.opengis.net/def/crs/OGC/1.3/CRS84");
