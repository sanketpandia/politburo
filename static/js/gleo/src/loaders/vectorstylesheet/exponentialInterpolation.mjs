/**
 * Straight from https://github.com/maplibre/maplibre-gl-js/blob/main/src/style-spec/expression/definitions/interpolate.ts
 *
 * Code from Anand Thakker http://anandthakker.net under BSD-3 license.
 */

export default function exponentialInterpolation(input, base, lowerValue, upperValue) {
	const difference = upperValue - lowerValue;
	const progress = input - lowerValue;

	if (difference === 0) {
		return 0;
	} else if (base === 1) {
		return progress / difference;
	} else {
		return (Math.pow(base, progress) - 1) / (Math.pow(base, difference) - 1);
	}
}
