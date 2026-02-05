import LngLat from "./LngLat.mjs";

/**
 * @class LatLng
 * @inherits LngLat
 *
 * A `Geometry` of latitude-longitude coordinates, assuming EPSG:4326.
 *
 * Note that the order of the axis is inverted: `new LatLng([90, 180])`
 * is equivalent to `new Geometry(epsg4326, [180, 90])`.
 *
 * The issue of the order of the axis might be confusing. See also `LatLng` and
 * https://macwright.com/lonlat/ .
 */

export default class LatLng extends LngLat {
	/**
	 * @constructor LatLng(xy: Array of Number, opts?: Geometry Options)
	 */
	constructor(yx, opts) {
		const xy = flip(yx);
		super(xy, opts);
	}
}

function flip(arr) {
	if (typeof arr[0] === "number") {
		return [arr[1], arr[0]];
	} else {
		return arr.map(flip);
	}
}
