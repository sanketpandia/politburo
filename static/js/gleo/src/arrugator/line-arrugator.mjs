import TinyQueue from "./3rd-party/tinyqueue.mjs";

// A variant of Arrugator that works with segment strings (AKA "lines") instead
// of triangle meshes.
export default class LineArrugator {
	// #projector;
	// #verts;

	constructor(projector, verts) {
		// The projector function. Must be able to take
		// an array of two numbers [x,y] and return an array of
		// two numbers.
		// The typical use case is a proj4(from,to).forward function.
		this._projector = projector;

		// A two-dimensional array of vertex coordinates. Each vertex is a
		// two-element [x,y] array.
		this._verts = verts;

		// A two-dimensional array of vertex coordinates, projected. Each
		// vertex is a two-element [x,y] array.
		this._projVerts = verts.map(projector);

		// A priority queue of segments, ordered by their epsilons, in descending order.
		this._queue = new TinyQueue([], function (a, b) {
			return b.epsilon - a.epsilon;
		});

		for (let i=0, l=this._verts.length - 1; i<l; i++) {
			this._calcSegment(i, i+1);
		}

		// Keeps the indices of the vertices, in connectivity order
		this._order = Array.from({length: this._verts.length}, (v,i)=>i);

	}

	// Calculates data for a segment and pushes it to the priority queue.
	_calcSegment(v1, v2) {
		const midpoint = [
			(this._verts[v1][0] + this._verts[v2][0]) / 2,
			(this._verts[v1][1] + this._verts[v2][1]) / 2,
		];
		const projectedMid = this._projector(midpoint);
		const midProjected = [
			(this._projVerts[v1][0] + this._projVerts[v2][0]) / 2,
			(this._projVerts[v1][1] + this._projVerts[v2][1]) / 2,
		];

		const epsilon =
			(projectedMid[0] - midProjected[0]) ** 2 +
			(projectedMid[1] - midProjected[1]) ** 2;

		this._queue.push({
			v1: v1,
			v2: v2,
			epsilon: epsilon,
			midpoint: midpoint,
			projectedMid: projectedMid,
		});

	}


	step() {
		const top = this._queue.pop();

		const v1 = top.v1;
		const v2 = top.v2;

		const vm = this._verts.length;
		this._verts[vm] = top.midpoint;
		this._projVerts[vm] = top.projectedMid;
		this._order.splice(this._order.indexOf(v2), 0, vm)

		this._calcSegment(v1, vm);
		this._calcSegment(vm, v2);
	}


	// Outputs a copy of the coordinates for the linestring.
	output() {

		return this._order.map(i=>this._projVerts[i]);
	}

	// Subdivides the mesh until the maximum segment epsilon is below the
	// given threshold.
	// The `targetEpsilon` parameter must be in the same units as the
	// internal epsilons: units of the projected CRS, **squared**.
	lowerEpsilon(targetEpsilon) {
		while (this._queue.peek().epsilon > targetEpsilon) {
			this.step();
		}
	}

	get epsilon() {
		return this._queue.peek().epsilon;
	}

	set epsilon(ep) {
		return this.lowerEpsilon(ep);
	}

}

