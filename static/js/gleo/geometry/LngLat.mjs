import Geometry from "./Geometry.mjs";
import epsg4326 from "../crs/epsg4326.mjs";

/**
 * @class LngLat
 * @inherits Geometry
 * @relationship dependsOn epsg4326, 0..n, 1..1
 *
 * A `Geometry` of longitude-latitude coordinates, assuming EPSG:4326.
 *
 * Note that the order of the axis is longitude-latitude, or x-y: `new LngLat([180, 90])`
 * is equivalent to `new Coord(epsg4326, [180, 90])`.
 *
 * The issue of the order of the axis might be confusing. See also `LatLng` and
 * https://macwright.com/lonlat/ .
 */

export default class LngLat extends Geometry {
	/**
	 * @constructor LngLat(xy: Array of Number)
	 */
	constructor(xy, opts) {
		super(epsg4326, xy, opts);
	}
}
