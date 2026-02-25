/**
 * @namespace projector
 *
 * The `projector` is the piece of code in charge of transforming ("reprojecting")
 * coordinates (from either `Coord` or `CoordNest`) into a different `BaseCRS`.
 *
 * By default, Gleo only supports projecting from/to `EPSG:4326` and `EPSG:3857`.
 * The intended way to support any other projections is to inject the
 * `proj4`/`proj4js` dependency via `enableProj()`.
 *
 * `projector` works as a Singleton pattern, and cannot be instanced.
 *
 * @example
 * ```
 * import proj4 from 'proj';
 * import {enableProj, project} from 'gleo/src/crs/projector.mjs';
 *
 * enableProj(Proj4js);
 *
 * proj4.defs("EPSG:3995","+proj=stere +lat_0=90 +lat_ts=71 +lon_0=0 +k=1 +x_0=0 +y_0=0 +datum=WGS84 +units=m +no_defs");
 *
 * const epsg3995 = new BaseCRS("EPSG:3995", Infinity, Infinity);
 *
 * map.crs = epsg3995;
 * ```
 */

function gleoProject(sCRS, dCRS, xy) {
	if (sCRS === "EPSG:4326" && dCRS === "EPSG:3857") {
		return lnglat2webmercator(xy);
	} else if (sCRS === "EPSG:3857" && dCRS === "EPSG:4326") {
		return webmercator2lnglat(xy);
	} else {
		throw new Error(`Unsupported coordinate reprojection ${sCRS}→${dCRS}`);
	}
}

gleoProject.defs = function noop() {};

/**
 * @function project(sCRS: String, dCRS: String, xy: Array of Number): Array of Number
 * Projects the given `Array` of two `Number`s from the source CRS `sCRS` into
 * the destination `dCRS`, returning a new `Array` of two `Number`s.
 */
export let project = gleoProject;

/**
 * @function registerProjectionFunction(sCRS: String, dCRS: String, fn: Function): undefined
 * Registers the given projection function, so it will be used whenever Gleo
 * needs to project coordinates from `sCRS` into `dCRS`.
 *
 */
export function registerProjectionFunction(sCRS, dCRS, fn) {
	const prev = gleoProject;
	gleoProject = function gleoProject(s, d, xy) {
		if (s === sCRS && d === dCRS) {
			return fn(xy);
		} else {
			return prev(s, d, xy);
		}
	};
	if (project === prev) {
		project = gleoProject;
	}
}

/**
 * @function enableProj(proj: Module): undefined
 * Expects a reference to the `proj4` (AKA `proj4js`) module. All further reprojections
 * (including those from `Coord.toCRS()`) shall be done via the specified module.
 * @alternative
 * @function enableProj(undefined: undefined): undefined
 * Disables usage of `proj4`/`proj4js`, and re-enables Gleo's built-in reprojection code.
 */
export function enableProj(proj) {
	if (proj) {
		project = proj;
	} else {
		project = gleoProject;
	}
}

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
