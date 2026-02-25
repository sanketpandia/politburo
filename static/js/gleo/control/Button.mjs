import Control from "./Control.mjs";

import css from "../dom/CSS.mjs";

css(`
button.gleo-control {
	display: block;
	width: 3em;
	height: 3em;
	border-radius: 0.75em;
	border: #888 solid 0.375em;
	padding: 0;
}
.gleo-controlcorner > button.gleo-control {
	margin: 0.5em;
}

button.gleo-control > svg {
	vertical-align: middle;
}

button.gleo-control:active {
	inset 0 0px 7px 3px #1b74ff;
}
`);

/**
 * @class Button
 * @inherits Control
 * A single UI button.
 */
export default class Button extends Control {
	constructor({
		/**
		 * @option string: String
		 * The (text) label to be shown inside the button.
		 */
		string,

		/**
		 * @option svgString: String
		 * The icon for the button, as a string containing a SVG document.
		 * Mutually exclusive with `string`.
		 */
		svgString,

		/**
		 * @option title: String
		 * The text for the [`title` HTML attribute](https://developer.mozilla.org/en-US/docs/Web/HTML/Global_attributes/title)
		 * of the button.
		 */
		title,
		...opts
	} = {}) {
		super(opts);
		if (string) {
			this.button.innerText = string;
		}

		if (svgString) {
			this.button.innerHTML = svgString;
		}

		if (title) {
			this.button.title = title;
		}
	}
	spawnElement() {
		this.element = this.button = document.createElement("button");
		this.button.className = "gleo-control";

		// TODO: ARIA stuff.
	}

	/**
	 * @section Button event handlers
	 * @method on(eventName: String, handler: Function): this
	 * Alias to `addEventListener`.
	 * @method off(eventName: String, handler: Function): this
	 * Alias to `removeEventListener`.
	 */
	on() {
		return this.addEventListener.apply(this, arguments);
	}
	off() {
		return this.removeEventListener.apply(this, arguments);
	}

	/**
	 * @method addEventListener(eventName: String, handler: Function): this
	 * Attaches an event handler to a DOM event of the `HTMLButtonElement` for
	 * the control.
	 */
	addEventListener(eventName, handler) {
		this.button.addEventListener(eventName, handler);
		return this;
	}

	/**
	 * @method addEventListener(eventName: String, handler: Function): this
	 * Detaches an event handler to a DOM event from the `HTMLButtonElement` for
	 * the control.
	 */
	removeEventListener(eventName, handler) {
		this.button.removeEventListener(eventName, handler);
		return this;
	}

	// TODO: disable, enable.
	// TODO: focus, blur
	// TODO: keyboard accesibility (keydown/up for space & enter)
	// TODO: Wrap keyboard & pointer events (i.e. fire "pressstart", "pressend")
}
