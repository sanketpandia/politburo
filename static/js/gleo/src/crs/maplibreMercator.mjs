import BaseCRS from "./BaseCRS.mjs";

import epsg4326 from "./epsg4326.mjs";

import { registerProjectionFunction } from "./projector.mjs";

/**
 * @namespace maplibreMercator
 * @inherits BaseCRS
 *
 * An adaptation of the EPSG:3857 CRS (aka "spherical web mercator") to the
 * way that Maplibre-gl-js handles coordinates.
 *
 * The upper-left (north-west) corner in EPSG:3857 is at [-20037508.34, +20037508.34],
 * but for maplibre it has to be at [0,0]. The lower-right (south-east) corner is
 * at [+20037508.34, -20037508.34] in EPSG:3857 but maplibre expects it to be at
 * [1,1]. Likewise, Null Island (latitude zero, longitude zero) is at [0.5, 0.5].
 *
 * This is intended to be use in conjunction with [maplibre-gleo](https://gitlab.com/IvanSanchez/maplibre-gleo).
 *
 */

const maplibreMercator = new BaseCRS("maplibremercator", {
	wrapPeriodX: 1,
	distance: epsg4326,
	// ogcUri: "http://www.opengis.net/def/crs/EPSG/0/3857",
	ogcUri: "-",
	minSpan: 1e-9,
	maxSpan: 2,
	viewableBounds: [-Infinity, 0, Infinity, 1],
});

export default maplibreMercator;

const limit = 20037508.34;
const span = 2 * limit;

// Converts +/-20037508 → 0/1
function normalizeMercator([x, y]) {
	return [(x + limit) / span, 1 - (limit - y) / span];
}

// Converts 0/1 → +/-20037508
function denormalizeMercator([x, y]) {
	return [span * (x - 0.5), span * (0.5 - y)];
}

// Copied from epsg3857.mjs
const R = 6378137; // Earth's radius as per spherical mercator
const D = Math.PI / 180; // One degree, in radians
const rad = 180 / Math.PI; // One radian, in degrees
const halfPi = Math.PI / 2;

function lnglat2webmercator([lng, lat]) {
	const sin = Math.sin(lat * D);
	return [R * D * lng, (R * Math.log((1 + sin) / (1 - sin))) / 2];
}

function webmercator2lnglat([x, y]) {
	return [(x * rad) / R, (2 * Math.atan(Math.exp(y / R)) - halfPi) * rad];
}

registerProjectionFunction("EPSG:4326", "maplibremercator", ([x, y]) => {
	return normalizeMercator(lnglat2webmercator([x, y]));
});

registerProjectionFunction("maplibremercator", "EPSG:4326", ([x, y]) => {
	return webmercator2lnglat(denormalizeMercator([x, y]));
});

registerProjectionFunction("maplibremercator", "EPSG:3857", denormalizeMercator);
registerProjectionFunction("EPSG:3857", "maplibremercator", normalizeMercator);
