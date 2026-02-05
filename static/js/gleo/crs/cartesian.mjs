import BaseCRS from "./BaseCRS.mjs";

/**
 * @namespace cartesian
 * @inherits BaseCRS
 *
 * A cartesian CRS - with infinite bounds, no wrapping, X going right, and Y going up.
 *
 * Note that `cartesian` works as a Singleton pattern - it's already an instance, so
 * do **not** call `new cartesian()`.
 *
 * @example
 *
 * ```
 * import cartesian from 'gleo/src/crs/cartesian.mjs';
 * import Geometry from 'gleo/src/crs/coord.mjs';
 *
 * let myPoint = new Geometry(cartesian, [5, 9]);
 * ```
 *
 */

const cartesian = new BaseCRS("cartesian", {
	distance: function euclideanDistance(a, b) {
		return Math.sqrt(
			Math.pow(a.coords[0] - b.coords[0], 2) +
				Math.pow(a.coords[1] - b.coords[1], 2)
		);
	},
	ogcUri: "OGC:engineering-2d",
});

export default cartesian;
