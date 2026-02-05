import RawGeometry from "../geometry/RawGeometry.mjs";

// Parts of this ripped from https://github.com/mapbox/vector-tile-js/blob/master/lib/vectortilefeature.js

/**
 * @namespace Util
 * @function loadProtobufferRawGeometry(crs: BaseCRS, feat: VectorTileFeature, bbox: Array of Number, extent): RawGeometry
 *
 * Alternative way of fetching geometries from protobuffer vectot tile
 * features (from https://github.com/mapbox/vector-tile-js ). The only
 * foreseen usage of this function is as part of `ProtobufVectorTileLoader`.
 *
 * The aim is similar to the `loadGeometry()` method of a `vector-tile-js`'s
 * `VectorTileFeature`, with a few following differences:
 * - Skips array nesting
 * - Returns a Gleo `RawGeometry` instead of an array of arrays of coordinates
 * - Normalizes in-tile coordinates to absolute CRS coordinates, given the
 *   bbox and extent of the tile.
 */

export default function loadProtobufferRawGeometry(crs, feat, bbox, extent) {
	const pbf = feat._pbf;
	pbf.pos = feat._geometry;

	const x1 = bbox[0],
		y1 = bbox[1],
		w = (bbox[2] - bbox[0]) / extent,
		h = (bbox[3] - bbox[1]) / extent;

	let end = pbf.readVarint() + pbf.pos,
		cmd = 1,
		length = 0,
		x = 0,
		y = 0,
		i = 0,
		coords = [],
		rings = [],
		hulls = [],
		lastRingStart = 0,
		winding = true;

	while (pbf.pos < end) {
		if (length <= 0) {
			const cmdLen = pbf.readVarint();
			cmd = cmdLen & 0x7;
			length = cmdLen >> 3;
		}

		length--;

		switch (cmd) {
			case 1:
				if (i && feat.type === 2) {
					rings.push(i);
				}
			case 2:
				x += pbf.readSVarint();
				y += pbf.readSVarint();

				coords.push(x1 + x * w, y1 + y * h);
				i++;
				break;
			case 7:
				coords.push(coords[lastRingStart * 2], coords[lastRingStart * 2 + 1]);
				i++;

				// On the `closePolygon` command,, calculate the signed area -
				// the ring will be an outer or inner ring
				// depending on the sign of its signed area.
				const area = signedArea(coords, lastRingStart, i);
				winding = area < 0;

				if (lastRingStart) {
					if (winding) {
						rings.push(lastRingStart);
					} else {
						hulls.push(lastRingStart);
					}
				}

				lastRingStart = i;
				break;
			default:
				throw new Error("unknown command " + cmd);
		}
	}

	return new RawGeometry(crs, coords, rings, hulls, { wrap: false });
}

// Needed to tell apart outer/inner rings in polygons.
function signedArea(coords, start, end) {
	let sum = 0;
	for (let i = start * 2, end2 = end * 2 - 2; i < end2; i += 2) {
		const p1x = coords[i],
			p1y = coords[i + 1],
			p2x = coords[i + 2],
			p2y = coords[i + 3];
		sum += (p2x - p1x) * (p1y + p2y);
	}

	const p1x = coords[end * 2 - 2],
		p1y = coords[end * 2 - 1],
		p2x = coords[start * 2],
		p2y = coords[start * 2 + 1];

	sum += (p2x - p1x) * (p1y + p2y);
	return sum;
}
