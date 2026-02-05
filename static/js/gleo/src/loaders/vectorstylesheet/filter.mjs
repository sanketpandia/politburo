import parseExpression from "./expression.mjs";

// Aux function for the VectorStylesheetLoader.

// Filter wrapper functions - returns a `function(geom, attrs)` that runs the filter,
// and returns a boolean.
// As per https://docs.mapbox.com/mapbox-gl-js/style-spec/expressions/
export default function getFilterFunc(filter) {
	const op = filter[0];

	if (op === "all" && filter.length === 2) {
		// "all" boolean joiner with just one condition
		return getFilterFunc(filter[1]);
	} else if (op === "any" && filter.length === 2) {
		// "any" boolean joiner with just one condition
		return getFilterFunc(filter[1]);
	}
	if ((op === "all" || op === "any") && filter.length === 1) {
		// "all"/"any" booleans with zero conditions, assume always true.
		return function alwaysTrue(geom, attrs) {
			return true;
		};
	} else if (op === "all" && filter.length > 2) {
		// "all" boolean, run Array.prototype.every
		filter.splice(0, 2);
		const fns = filter.map((f) => getFilterFunc(f));

		return function filterEvery(geom, attrs) {
			return fns.every((f) => f(geom, attrs));
		};
	} else if (op === "any" && filter.length > 2) {
		// "any" boolean, run Array.prototype.some
		filter.splice(0, 2);
		const fns = filter.map((f) => getFilterFunc(f));

		return function filterSome(geom, attrs) {
			return fns.some((f) => f(geom, attrs));
		};
	}

	let attr = filter[1];
	const values = filter.slice(2);

	// The "get" can be found in a filter, too
	if (attr instanceof Array) {
		if (attr[0] === "get") {
			attr = attr[1];
		} else if (attr[0] === "geometry-type") {
			attr = "$type";
		} else {
			throw new Error(`Unsupported expression inside filter: ${attr}`);
		}
	}

	if (op === "!=") {
		return function filterNotEqual(geom, attrs) {
			return attrs[attr] != values[0];
		};
	} else if (op === "==") {
		return function filterNotEqual(geom, attrs) {
			return attrs[attr] == values[0];
		};
	} else if (op === "in") {
		return function filterIn(geom, attrs) {
			return values.includes(attrs[attr]);
		};
	} else if (op === "!in") {
		return function filterIn(geom, attrs) {
			return !values.includes(attrs[attr]);
		};
	} else if (op === ">") {
		return function filterGreaterThan(geom, attrs) {
			return attrs[attr] > values[0];
		};
	} else if (op === "<") {
		return function filterLessThan(geom, attrs) {
			return attrs[attr] < values[0];
		};
	} else if (op === ">=") {
		return function filterGreaterOrEqualThan(geom, attrs) {
			return attrs[attr] >= values[0];
		};
	} else if (op === "<=") {
		return function filterLessOrEqualThan(geom, attrs) {
			return attrs[attr] <= values[0];
		};
	} else if (op === "has") {
		return function filterHas(geom, attrs) {
			return attr in attrs;
		};
	} else if (op === "!has") {
		return function filterNotHas(geom, attrs) {
			return !(attr in attrs);
		};
	} else if (op === "match" && values.length === 3) {
		// Somehow 'match' needs an extra wrapping of the attribute name in an expression.
		const matchable = parseExpression(attr);
		const candidates = values[0];
		const onMatch = values[1];
		const onNoMatch = values[2];
		return function filterMatch(geom, attrs) {
			const val = matchable(attrs);
			return candidates.includes(val)
				? onMatch
					? true
					: false
				: onNoMatch
				? true
				: false;
		};
	} else if (op === "!") {
		return function filterNot(geom, attrs) {
			return !getFilterFunc(attr)(geom, attrs);
		};
	} else {
		console.info("Unimplemented stylesheet filter", filter);
		return undefined;
	}
}
