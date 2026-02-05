/**
 * Performs a segment intersection with modulo: intersects the segment a1-a2
 * with all occurences of segment b modulo m (for all i in Z, segment b1+im to b2+im)
 *
 * Assumes a2>a1, b2>b1, and either m>0 or m=Infinity
 */
export default function intersectSegments(a1, a2, b1, b2, m) {
	if (b2 - b1 >= m) {
		// Edge case: the b segment is larger than the modulo therefore
		// all its occurences spans the whole R, therefore the intersection is
		// the identity function.
		return [[a1, a2]];
	}

	let minShift, maxShift;
	let modulo;
	if (isFinite(m)) {
		// How many times do we need to sum the modulo so that the end of the
		// b segment overlaps a?
		minShift = -Math.floor((b2 - a1) / m);
		maxShift = Math.floor((a2 - b1) / m);
		modulo = m;
	} else {
		minShift = maxShift = 0;
		modulo = 0;
	}

	if (b1 + minShift * m > a2) {
		// The segments do not intersect
		return [];
	}

	const intersections = [];
	for (let shift = minShift; shift <= maxShift; shift++) {
		const offset = shift * modulo;
		intersections.push([Math.max(a1, b1 + offset), Math.min(a2, b2 + offset)]);
	}
	return intersections;
}
