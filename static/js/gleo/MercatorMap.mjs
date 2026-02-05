import GleoMap from "./Map.mjs";

import epsg3857 from "./crs/epsg3857.mjs";
import "./actuators/DragActuator.mjs";
import "./actuators/PinchActuator.mjs";
import "./actuators/WheelActuator.mjs";
import "./actuators/InertiaActuator.mjs";
import "./actuators/SpanClampActuator.mjs";
import "./actuators/ZoomYawSnapActuator.mjs";
import "./actuators/BoundsClampActuator.mjs";

import ZoomInOut from "./control/ZoomInOut.mjs";
import ScaleBar from "./control/ScaleBar.mjs";
import Attribution from "./control/Attribution.mjs";

import { setFactory } from "./geometry/DefaultGeometry.mjs";
import LatLng from "./geometry/LatLng.mjs";

/**
 * @class MercatorMap
 * @inherits GleoMap
 *
 * A `GleoMap` with some useful defaults:
 * - Defaults to Web Mercator CRS (`epsg3857`)
 * - Sets a `maxSpan` of 45 million (mainly to prevent users zooming out far
 *   enough to see horizontal bands above 85° / below -85° when zooming), can
 *   be overridden
 * - Enables some actuators by default:
 *   - `DragActuator` to move the map with a drag-and-drop interaction.
 *   - `PinchActuator` to zoom/rotate the map with two-finger gestures.
 *   - `InertiaActuator` to perform animations on view change.
 *   - `WheelActuator` to zoom in/out with a mouse wheel.
 *   - `ZoomYawSnapActuator` to lock onto the "zoom levels" of the map tiles.
 *   - `SpanClampActuator` to prevent the user from zooming too far in or too far out.
 *   - `BoundsClampActuator` to prevent the user from moving too far north or too far south.
 * - Adds some controls to the map:
 *   - `ZoomInOut` buttons
 *   - `ScaleBar`
 *   - `Attribution`
 * - All methods that take `Geometry`s as input can take arrays of the form
 *   `[lat, lng]` instead, as per `LatLng` (via `DefaultGeometry`).
 *
 * In order to have a map without these defaults, use `GleoMap` instead, add
 * CRS, actuators, and controls as desired; and use `DefaultGeometry` functionality
 * to define how to handle geometry inputs.
 *
 * @example
 *
 * ```
 * <div id='gleomap' style='height:500px; width:500px;'></div>
 * <script type='module'>
 * // Import the Gleo files - the paths depend on your importmaps and/or installation
 * import MercatorMap from 'gleo/src/MercatorMap.mjs';
 * import MercatorTiles from 'gleo/src/loaders/MercatorTiles.mjs';
 *
 * // Instantiate the MercatorMap instance, given the ID of the <div>
 * const myGleoMap = new MercatorMap('gleomap');
 *
 * // Load some default OpenStreetMap tiles in the map
 * new MercatorTiles("https://tile.osm.org/{z}/{y}/{x}.png", {maxZoom: 10}).addTo(myGleoMap);
 *
 * // The map center can be specified as a plain array in [latitude, longitude] form
 * myGleoMap.center = [40, -3];
 *
 * // The map span is the length of the map's diagonal in "meters"
 * myGleoMap.span = 1000000;
 * </script>
 * ```
 */

export default class MercatorMap extends GleoMap {
	/**
	 * @constructor MercatorMap(div: HTMLDivElement, options: GleoMap Options)
	 * @alternative
	 * @constructor MercatorMap(divID: string, options: GleoMap Options)
	 */
	constructor(container, { ...options } = {}) {
		super(container, { crs: epsg3857, ...options });

		new ZoomInOut().addTo(this);
		new ScaleBar().addTo(this);
		new Attribution().addTo(this);
	}
}

setFactory(function latLngize(coords, opts) {
	return new LatLng(coords, opts);
});
