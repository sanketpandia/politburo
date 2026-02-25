/**
 * @namespace knownCRSs
 * @relationship aggregationOf BaseCRS, 1..1, 0..n
 *
 * Holds a list of `BaseCRS`s globally known to Gleo, keyed by their OGC URI.
 */

const knownCRSs = new Map();

// This is a super-simplistic CURIE parser that doesn't take into account
// the full spec (the full list of valid characters), instead parses only
// the brackets, double colon and non-whitespace. See https://www.w3.org/TR/curie/ .
const curieRegexp = /^\[(\S+):(\S+)\]$/;

/**
 * @function getCRS(ogcUri: String): BaseCRS
 * Returns the known `BaseCRS` with the given OGC URI. Accepts the URI in CURIE
 * form (e.g. `[EPSG:4326]`) as well. Throws an error if not found.
 * @alternative
 * @function getCRS(name: String): BaseCRS
 * Returns the known `BaseCRS` with the given internal name (e.g. "EPSG:4326",
 * "cartesian"). Throws an error if not found.
 */
export function getCRS(ogcUri) {
	const match = curieRegexp.exec(ogcUri);
	let uri = ogcUri;
	if (match) {
		const [_, authority, code] = match;
		uri = `http://www.opengis.net/def/crs/${authority}/0/${code}`;
	}
	const crs = knownCRSs.get(uri);
	if (!crs) {
		throw new Error(
			"There is no known Gleo CRS for the given name/OGC URI: " + ogcUri
		);
	}
	return crs;
}

/**
 * @function registerCRS(crs: BaseCRS, overrideUri: undefined): undefined
 * Registers the given CRS. Meant for internal use only. Trying to register
 * a CRS twice (or two CRSs with the same OGC URI) will throw an error.
 * @alternative
 * @function registerCRS(crs: BaseCRS, overrideUri: String): undefined
 * Registers the given CRS, but with an alternate OGC URI. Meant for internal
 * use only, for pointing both `EPSG:4326` and `OGC:WGS84` to the same object.
 */
export function registerCRS(crs, overrideUri) {
	const uri = overrideUri ?? crs.ogcUri;
	if (uri) {
		if (knownCRSs.has(uri)) {
			throw new Error("CRS has been defined twice for the given OGC URL: " + uri);
		}
		knownCRSs.set(uri, crs);
	}
	knownCRSs.set(crs.name, crs);
}
