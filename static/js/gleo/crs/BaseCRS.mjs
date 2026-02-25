import { getCRS, registerCRS } from "./knownCRSs.mjs";
import { project } from "./projector.mjs";

/**
 * @class BaseCRS
 *
 * Represents a Coordinate Reference System.
 *
 * A `CRS` is identified by:
 * - Its well-known name (e.g. "EPSG:4326" or "cartesian")
 * - A wrapping delta vector (the `wrapPeriodX` and `wrapPeriodY` options)
 * - A way to calculate distance, either
 *   - A distance function that takes two points in the CRS, *or*
 *   - An instance of a different CRS that will perform the distance
 *     calculations after reprojecting the points
 * - An bounding box (in the `[minX, minY, maxX, maxY]` form)
 *   informative of the area in which the CRS makes sense; this is mostly to
 *   prevent users from setting the platina's center too far away.
 * - An informative set of minimum and maximum span; this is mostly to
 *   prevent users from zooming in/out foo far.
 *
 * Members of bounding boxes, as well as the minimum/maximum span, can have
 * values of `Infinity` or `-Infinity` (or `Number.POSITIVE_INFINITY`/`Number.NEGATIVE_INFINITY`).
 *
 * The wrapping delta vector is meant to draw `Acetate`s multiple times, offsetting
 * by that value. Also used to wrap coordinates when their X (or Y) component is larger
 * than *half* the X (or Y) component of the wrapping delta vector.
 *
 * It's important to note that the the wrapping vector does **not** mean that
 * coordinates wrap, but rather that the *display* of the coordinates wrap.
 * Thus, a point at a position `P` and another point at `P+wrap` are different,
 * but will be displayed at the same pixel. (Idem for `P+2*wrap`, `P-wrap` and
 * in general, for `P+n*wrap` for all natural numbers `n`).
 *
 * For geographical CRSs, it's highly recommended to use names that match
 * a Proj definition. Other functionality, such as the `ConformalWMS` loader,
 * depends on the CRS names.
 */

export default class BaseCRS {
	/**
	 * @constructor BaseCRS(name: String, opts: BaseCRS Options)
	 */
	constructor(
		name,
		{
			/**
			 * @section
			 * @aka BaseCRS Options
			 * @uninheritable
			 * @option wrapPeriodX: Number = Infinity
			 * The horizontal length, in CRS units, of the display wrapping.
			 * @option wrapPeriodY: Number = Infinity
			 * The vertical length, in CRS units, of the display wrapping.
			 *
			 * @option distance: Function
			 * A distance function, that should take two point `Geometry`s as arguments
			 * and return the distance between them (when given non-point `Geometry`s,
			 * it shall return the distance between the first coordinate pair of each
			 * `Geometry`). The units of distance depend on the CRS. They **should**
			 * be meters for geographical CRSs, and unitless for `cartesian`.
			 * @alternative
			 * @option distance: BaseCRS
			 * Whenever it's not trivial or convenient to calculate distances,
			 * calculations can be proxied to another CRS (which must have a `distance`
			 * function defined). This is useful for geographical CRSs where a CRS
			 * represents a geoid (e.g. the `EPSG:4326` CRS represents the `WGS84` geoid)
			 * and all CRSs for that geoid use one common distance calculation.
			 *
			 * @option flipAxes: Boolean = false
			 * Used **only** for OGC services (WMS, WFS, etc). The default `false` works
			 * for CRSs with an X-Y axis order (or easting-northing, or longitude-latitude).
			 *
			 * This should be set to `true` whenever the CRS definition specifies that
			 * the axes should be in Y-X order (or northing-easting, or latitude-longitude)
			 * (e.g. EPSG:4326 and EPSG:3035).
			 *
			 * Gleo `Geometry`s always store data in X-Y (or easting-northing, or
			 * lng-lat) order, regardless of this setting.
			 *
			 * @option ogcUri: String = ""
			 * Used **only** for OGC API services (OGC API Tiles, etc). This
			 * should be a URI string like `"https://www.opengis.net/def/crs/EPSG/0/4326"`
			 * that will be used to match metadata.
			 *
			 * @option minSpan: Number = 0
			 * The minimum span of a map/platina using the CRS, expressed in CRS units
			 * (meters/degrees/etc) across the diagonal of a platina.
			 *
			 * This is an informative value that actuators use to prevent the user from
			 * zooming in too far.
			 *
			 * @option maxSpan: Number = Infinity
			 * The maximum span of a map/platina using the CRS, expressed in CRS units
			 * (meters/degrees/etc) across the diagonal of a platina.
			 *
			 * This is an informative value that actuators use to prevent the user from
			 * zooming out too far.
			 *
			 * @option viewableBounds: Array of Number = [-Infinity, -Infinity, Infinity, Infinity]
			 * The *practical* viewable bounds of the CRS, as an array of the form
			 * `[x1, y1, x2, y2]`.
			 *
			 * This is an informative value that actuators use to prevent the user
			 * from moving away from areas where the CRS makes sense.
			 *
			 * Values are absolute, not relative to the CRS's offset (important
			 * for instances of `offsetCRS`).
			 */
			wrapPeriodX = Infinity,
			wrapPeriodY = Infinity,
			distance,
			flipAxes = false,
			ogcUri = "",
			minSpan = 0,
			maxSpan = Infinity,
			viewableBounds = [-Infinity, -Infinity, Infinity, Infinity],
		} = {}
	) {
		Object.defineProperty(this, "name", { value: name, writable: false });
		this.wrapPeriodX = wrapPeriodX;
		this.wrapPeriodY = wrapPeriodY;
		this.halfPeriodX = wrapPeriodX / 2;
		this.halfPeriodY = wrapPeriodY / 2;

		if (wrapPeriodX === Infinity && wrapPeriodY === Infinity) {
			this.wrap = this._wrapNone;
			this.wrapString = this._wrapNone;
		} else if (wrapPeriodX !== Infinity && wrapPeriodY === Infinity) {
			this.wrap = this._wrapX;
		} else if (wrapPeriodX === Infinity && wrapPeriodY !== Infinity) {
			this.wrap = this._wrapY;
		} else {
			this.wrap = this._wrapXY;
		}

		if (distance instanceof Function) {
			// Ensure both parameters are in the desired CRS before going on
			this.distance = function assertedDistance(a, b) {
				return distance(a.toCRS(this), b.toCRS(this));
			}.bind(this);
		} else if (distance instanceof BaseCRS) {
			const proxiedCRS = distance;
			// Proxy to the distance calculation of the given CRS
			// The CRS that this calculation is ultimately proxied to
			// will assert geometries are reprojected.
			this.distance = function proxiedDistance(a, b) {
				return proxiedCRS.distance(a, b);
			};
		} else {
			this.distance = function noDistance() {
				throw new Error("CRS cannot calculate distances");
			};
		}

		this.flipAxes = flipAxes;
		this.minSpan = minSpan;
		this.maxSpan = maxSpan;
		this.viewableBounds = viewableBounds;
		// console.log("Instantiated CRS", this.name, this.wrapPeriodX, this.wrapPeriodY);
		this.ogcUri = ogcUri;

		if (ogcUri && this.constructor === BaseCRS) {
			registerCRS(this);
		}
	}

	/**
	 * @method offsetToBase(xy: Array of Number): Array of Number
	 * Identity function (all coordinates represented in a Base CRS are already
	 * relative to the 0,0 origin of coordinates).
	 */
	offsetToBase(xy) {
		return xy;
	}

	/**
	 * @method offsetFromBase(xy: Array of Number): Array of Number
	 * Identity function (all coordinates represented in a Base CRS are already
	 * relative to the 0,0 origin of coordinates).
	 */
	offsetFromBase(xy) {
		return xy;
	}

	/**
	 * @method wrap(xy: Array of Number, ref: Array of Number): Array of Number
	 * Wraps the given coordinate if it's further away from the reference `ref`
	 * than half the wrap period. This guarantees that the return value is less than
	 * half a period away from the reference.
	 */

	_wrapNone(xy) {
		return xy;
	}

	_wrapX([x, y], [refX, refY]) {
		return [
			modulo(x - refX + this.halfPeriodX, this.wrapPeriodX) +
				refX -
				this.halfPeriodX,
			y,
		];
	}

	_wrapY([x, y], [refX, refY]) {
		return [
			x,
			modulo(y - refY + this.halfPeriodY, this.wrapPeriodY) +
				refY -
				this.halfPeriodY,
		];
	}

	_wrapXY([x, y], [refX, refY]) {
		return [
			modulo(x - refX + this.halfPeriodX, this.wrapPeriodX) +
				refX -
				this.halfPeriodX,
			modulo(y - refY + this.halfPeriodY, this.wrapPeriodY) +
				refY -
				this.halfPeriodY,
		];
	}

	/**
	 * @method wrapString(xys: Array of Number): Array of Number
	 * Given a linestring array of the form `[x1,y2, x2,y2, ... xn,yn],
	 * runs `wrap()` on every `x,y` pair. This ensures that the first point of
	 * the linestring is less than half a period away from the CRS' origin, and
	 * idem with each pair of consecutive points.
	 */
	wrapString(xys) {
		const l = xys.length / 2;
		const dest = new Array(l);
		let ref = this.offset || [0, 0];
		for (let i = 0; i < l; i++) {
			const j = i * 2;
			const xy = xys.slice(j, j + 2);
			dest[i] = this.wrap(xy, ref);
			if (Number.isFinite(xy[0]) && Number.isFinite(xy[1])) {
				ref = dest[i];
			}
		}
		return dest.flat();
	}

	/**
	 * @function guessFromCode(crs: String): Promise to BaseCRS
	 *
	 * Factory method. Expects a string like `"EPSG:12345"`.
	 *
	 * Fetches information from https://crs-explorer.proj.org/ and
	 * tries to build a Gleo CRS on a best-effort basis. Registers it via `proj4js`
	 * as well, assuming `enableProj()` has been called.
	 *
	 * The resulting CRS might lack information such as wrap periods or min/max spans.
	 */
	static async guessFromCode(code) {
		try {
			return getCRS(code);
		} catch (ex) {
			const [_, org, number] = /(\w+):(\d+)/.exec(code);

			let wkt;
			try {
				wkt = await (
					await fetch(`https://crs-explorer.proj.org/wkt1/${org}/${number}.txt`)
				).text();
			} catch (ex) {
				wkt = await (
					await fetch(
						`https://spatialreference.org/ref/${org}/${number}/ogcwkt/`
					)
				).text();
			}

			project.defs(code, wkt);

			// Assume that all EPSG CRSs are earth-based, and therefore can rely
			// on distance calculation via reprojection to EPSG:4326 and haversine
			// formula.
			const distance =
				org === "EPSG"
					? (await import("./epsg4326.mjs")).default.distance
					: undefined;

			return new BaseCRS(code, { distance });
		}
	}
}

function modulo(a, n) {
	// As per https://developer.mozilla.org/en-US/docs/Web/JavaScript/Reference/Operators/Remainder
	// return ((a % n ) + n ) % n;

	return a === n ? a : ((a % n) + n) % n;
}
