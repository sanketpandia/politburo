import Loader from "./Loader.mjs";
import Stroke from "../symbols/Stroke.mjs";
import CircleStroke from "../symbols/CircleStroke.mjs";
import CircleFill from "../symbols/CircleFill.mjs";
import LngLat from "../geometry/LngLat.mjs";

function defaultWaypointSymbolizer(waypoint, geometry) {
	return [new CircleStroke(geometry), new CircleFill(geometry)];
}

function defaultTrackSymbolizer(track, geometry) {
	return [new Stroke(geometry)];
}
function defaultRouteSymbolizer(route, geometry) {
	return [new Stroke(geometry)];
}

const XMLparser = new window.DOMParser();

/**
 * @class GPX
 * @inherits Loader
 * @relationship compositionOf GleoSymbol, 0..1, 0..n
 *
 * A `Loader` for requesting, parsing and symbolizing data in [GPS Exchange Format](https://en.wikipedia.org/wiki/GPS_Exchange_Format) (AKA "GPX").
 */

/// TODO: Handle some of the metadata. "Author" and "Copyright" fields can be
/// interesting for attribution.

export default class GPX extends Loader {
	#symbols = [];

	/**
	 * @constructor GPX(gpx: XMLDocument, options: GPX Options)
	 * Symbolizes the data in the given XML document structure. The XML must
	 * be conformant to the GPX schema.
	 * @alternative
	 * @constructor GPX(url: URL, options: GPX Options)
	 * If given a URL object, that URL will be requested, and the returned
	 * GPX will be parsed.
	 * @alternative
	 * @constructor GPX(url: String, options: GPX Options)
	 * When given a `String`, it will be trated as an `URL`.
	 */
	constructor(
		gpx,
		{
			/**
			 * @section GPX Options
			 * @option waypointSymbolizer: Function = *
			 * A `Function` that defines how waypoints get transformed into
			 * `GleoSymbol`s. It receives the waypoint (as a parsed XML node),
			 * an instance of a Gleo `Geometry` as its second parameter, and
			 * must return an array of `GleoSymbol`s.
			 *
			 * The parsed XML node is an [`Element`](https://developer.mozilla.org/docs/Web/API/Element), and its schema is specified at [https://www.topografix.com/GPX/1/1/#type_wptType](https://www.topografix.com/GPX/1/1/#type_wptType)
			 *
			 * When not specified, a default implementation is used. This default
			 * implementation symbolizes waypoints with a `CircleFill` and
			 * a `CircleStroke` with default options.
			 *
			 * @option trackSymbolizer: Function = *
			 * Akin to `pointSymbolizer`, but for GPX tracks.
			 *
			 * When not specified, the default is to use a default `Stroke`.
			 * @option routeSymbolizer: Function = *
			 * Akin to `pointSymbolizer`, but for GPX routes.
			 *
			 * When not specified, the default is to use a default `Stroke`.
			 */
			waypointSymbolizer = defaultWaypointSymbolizer,
			trackSymbolizer = defaultTrackSymbolizer,
			routeSymbolizer = defaultRouteSymbolizer,
			...opts
		} = {}
	) {
		super(opts);
		/**
		 * @property waypointSymbolizer: Function
		 * The `Function` currently used for turning a waypoint into a set of
		 * `GleoSymbol`s.
		 *
		 * Changing this function during runtime does **not** trigger a
		 * re-symbolization of the dataset.
		 * @property trackSymbolizer: Function
		 * Akin to `waypointSymbolizer`, but for GPX tracks.
		 * @property routeSymbolizer: Function
		 * Akin to `waypointSymbolizer, bt for GPX routes.
		 */
		this.waypointSymbolizer = waypointSymbolizer;
		this.trackSymbolizer = trackSymbolizer;
		this.routeSymbolizer = routeSymbolizer;

		if (gpx instanceof XMLDocument) {
			// Assuming a well-formed GeoJSON data structure was received
			this.#symbols = this._symbolizeGPX(gpx);
		} else {
			// Assuming URL, or url-like string
			let url = gpx instanceof URL ? gpx : new URL(gpx, document.URL);

			fetch(url)
				.then((response) => response.text())
				.then((text) => XMLparser.parseFromString(text, "text/xml"))
				.then((xml) => {
					this.#symbols = this._symbolizeGPX(xml);
					this.platina?.multiAdd(this.#symbols);
				});
		}
	}

	addTo(target) {
		super.addTo(target);
		return this;
	}

	remove() {
		this.platina?.multiRemove(this.#symbols);
		return super.remove();
	}

	_symbolizeGPX(gpxDoc) {
		const waypointSymbols = Array.from(gpxDoc.querySelectorAll("gpx > wpt"))
			.map((wpt) => {
				const geom = new LngLat([
					Number(wpt.attributes.lon.value),
					Number(wpt.attributes.lat.value),
				]);
				return this.waypointSymbolizer(wpt, geom);
			})
			.flat();

		const tracksSymbols = Array.from(gpxDoc.querySelectorAll("gpx > trk"))
			.map((trk) =>
				this.trackSymbolizer(
					trk,
					new LngLat(
						Array.from(trk.querySelectorAll("trkseg"))
							.map((trkseg) =>
								Array.from(trkseg.querySelectorAll("trkpt")).map(
									(trkpt) => [
										Number(trkpt.attributes.lon.value),
										Number(trkpt.attributes.lat.value),
									]
								)
							)
							.filter((seg) => seg.length)
					)
				)
			)
			.flat();

		const routesSymbols = Array.from(gpxDoc.querySelectorAll("gpx > rte"))
			.map((rte) =>
				this.routeSymbolizer(
					rte,
					new LngLat(
						Array.from(rte.querySelectorAll("rtept")).map((rtept) => [
							Number(rtept.attributes.lon.value),
							Number(rtept.attributes.lat.value),
						])
					)
				)
			)
			.flat();

		return [waypointSymbols, tracksSymbols, routesSymbols].flat();
	}
}
