import ButtonGroup from "./ButtonGroup.mjs";
import ZoomIn from "./ZoomIn.mjs";
import ZoomOut from "./ZoomOut.mjs";

/**
 * @class ZoomInOut
 * @inherits ButtonGroup
 * @relationship compositionOf ZoomIn, 0..1, 1..1
 * @relationship compositionOf ZoomOut, 0..1, 1..1
 *
 * A group of two `Button`s: one for zooming in, one for zooming out.
 */
export default class ZoomInOut extends ButtonGroup {
	constructor({ direction = "vertical", ...opts } = {}) {
		super({
			direction,
			buttons: [new ZoomIn(), new ZoomOut()],
			...opts,
		});
	}
}
