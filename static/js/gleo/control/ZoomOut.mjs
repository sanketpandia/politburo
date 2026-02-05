import ZoomButton from "./ZoomButton.mjs";

/**
 * @class ZoomOut
 * @inherits ZoomButton
 * A "Zoom Out" button.
 */

export default class ZoomOut extends ZoomButton {
	constructor(opts) {
		super({
			svgString: `<svg width="24" height="24" xmlns="http://www.w3.org/2000/svg"><path style="fill:none;stroke:#464646;stroke-width:2;stroke-linecap:butt;stroke-linejoin:miter;stroke-opacity:1;stroke-dasharray:none" d="M2 12h20"/></svg>`,
			title: "Zoom out",
			...opts,
		});

		this.scaleFactorPerSecond = 4;
	}
}
