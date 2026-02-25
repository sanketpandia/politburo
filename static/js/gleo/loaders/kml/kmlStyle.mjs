import Sprite from "../../symbols/Sprite.mjs";
import Stroke from "../../symbols/Stroke.mjs";
import Fill from "../../symbols/Fill.mjs";

// KML colour definitions are "aabbggrr", opposite to CSS hex colour defs ("#rrggbbaa")
// This function works similar to parseCSSColour.
function parseKMLColour(str) {
	const iv = parseInt(str, 16);
	if (!(iv >= 0 && iv <= 0xffffffff)) return null; // Covers NaN.
	return [
		iv & 0x000000ff,
		(iv & 0x0000ff00) >> 8,
		(iv & 0x00ff0000) >> 16,
		((iv & 0xff000000) >> 24) & 0xff,
	];
}

// Returns an object o the form `{ point: [], line: [], polygon: [] }`,
// containing symbolizer functions appropriate for points/linestrings/polygons.
// Parameters are one `<Style>` node from a KML document (either from a named
// style, or inlined in a `<Placemark>`), and the URL of the KML document
// (needed for the relative URLs of icons).
export default function parseKMLStyle(node, baseUrl) {
	let symbolizers = { point: [], line: [], polygon: [] };

	let iconStyle = node.querySelector("IconStyle");
	if (iconStyle) {
		const imageHref = new URL(iconStyle.querySelector("href").textContent, baseUrl);
		let anchor = ["50%", "50%"];

		const hotspot = iconStyle.querySelector("hotSpot");
		if (hotspot) {
			if ("x" in hotspot.attributes) {
				const x = Number(hotspot.attributes.x.value);
				const xunits = hotspot.attributes.xunits.value;
				if (xunits === "fraction" || !("xunits" in hotspot.attributes)) {
					anchor[0] = x * 100 + "%";
				} else if (xunits === "insetPixels") {
					anchor[0] = -x;
				} else if (xunits === "pixels") {
					anchor[0] = x;
				} else {
					// This case includes the "insetPixels" mode,
					// which is pixels counted from the top-right
					throw new Error("Unimplemented unit for KML icon hotspot");
				}
			}
			if ("y" in hotspot.attributes) {
				const y = Number(hotspot.attributes.y.value);
				const yunits = hotspot.attributes.yunits.value;
				if (yunits === "fraction" || !("yunits" in hotspot.attributes)) {
					anchor[1] = y * 100 + "%";
				} else if (yunits === "insetPixels") {
					anchor[1] = y;
				} else if (yunits === "pixels") {
					anchor[1] = -y;
				} else {
					throw new Error("Unimplemented unit for KML icon hotspot");
				}
			}
		}

		symbolizers.point.push(function kmlIconStyle(geom) {
			return new Sprite(geom, {
				image: imageHref,
				spriteAnchor: anchor,
			});
		});
	}

	let lineStyle = node.querySelector("LineStyle");
	if (lineStyle) {
		let colour = lineStyle.querySelector("color")?.textContent;

		if (colour) {
			colour = parseKMLColour(colour);
		} else {
			colour = [255, 255, 255, 255];
		}

		let width = Number(lineStyle.querySelector("width")?.textContent ?? 1);

		function kmlLineStyle(geom) {
			return new Stroke(geom, { colour, width });
		}
		symbolizers.line.push(kmlLineStyle);
		if (node.querySelector("PolyStyle > outline")?.textContent !== "0") {
			symbolizers.polygon.push(kmlLineStyle);
		}
	}

	let polyStyle = node.querySelector("PolyStyle");
	if (polyStyle) {
		let colour = polyStyle.querySelector("color")?.textContent;
		if (colour) {
			colour = parseKMLColour(colour);
		} else {
			colour = [255, 255, 255, 255];
		}

		function kmlFillStyle(geom) {
			return new Fill(geom, { colour });
		}

		if (polyStyle.querySelector("fill")?.textContent !== "0") {
			symbolizers.polygon.push(kmlFillStyle);
		}

		/// TODO: If there's an "outline" element with a value of
		/// zero, there should be no stroke associated.
	}

	let labelStyle = node.querySelector("LabelStyle");
	if (labelStyle) {
		/// TODO!!!
	}

	return symbolizers;
}
