import { project } from "../crs/projector.mjs";
import { getCRS } from "../crs/knownCRSs.mjs";
import ExpandBox from "./ExpandBox.mjs";

/**
 * @class RawGeometry
 * @relationship compositionOf BaseCRS, 1..1, 0..n
 * @relationship associated projector
 * @relationship dependsOn knownCRSs
 *
 * Like `Geometry`, but expects the "raw" flattened coordinate array,
 * rings array, hulls array, and skips the assertions.
 *
 */

export default class RawGeometry {
	/**
	 * @constructor RawGeometry(crs: BaseCRS, coords: Array of Number, rings: Array of Number, hulls: Array of Number, options: RawGeometry Options)
	 */
	constructor(
		crs,
		coords,
		rings = [],
		hulls = [],
		{ wrap = true, dimension = 2 } = {}
	) {
		/**
		 * @section RawGeometry Options
		 * @option wrap: Boolean = true
		 * Whether antimeridian-wrap functionality should be enabled for this geometry.
		 *
		 * Only works for 2-dimensional `Geometry`s.
		 *
		 * @option dimension: Number = 2
		 * The dimension of each coordinate. 2 for X-Y, 3 for X-Y-Z, 4 for X-Y-Z-M.
		 */
		this.wrap = wrap;
		this.dimension = dimension;
		this.crs = crs;

		/**
		 * @property coords: Array of Number
		 * A flat `Array` containing the CRS-relative coordinates or the
		 * geometry, in `[x1, y1, x2, y2, ..., xn, yn]` form.
		 */
		this.coords = this.wrap ? crs.wrapString(coords) : coords;
		/**
		 * @property rings: Array of Number
		 * A flat `Array` containing indices (0-indexed) of the coordinate
		 * pairs that start a new ring.
		 */
		this.rings = rings;
		/**
		 * @property hulls: Array of Number
		 * A flat `Array` containing indices (0-indexed) of the coordinate
		 * pairs that start a new hull.
		 */
		this.hulls = hulls;
	}

	/**
	 * @method toCRS(newCRS: CRS): Geometry
	 * Returns the `Geometry`, translated/projected to the given CRS, as a new instance.
	 *
	 * If the CRS is exactly the same, `this` is returned instead.
	 * @alternative
	 * @method toCRS(newCRS: String): Geometry
	 * Idem, but takes a `String` containing the name (e.g. "EPSG:4326", "cartesian")
	 * or the OGC URI of a CRS. The corresponding CRS instance will be looked up.
	 */
	toCRS(newCRS) {
		if (newCRS === this.crs) {
			return this;
		} else if (typeof newCRS === "string") {
			return this.toCRS(getCRS(newCRS));
		} else if (newCRS.name === this.crs.name) {
			// Return a new Geometry mapping an offset to all xy pairs.
			return new RawGeometry(
				newCRS,
				newCRS.offsetFromBase(this.crs.offsetToBase(this.coords)),
				this.rings,
				this.hulls,
				{ wrap: this.wrap, dimension: 2 }
			);

			/// TODO: For wrapping CRSs, wrap the coordinate... with the original CRS
			/// wrapping limit/dimension/span, but using the offset center.
			/// So e.g. epsg:4326 offset to +170 longitude would wrap -175 to relative
			/// +15; this pushes the antimeridian opposite to the offset center.
		} else {
			// Reproject
			return new RawGeometry(
				newCRS,
				this.mapCoords((xy) =>
					newCRS.offsetFromBase(
						project(this.crs.name, newCRS.name, this.crs.offsetToBase(xy))
					)
				),
				this.rings,
				this.hulls,
				{ wrap: this.wrap, dimension: 2 }
			);
		}
	}

	/**
	 * @method asLatLng(): Array of Number
	 *
	 * Returns a **flat** array of latitude-longitude (Y-X) representing this geometry,
	 * in the form `[lat1, lng1, lat2, lng2, ... latN, lngN]`.
	 *
	 * Will throw an error if the geometry cannot be converted to latitude-longitude
	 * (i.e. is in a CRS that cannot be reprojected to EPSG:4326).
	 *
	 */
	asLatLng() {
		const xys = this.asLngLat();

		const yxs = new Array(xys.length);
		for (let i = 0, l = xys.length; i < l; i += 2) {
			yxs[i] = xys[i + 1];
			yxs[i + 1] = xys[i];
		}
		return yxs;
		/*
		return new Array(xys,(_,i)=> xys[
			(i >> 1 << 1) + // Get rid of the least significant bit
			!(i % 2) // Reverse of modulo 2
		]);*/
	}

	/**
	 * @method asLngLat(): Array of Number
	 *
	 * Returns a **flat** array of longitude-latitude (X-Y) representing this geometry,
	 * in the form `[lng1, lat1, lng2, lat2, ... lngN, latN]`.
	 *
	 * Will throw an error if the geometry cannot be converted to latitude-longitude
	 * (i.e. is in a CRS that cannot be reprojected to EPSG:4326).
	 *
	 */
	asLngLat() {
		return this.toCRS("EPSG:4326").coords;
	}

	#loops;
	/**
	 * @property loops: Array of Boolean
	 * A read-only `Array` with one `Boolean` values per ring. The value is
	 * `true` for those rings which forms loops (the first coordinate pair
	 * equals the last one).
	 */
	get loops() {
		if (this.#loops) {
			return this.#loops;
		}

		/// NOTE: This implementation only works for dimension 2 (XY) geometries.
		/// Ideally t should cover XYZ/XYM/ZYZM geometries.

		/// FIXME: Logic when the linestring loops across the antimridian.
		/// Compare start and end points, **taking into account CRS wrapping**.
		return (this.#loops = this.mapRings((start, end) => {
			const start2 = start * 2;
			const end2 = 2 * (end - 1);
			return (
				this.coords[start2 + 0] === this.coords[end2 + 0] &&
				this.coords[start2 + 1] === this.coords[end2 + 1]
			);
		}));
	}

	/**
	 * @property loops: Array of Boolean
	 * A read-only containing the starts (inclusive) and ends (exclusive)
	 * of all rings.
	 *
	 * Contains, at least, `0` and the amount of coordinate pairs (for
	 * geometries with just one ring)
	 */
	#stops;
	get stops() {
		if (this.#stops) {
			return this.#stops;
		}
		return (this.#stops = [
			0,
			...this.rings,
			...this.hulls,
			this.coords.length / this.dimension,
		].sort((a, b) => a - b));
	}

	/**
	 * @method mapCoords(fn: Function): Array of Number
	 * Returns a new array of the form `[x1,y1, ... xn,yn]`, having run the given
	 * `Function` on every `x,y` pair of coordinates from self. The `Function`
	 * must take an `Array` of 2 `Number`s as its first parameter (the coordinate
	 * pair), a `Number` as its second parameter (the index of the current
	 * coordinate pair, 0-indexed), and must return an `Array` of 2 `Number`s as well.
	 *
	 * Takes into account the dimension (dimension 3 works for `x,y,z` and dimension 4
	 * works for `x,y,z,m`)
	 *
	 */
	mapCoords(fn) {
		const d = this.dimension;
		const l = this.coords.length / d;
		const result = new Array(l);

		for (let i = 0; i < l; i++) {
			const j = i * d;
			result[i] = fn(this.coords.slice(j, j + d), i);
		}

		return result.flat();
	}

	/**
	 * @method mapRings(fn: Function): Array
	 * Runs the given `Function` once per ring (including each ring in each hull, if
	 * applicable), and returns an `Array` containing the return values.
	 *
	 * The given `Function` can expect four parameters:
	 * * `start` coordinate (0-indexed, inclusive)
	 * * `end` coordinate (0-indexed, exclusive)
	 * * `length` of the ring (how many coordinates in that ring, also `end-start`)
	 * * `i`, index of the current ring (0-indexed)
	 */
	mapRings(fn) {
		// I'm sure this can be done more efficiently in a C-like fashion
		// (i.e. pulling a value from either this.rings or this.hulls at
		// each pass of the loop, no array sorting/concat'ing), but this
		// should do for now.

		let stops = this.stops;

		const result = new Array(stops.length - 1);

		for (let i = 0, l = stops.length - 1; i < l; i++) {
			const start = stops[i];
			const end = stops[i + 1];
			result[i] = fn(start, end, end - start, i);
		}

		return result;
	}

	#cachedBBox;

	/**
	 * @method bbox(): ExpandBox
	 * Calculates and returns the bounding box of the geometry. The coordinates
	 * of the resulting will implicitly be in the geometry's CRS.
	 */
	bbox() {
		return (this.#cachedBBox ??= new ExpandBox().expandGeometry(this)).clone();
	}
}
