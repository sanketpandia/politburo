import { default as parseCSSColour } from "../../3rd-party/css-colour-parser.mjs";

// For whatever reason, the mapbox-gl-js stylesheet spec doesn't follow
// CSS colours.
// Specifically, the RGBA declaration assumes that A is between 0 and 1,
// instead of between 0 and 255.

export default function parseColour(c) {
	if (typeof c === "string" && c.substr(0, 4) === "rgba") {
		const tmpColour = parseCSSColour(c);
		const match = c.match(/,\s*([\d\.]+)\s*\)$/);
		tmpColour[3] = match[1] * 255;
		return tmpColour;
	} else {
		return parseCSSColour(c);
	}
}
