import css from "../dom/CSS.mjs";
import HTMLPin from "./HTMLPin.mjs";

css(`
.gleo-balloon {
  --background-color: white;
  --border-color: black;
}

.gleo-balloon { display: flex; }
.gleo-balloon-right, .gleo-balloon-left { align-items: center; }
.gleo-balloon-above, .gleo-balloon-below { justify-content: center; }

.gleo-balloon-tip {position: absolute;}
.gleo-balloon-right > .gleo-balloon-tip ,
.gleo-balloon-left > .gleo-balloon-tip { width: .5rem; height: 1rem; }

.gleo-balloon-above > .gleo-balloon-tip ,
.gleo-balloon-below > .gleo-balloon-tip { width: 1rem; height: .5rem; }

.gleo-balloon > .gleo-balloon-tip::before,
.gleo-balloon > .gleo-balloon-tip::after {
	box-sizing: border-box;
	position: absolute;
	border-width: .5rem;
	content:"";
	border-style: solid;
}
.gleo-balloon > .gleo-balloon-tip::before {
	border-color: transparent;
	z-index: 1;
}
.gleo-balloon > .gleo-balloon-tip::after {
	border-color: transparent;
	z-index: 3;
}
.gleo-balloon-right > .gleo-balloon-tip {left: 0;}
.gleo-balloon-right > .gleo-balloon-tip::before {
	border-right-color: var(--border-color);
	border-left-width: 0}
.gleo-balloon-right > .gleo-balloon-tip::after  {
	border-right-color: var(--background-color);
	border-left-width: 0; left: 1px}

.gleo-balloon-above > .gleo-balloon-tip {bottom: 0;}
.gleo-balloon-above > .gleo-balloon-tip::before {
	border-top-color: var(--border-color);
	border-bottom-width: 0}
.gleo-balloon-above > .gleo-balloon-tip::after  {
	border-top-color: var(--background-color);
	border-bottom-width: 0; bottom: 1px}

.gleo-balloon-left > .gleo-balloon-tip {right: 0;}
.gleo-balloon-left > .gleo-balloon-tip::before {
	border-left-color: var(--border-color);
	border-right-width: 0}
.gleo-balloon-left > .gleo-balloon-tip::after  {
	border-left-color: var(--background-color);
	border-right-width: 0; right: 1px; }

.gleo-balloon-below > .gleo-balloon-tip {top: 0;}
.gleo-balloon-below > .gleo-balloon-tip::before {
	border-bottom-color: var(--border-color);
	border-top-width: 0}
.gleo-balloon-below > .gleo-balloon-tip::after  {
	border-bottom-color: var(--background-color);
	border-top-width: 0; top: 1px}

.gleo-balloon-body {
	position: absolute;
	border-radius: .5rem;
	background: var(--background-color);
	padding: .25rem;
	border: 1px solid var(--border-color);
	z-index: 2;
	display:flex;
}

.gleo-balloon-right > .gleo-balloon-body ,
.gleo-balloon-left > .gleo-balloon-body {
	align-items: center;
	min-height: calc(1rem + 4px);
}

.gleo-balloon-above > .gleo-balloon-body ,
.gleo-balloon-below > .gleo-balloon-body {
	justify-content: center;
	min-width: calc(1rem + 4px);
}

.gleo-balloon-right > .gleo-balloon-body { left: .5rem; }
.gleo-balloon-above > .gleo-balloon-body { bottom: .5rem;}
.gleo-balloon-left  > .gleo-balloon-body { right: .5rem;}
.gleo-balloon-below > .gleo-balloon-body { top: .5rem;}
`);

/**
 * @class Balloon
 * @inherits HTMLPin
 *
 * A styled `HTMLPin`, with a triangular tip on the anchor point and a rounded
 * border. Suitable for popups/popovers/tooltips.
 *
 */
export default class Balloon extends HTMLPin {
	/**
	 * @constructor Balloon(geometry: RawGeometry, contents: HTMLElement, options?: Balloon Options)
	 * @alternative
	 * @constructor Balloon(geometry: RawGeometry, contents: String, options?: Balloon Options)
	 * @alternative
	 * @constructor Balloon(geometry: Array of Number, contents: HTMLElement, options?: Balloon Options)
	 * @alternative
	 * @constructor Balloon(geometry: Array of Number, contents: String, options?: Balloon Options)
	 */
	constructor(
		geometry,
		contents,
		{
			/**
			 * @section Balloon Options
			 * @option position: String = 'above'
			 * Valid values are: `above`, `below`, `left` and `right`.
			 */
			position = "above",
			...opts
			/// FIXME: backgroundColour, borderColour?
		} = {}
	) {
		const el = document.createElement("div");
		const tip = document.createElement("div");
		const body = document.createElement("div");
		el.classList.add("gleo-balloon");
		el.classList.add(`gleo-balloon-${position}`);
		tip.classList.add("gleo-balloon-tip");
		body.classList.add("gleo-balloon-body");
		if (contents instanceof HTMLElement) {
			body.appendChild(contents);
		} else {
			body.innerHTML = contents;
		}
		el.appendChild(tip);
		el.appendChild(body);
		super(geometry, el, opts);
		this.#body = body;
	}

	#body;
	/**
	 * @section
	 * @property body: HTMLElement
	 * Read-only accessor to the `HTMLElement` for the body of the balloon, not
	 * including the tip.
	 */
	get body() {
		return this.#body;
	}
}
