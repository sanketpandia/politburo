// import parseColour from "../../3rd-party/css-colour-parser.mjs";
import parseColour from "./parseColour.mjs";
import exponentialInterpolation from "./exponentialInterpolation.mjs";

// Aux function for the VectorStylesheetLoader.

// Given an object/array with a mapbox/maplibre stylesheet expression,
// returns *either* a constant value, or a function that takes
// feature properties.

// The idea being that every expression **and subexpression** can be mapped
// to a function.

// See https://docs.mapbox.com/mapbox-gl-js/style-spec/expressions/

export default function parseExpression(exp, isColour = false) {
	if (typeof exp === "number" || typeof exp === "string") {
		return getConstant(exp, isColour);
	} else if (exp instanceof Array) {
		// Expression is an array

		if (exp.every((n) => typeof n === "number")) {
			return getConstant(exp);
		}

		if (exp[0] === "literal") {
			return parseExpression(exp[1], isColour);
		}
		if (exp[0] === "match") {
			return getMatcher(exp, isColour);
		}
		if (exp[0] === "get") {
			return getGetterFunction(exp, isColour);
		}
		if (exp[0] === "interpolate") {
			return getInterpolateFunction(exp, isColour);
		}
		if (exp[0] === "step") {
			return getSteps(exp, isColour);
		}
	} else {
		// Expression is an object

		if (exp.stops) {
			return getStopsInterpolator(exp, isColour);
		}
	}

	throw new Error("Could not parse vector stylesheet expression", exp);
}

/**
 * Parses constant expressions
 */
function getConstant(exp, isColour) {
	if (isColour) {
		return getConstant(parseColour(exp));
	}

	const fn = function constant() {
		return exp;
	};
	fn.constant = exp;
	return fn;
}

/**
 * Parses "match" expressions
 */
function getMatcher(matchData, isColour) {
	if (matchData[0] !== "match") {
		throw new Error("getMatcher needs a `match` expression structure");
	}

	if (matchData[1][0] !== "get") {
		throw new Error("getMatcher cannot handle this `match` expression structure");
		// Ideally, this should parse the "get" expression instead.
		// The current implementation provides a fast path to the most
		// usual case.
	}

	let matchMap = {};
	const attrName = matchData[1][1];
	const l = matchData.length;

	for (let i = 2; i < l - 2; i += 2) {
		const value = parseExpression(matchData[i + 1], isColour);
		if (matchData[i] instanceof Array) {
			matchData[i].forEach((m) => (matchMap[m] = value));
		} else {
			matchMap[matchData[i]] = value;
		}
	}
	const defaultValue = parseExpression(matchData[l - 1], isColour);

	/// TODO: Fast-path alternative when all possible matches are
	/// **not** a function.

	//console.log(matchMap);
	return function matcher(attrs) {
		const match = matchMap[attrs[attrName]];
		return match?.(attrs) ?? defaultValue(attrs);
	};
}

/**
 * Parses (deprecated) "stops" expressions
 */
function getStopsInterpolator(exp, isColour) {
	const attrName = exp.property ?? "$zoom";
	const table = exp.stops.map((s) => [s[0], parseExpression(s[1], isColour)]);
	const l = table.length;

	/// TODO: Use base currently this is always linear interpolation.
	//const base = exp.base ?? 1;	// Base of exponential interpolation

	return function stopsInterpolator(attrs) {
		const value = attrs[attrName];
		if (value <= table[0][0]) {
			return table[0][1](attrs);
		}

		for (let i = 1; i < l; i++) {
			if (value <= table[i][0]) {
				const range = table[i][0] - table[i - 1][0];
				const minVal = table[i - 1][1](attrs);
				const maxVal = table[i][1](attrs);
				const percentage = (value - table[i - 1][0]) / range;
				if (minVal instanceof Array) {
					return minVal.map(
						(_, j) => minVal[j] * (1 - percentage) + maxVal[j] * percentage
					);
				} else {
					return minVal * (1 - percentage) + maxVal * percentage;
				}
			}
		}
		return table[l - 1][1](attrs);
	};

	/// TODO: fast path functions for constant values
}

function getInterpolateFunction(exp, isColour) {
	if (exp[0] !== "interpolate") {
		throw new Error("Malformed interpolate expression");
	} else if (exp[2].length !== 1) {
		throw new Error("Unsupported interpolate expression");
	}

	let attr = exp[2][0];
	if (attr === "zoom") {
		/// FIXME: Find where the 'zoom' and '$zoom' fields are documented.
		attr = "$zoom";
	}

	const rawTable = exp.slice(3);
	//const defaultValue = parseExpression(exp[exp.length-1], isColour);
	let l = rawTable.length;
	const table = [];
	for (let i = 0; i < l; i += 2) {
		table.push([rawTable[i], parseExpression(rawTable[i + 1], isColour)]);
	}
	l = table.length;

	if (exp[1][0] === "linear") {
		return function linearInterpolator(attrs) {
			const value = attrs[attr];
			if (value <= table[0][0]) {
				return table[0][1](attrs);
			} else if (value >= table[l - 1][0]) {
				return table[l - 1][1](attrs);
			} else {
				for (let i = 1; i < l; i++) {
					if (value <= table[i][0]) {
						const range = table[i][0] - table[i - 1][0];
						const minVal = table[i - 1][1](attrs);
						const maxVal = table[i][1](attrs);
						const percentage = (value - table[i - 1][0]) / range;
						if (minVal instanceof Array) {
							return minVal.map(
								(_, j) =>
									minVal[j] * (1 - percentage) + maxVal[j] * percentage
							);
						} else {
							return minVal * (1 - percentage) + maxVal * percentage;
						}
					}
				}
			}
		};
	} else if (exp[1][0] === "exponential") {
		const base = exp[1][1];

		return function exponentialInterpolator(attrs) {
			const value = attrs[attr];
			if (value <= table[0][0]) {
				return table[0][1](attrs);
			} else if (value >= table[l - 1][0]) {
				return table[l - 1][1](attrs);
			} else {
				for (let i = 1; i < l; i++) {
					if (value <= table[i][0]) {
						const percentage = exponentialInterpolation(
							value,
							base,
							table[i - 1][0],
							table[i][0]
						);
						const minVal = table[i - 1][1](attrs);
						const maxVal = table[i][1](attrs);
						if (minVal instanceof Array) {
							return minVal.map(
								(_, j) =>
									minVal[j] * (1 - percentage) + maxVal[j] * percentage
							);
						} else {
							return minVal * (1 - percentage) + maxVal * percentage;
						}
					}
				}
			}
			return defaultValue(attrs);
		};
	} else {
		throw new Error("Unknown/unsupported interpolation type in stylesheet");
	}
}

/**
 * Parses "get" expressions
 */
function getGetterFunction(exp, isColour) {
	if (exp[0] !== "get") {
		throw new Error('Bad "get" expression');
	}

	const attrName = exp[1];

	if (isColour) {
		return function getColourAttrib(attrs) {
			return parseColour(attrs[attrName]);
		};
	} else {
		return function getAttrib(attrs) {
			return attrs[attrName];
		};
	}
}

/**
 * Parses "step" expressions
 * This expression is assumed to have the step values in strictly ascending order
 */
function getSteps(exp, isColour) {
	if (exp[0] !== "step") {
		throw new Error("Malformed interpolate expression");
	}

	let attr = exp[1];
	if (attr === "zoom") {
		/// FIXME: Find where the 'zoom' and '$zoom' fields are documented.
		attr = "$zoom";
	}

	const rawTable = exp.slice(2);
	let l = rawTable.length - 1;
	const table = [];
	for (let i = 0; i < l; i += 2) {
		table.push([rawTable[i], parseExpression(rawTable[i + 1], isColour)]);
	}
	l = table.length;

	return function step(attrs) {
		const value = attrs[attr];

		if (value < table[0][0]) {
			return table[0][1](attrs);
		}
		for (let i = 1; i < l; i++) {
			if (value >= table[i][0]) {
				return table[i][1](attrs);
			}
		}
	};
}
