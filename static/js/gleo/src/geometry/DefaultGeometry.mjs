import RawGeometry from "./RawGeometry.mjs";

const nullGeometry = new RawGeometry({ name: "null" }, [], [], [], { wrap: false });

/**
 * @namespace DefaultGeometry
 * @relationship associated Geometry
 *
 * This module contains facilities for easing the task of defining geometries
 * when a well-known CRS is used for all input coordinates.
 *
 * Note that this functionality is **global to all maps** since it must work for
 * stuff which does not belong to any map, or belongs to more than one map at
 * the same time.
 *
 * @example
 * ```
 * import { setFactory, factory } from `gleo/src/coords/DefaultGeometry.mjs`;
 *
 * setFactory(function(coords, opts) {
 * 	return new Geometry( myFavouriteCRS, coords, opts);
 * }
 *
 * map.center = [100, -500]; // These stand-alone coordinates will internally
 *                           // be converted into a `Geometry` of `myFavouriteCRS`
 * ```
 *
 * Calling `setFactory` more than once (either manually or by importing `MercatorMap`)
 * is highly discouraged.
 */

let currentFactory = function uninitializedDefaultGeometry() {
	throw new Error(
		`A way to spawn geometries without an explicit CRS has not been defined`
	);
};

/**
 * @function setFactory(fn: Function): undefined
 * Sets the factory function for transforming stand-alone coordinates into
 * geometries with a specific CRS.
 *
 * The function must take a single `coordinates` argument, and return a `Geometry`
 * instance.
 */
export function setFactory(fn) {
	currentFactory = fn;
}

/**
 * @function factory(coords: Array of Number, opts?: Geometry Options): RawGeometry
 * The factory function that takes coordinates and returns `Geometry`s.
 *
 * All Gleo functionality that can take geometries as input **must** pass it
 * through this factory function.
 *
 * In some cases, geometry constructor options (such as `deduplicate` or `wrap`)
 * will be passed. The factory function should honour these.
 * @alternative
 * @function factory(geom: RawGeometry, opts?: Geometry Options): RawGeometry
 * When the factory function receives a `Geometry`, it is returned as is.
 *
 * In this way, all Gleo functionality that uses this factory will be able to
 * take as input either stand-alone coordinates, or fully-defined `Geometry`s,
 * in a transparent way.
 *
 * This check is performed by the `DefaultGeometry` module; users do **not** need
 * to check whether the input is a `RawGeometry` instance when using `setFactory()`.
 * @alternative
 * @function factory(geom: undefined): RawGeometry
 * A "null" geometry, containing zero vertices, will be returned.
 */
export function factory(coords, opts) {
	if (coords instanceof RawGeometry) {
		return coords;
	}
	if (coords === undefined) {
		return nullGeometry;
	}
	return currentFactory(coords, opts);
}
