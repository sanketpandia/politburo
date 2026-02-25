/**
 * @class Actuator
 *
 * > Actuator, n:
 * > a mechanical device for moving or controlling something
 *
 * In Gleo, an `Actuator` is an attachment to a `GleoMap` that handles DOM events
 * to change the view in some ways.
 *
 *
 * @example
 *
 * Any actuators defined will be registered into the map and enabled by default.
 * In order to manage them, use the `actuators` property of the map.
 *
 * ```js
 * import GleoMap from "gleo/src/Map.mjs";
 * import "gleo/src/actuators/DragActuator.mjs";
 * import "gleo/src/actuators/WheelActuator.mjs";
 *
 * const gleomap = new GleoMap();
 *
 * gleomap.actuators.get('drag').disable();
 * gleomap.actuators.get('wheel').wheelPxPerLog2 = 55;
 * ```
 *
 *
 * @constructor Actuator(map: GleoMap)
 *
 * @section Subclass interface
 * @uninheritable
 * Subclasses of `Actuator` shall provide the following:
 *
 * @method enable(): this
 * Enable this actuator. Should add DOM event listeners.
 *
 * @method disable(): this
 * Disable this actuator. Should remove DOM event listeners.
 *
 *
 */
