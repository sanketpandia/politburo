import ZoomButton from "./ZoomButton.mjs";

/**
 * @class ZoomIn
 * @inherits ZoomButton
 * A "Zoom In" button.
 */

export default class ZoomIn extends ZoomButton {
	constructor(opts) {
		super({
			svgString: `<svg width="24" height="24" xmlns="http://www.w3.org/2000/svg"><path style="fill:#464646;stroke:none;stroke-width:1px;stroke-linecap:butt;stroke-linejoin:miter;stroke-opacity:1;fill-opacity:1" d="M11 2h2v9h9v2h-9v9h-2v-9H2v-2h9z"/></svg>`,
			title: "Zoom in",
			...opts,
		});

		this.scaleFactorPerSecond = 1 / 4;
	}
}
