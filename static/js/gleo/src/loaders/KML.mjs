import Loader from "./Loader.mjs";
// import CircleStroke from "../symbols/CircleStroke.mjs";
// import CircleFill from "../symbols/CircleFill.mjs";
import LngLat from "../geometry/LngLat.mjs";
import ExpandBox from "../geometry/ExpandBox.mjs";

import parseKMLStyle from "./kml/kmlStyle.mjs";

const XMLparser = new window.DOMParser();

// Parses a string containing space-delimited triplets of comma-delimited
// numerical coordinates. Drops the altitude during `slice(0,2)`.
// Returns an array of arrays.
function parseKMLcoordinates(coordStr) {
	return coordStr
		.trim()
		.split(/\s+/)
		.map((pointString) => pointString.split(",").slice(0, 2).map(Number));
}

/**
 * @class KML
 * @inherits Loader
 * @relationship compositionOf GleoSymbol, 0..1, 0..n
 *
 * A `Loader` for requesting, parsing and symbolizing data in [Keyhole Markup Language](https://en.wikipedia.org/wiki/Keyhole_Markup_Language) (AKA "KML").
 */

export default class KML extends Loader {
	#symbols = [];
	#url = document.url; // URL of the KML document, if any.
	#bbox;

	/**
	 * @constructor KML(gpx: XMLDocument, options: GPX Options)
	 * Symbolizes the data in the given XML document structure. The XML must
	 * be conformant to the KML schema.
	 * @alternative
	 * @constructor KML(blob: Blob, options: KML Options)
	 * If given a `Blob` (which also includes `File`s), it will be parsed as KML.
	 * @alternative
	 * @constructor KML(url: URL, options: KML Options)
	 * If given a URL object, that URL will be requested, and the returned
	 * KML will be parsed.
	 * @alternative
	 * @constructor KML(url: String, options: KML Options)
	 * When given a `String`, it will be trated as an `URL`.
	 */
	constructor(
		kml,
		{
			/**
			 * @section KML Options
			 */
			...opts
		} = {}
	) {
		super(opts);

		if (kml instanceof XMLDocument) {
			// Assuming a well-formed GeoJSON data structure was received
			this.#syncSymbolizeKml(kml);
		} else if (kml instanceof Blob) {
			kml.text()
				.then((text) => XMLparser.parseFromString(text, "text/xml"))
				.then(this.#syncSymbolizeKml.bind(this));
		} else {
			// Assuming URL, or url-like string
			this.#url = kml instanceof URL ? kml : new URL(kml, document.URL);

			fetch(this.#url)
				.then((response) => response.text())
				.then((text) => XMLparser.parseFromString(text, "text/xml"))
				.then(this.#syncSymbolizeKml.bind(this));
		}
	}

	// As constructor, but parameter MUST be a parsed XML document.
	#syncSymbolizeKml(xml) {
		this.#symbols = this._symbolizeKML(xml);
		if (this.platina) {
			this.platina.multiAdd(this.#symbols);
		}
		/**
		 * @event load
		 * Fired when the KML data has been fully loaded ans symbolized.
		 */
		this.fire("load");
	}

	addTo(target) {
		super.addTo(target);
		this.platina.multiAdd(this.#symbols);
		return this;
	}

	remove() {
		this.platina.multiRemove(this.#symbols);
		return super.remove();
	}

	/**
	 * @property bbox: ExpandBox
	 * The minimal bounding box that contains all geometries from all loaded
	 * symbols.
	 * The units of this bounding box correspond to the CRS of the `Platina`
	 * this loader is in.
	 * @alternative
	 * @property bbox: undefined
	 * When this loader is not in any `Platina`, or there are no loaded symbols,
	 * the bounding box of its loaded symbols is undefined.
	 */
	get bbox() {
		const crs = this.platina?.crs;
		if (!crs || this.#symbols.length === 0) {
			return undefined;
		}
		if (this.#bbox) {
			return this.#bbox;
		}

		this.#bbox = new ExpandBox();

		this.#symbols.forEach((s) => {
			const coordData = s.geom.toCRS(crs).coords;
			for (let i = 0, l = coordData.length; i < l; i += 2) {
				if (Number.isFinite(coordData[i]) && Number.isFinite(coordData[i + 1])) {
					this.#bbox.expandXY(coordData[i], coordData[i + 1]);
				}
			}
		});
		return this.#bbox;
	}

	_symbolizeKML(kmlDoc) {
		// 1: parse styles
		// 2: parse stylemaps (and copy normal style to stylemap ref)
		// 3: parse placemarks

		const styles = new Map(
			Array.from(kmlDoc.querySelectorAll("Document > Style")).map((styleNode) => {
				let symbolizers = parseKMLStyle(styleNode, this.#url);

				return [styleNode.id, symbolizers];
			})
		);

		// Link stylemaps
		// A <stylemap> contains a pair of style references: "normal" and "highlight".
		// All three thingshave an ID (or ID reference) - Gleo will link stuff
		// so that the stylemap ID holds the same information as the "normal"
		// style ID ref.
		Array.from(kmlDoc.querySelectorAll("Document > StyleMap")).forEach((stylemap) => {
			const stylemapId = stylemap.attributes.id.value;
			stylemap.querySelectorAll("Pair").forEach((pair) => {
				if (pair.querySelector("key").textContent === "normal") {
					const normalStyleId = pair
						.querySelector("styleUrl")
						.textContent.replace(/^#/, "");
					styles.set(stylemapId, styles.get(normalStyleId));
				}
			});
		});

		// There can be `Document > Placemark` and `Document > Folder > Placemark`.
		// This Gleo loader will just add all of the placemarks, regardless of
		// whether they're in folders or not.
		// const placemarks = kmlDoc.querySelectorAll("Document > Placemark");
		const placemarks = kmlDoc.querySelectorAll("Document Placemark");

		const symbols = Array.from(placemarks).map((placemark) => {
			// This handles ONLY style URLs. Supposedly this should be able
			// to handle inline styles as well (by splitting the style parsing
			// functionality off)
			const styleRef = placemark
				.querySelector("styleUrl")
				?.textContent.replace(/^#/, "");
			const inlineStyleNode = placemark.querySelector("Style");
			const symbolizers = styleRef
				? styles.get(styleRef)
				: inlineStyleNode
				? parseKMLStyle(inlineStyleNode, this.#url)
				: { point: [], line: [], polygon: [] };

			// Most placemarks are a single geometry, but there are MultGeometries
			// also - so any number of geometries of any dimension are possible.
			const gleoPointGeoms = [];
			const gleoLineGeoms = [];
			const gleoPolygonGeoms = [];

			const pointGeoms = placemark.querySelectorAll("Point");
			pointGeoms.forEach((pointGeom) => {
				const coords = pointGeom.querySelector("coordinates").textContent;
				const [lng, lat /*, alt*/] = coords.split(",").map(Number);
				gleoPointGeoms.push(new LngLat([lng, lat]));
			});

			const linestringGeoms = placemark.querySelectorAll("LineString");
			linestringGeoms.forEach((linestringGeom) => {
				const coords = linestringGeom.querySelector("coordinates").textContent;
				const points = parseKMLcoordinates(coords);

				gleoLineGeoms.push(new LngLat(points));
			});

			const polygonGeoms = placemark.querySelectorAll("Polygon");
			polygonGeoms.forEach((polygonGeom) => {
				const outerRing = parseKMLcoordinates(
					polygonGeom.querySelector(
						"outerBoundaryIs > LinearRing > coordinates"
					).textContent
				);

				const innerRings = Array.from(
					polygonGeom.querySelectorAll(
						"innerBoundaryIs > LinearRing > coordinates"
					)
				).map((el) => parseKMLcoordinates(el.textContent));

				gleoPolygonGeoms.push(new LngLat([outerRing, ...innerRings]));
			});

			if (
				gleoPointGeoms.length === 0 &&
				gleoLineGeoms.length === 0 &&
				gleoPolygonGeoms.length === 0
			) {
				console.warn("KML placemark has invalid/unrecognized geometry");
			}

			if (symbolizers) {
				return [
					symbolizers.point.map((s) => gleoPointGeoms.map((g) => s(g))),
					symbolizers.line.map((s) => gleoLineGeoms.map((g) => s(g))),
					symbolizers.polygon.map((s) => gleoPolygonGeoms.map((g) => s(g))),
				].flat(2);
			} else {
				debugger;
			}
		});

		return symbols.flat();
	}
}
