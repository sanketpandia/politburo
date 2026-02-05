import GleoMap from "./Map.mjs";

import Geometry from "./geometry/Geometry.mjs";
import cartesian from "./crs/cartesian.mjs";
import "./actuators/DragActuator.mjs";
import "./actuators/PinchActuator.mjs";
import "./actuators/WheelActuator.mjs";
import "./actuators/InertiaActuator.mjs";
import "./actuators/SpanClampActuator.mjs";
import "./actuators/ZoomYawSnapActuator.mjs";
import "./actuators/BoundsClampActuator.mjs";

import ZoomInOut from "./control/ZoomInOut.mjs";
//import ScaleBar from "./control/ScaleBar.mjs";
import Attribution from "./control/Attribution.mjs";

import { setFactory } from "./geometry/DefaultGeometry.mjs";

/**
 * @class CartesianMap
 * @inherits GleoMap
 *
 * As `MercatorMap`, but:
 * - Uses the `cartesian` CRS instead of the EPSG:3857 Mercator CRS.
 * - All methods that take `Geometry`s as input can take arrays of the form
 *   `[x, y]` instead.
 *
 * When using the `cartesian` CRS, all coordinates are in abstract, unit-less
 * cartesian units in X-Y form.
 *
 * @example
 *
 * ```
 * <div id='gleomap' style='height:500px; width:500px;'></div>
 * <script type='module'>
 * // Import the Gleo files - the paths depend on your importmaps and/or installation
 * import CartesianMap from 'gleo/src/CartesianMap.mjs';
 *
 * const myGleoMap = new CartesianMap('gleomap');
 *
 * // The map center can be specified as a plain array in [x, y] form
 * myGleoMap.center = [1000, 1500];
 *
 * // Initial scale ("zoom"): 1 map unit per CSS pixel
 * myGleoMap.scale = 1;
 * </script>
 * ```
 */

export default class CartesianMap extends GleoMap {
	/**
	 * @constructor CartesianMap(div: HTMLDivElement, options: GleoMap Options)
	 * @alternative
	 * @constructor CartesianMap(divID: string, options: GleoMap Options)
	 */
	constructor(container, options) {
		super(container, { crs: cartesian, ...options });

		new ZoomInOut().addTo(this);
		//new ScaleBar().addTo(this);
		new Attribution().addTo(this);
	}
}

setFactory(function cartesianize(coords, opts) {
	return new Geometry(cartesian, coords, opts);
});
