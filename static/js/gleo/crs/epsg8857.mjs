import BaseCRS from "./BaseCRS.mjs";
import { registerProjectionFunction } from "./projector.mjs";
import epsg4326 from "./epsg4326.mjs";

const halfπ = Math.PI / 2;

/**
 * @namespace epsg8857
 * @inherits BaseCRS
 *
 * A EPSG:8857 CRS - aka "Equal Earth" (over WGS84).
 *
 */

const epsg8857 = new BaseCRS("EPSG:8857", {
	distance: epsg4326,
	ogcUri: "http://www.opengis.net/def/crs/EPSG/0/8857",
	minSpan: 0.1,
	maxSpan: 8,
	viewableBounds: [-Math.PI, -halfπ, Math.PI, halfπ],
});

export default epsg8857;

registerProjectionFunction("EPSG:4326", "EPSG:8857", lnglat2equalearth);
registerProjectionFunction("EPSG:8857", "EPSG:4326", equalearth2lnglat);

// The following implementation of equal earth projection is ripped off d3-geo,
// specifically https://github.com/d3/d3-geo/blob/main/src/projection/equalEarth.js
// by φlippe Rivière (under MIT license) based on Bojan Šavrič _et al._

const A1 = 1.340264,
	A2 = -0.081106,
	A3 = 0.000893,
	A4 = 0.003796,
	M = Math.sqrt(3) / 2,
	iterations = 12,
	ε = 1e-12,
	rad = 180 / Math.PI, // Degrees in a radian (i.e. ~57)
	deg = Math.PI / 180;

function asin(x) {
	return x > 1 ? halfπ : x < -1 ? -halfπ : Math.asin(x);
}

function lnglat2equalearth([λ, φ]) {
	var l = asin(M * Math.sin(φ * deg)),
		l2 = l * l,
		l6 = l2 * l2 * l2;
	return [
		(λ * deg * Math.cos(l)) / (M * (A1 + 3 * A2 * l2 + l6 * (7 * A3 + 9 * A4 * l2))),
		l * (A1 + A2 * l2 + l6 * (A3 + A4 * l2)),
	];
}

function equalearth2lnglat([x, y]) {
	var l = y,
		l2 = l * l,
		l6 = l2 * l2 * l2;
	for (var i = 0, Δ, fy, fpy; i < iterations; ++i) {
		fy = l * (A1 + A2 * l2 + l6 * (A3 + A4 * l2)) - y;
		fpy = A1 + 3 * A2 * l2 + l6 * (7 * A3 + 9 * A4 * l2);
		(l -= Δ = fy / fpy), (l2 = l * l), (l6 = l2 * l2 * l2);
		if (Math.abs(Δ) < ε) break;
	}
	return [
		(rad * M * x * (A1 + 3 * A2 * l2 + l6 * (7 * A3 + 9 * A4 * l2))) / Math.cos(l),
		rad * asin(Math.sin(l) / M),
	];
}
