import BaseCRS from "./BaseCRS.mjs";

import epsg4326 from "./epsg4326.mjs";

/**
 * @namespace epsg3857
 * @inherits BaseCRS
 *
 * A EPSG:3857 CRS - aka "spherical web mercator".
 *
 * Note that `epsg3857` works as a Singleton pattern - it's already an instance, so
 * do **not** call `new epsg3857()`.
 *
 * @example
 *
 * ```
 * import epsg3857 from 'gleo/src/crs/epsg3857.mjs';
 * import Geometry from 'gleo/src/geometry/Geometry.mjs';
 *
 * let myPoint = new Geometry(epsg3857, [5, 9]);
 * ```
 *
 */

const limit = 20037508.34;

const epsg3857 = new BaseCRS("EPSG:3857", {
	wrapPeriodX: 2 * limit,
	distance: epsg4326,
	ogcUri: "http://www.opengis.net/def/crs/EPSG/0/3857",
	minSpan: 1,
	maxSpan: 4 * limit,
	viewableBounds: [-Infinity, -limit, Infinity, limit],
});

export default epsg3857;
