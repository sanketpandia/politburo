import Control from "./Control.mjs";
import Button from "./Control.mjs";

import css from "../dom/CSS.mjs";

css(`
.gleo-buttongroup {
	display: flex;
	min-width: 3em;
	min-height: 3em;
}
.gleo-buttongroup.vertical { flex-direction: column }
.gleo-buttongroup.horizontal { flex-direction: row }
.gleo-controlcorner > .gleo-buttongroup {
	margin: 0.5em;
}
.gleo-buttongroup.vertical > button.gleo-control,
.gleo-buttongroup.vertical > div.gleo-control > button.gleo-control
{
	border-bottom-width: 0.1875em;
	border-top-width: 0.1875em;
	border-radius: 0;
}
.gleo-buttongroup.vertical > button.gleo-control:first-child,
.gleo-buttongroup.vertical > div.gleo-control:first-child > button
{
	border-top-right-radius: 0.75em;
	border-top-left-radius: 0.75em;
	border-top-width: 0.375em;
}
.gleo-buttongroup.vertical > button.gleo-control:last-child,
.gleo-buttongroup.vertical > div.gleo-control:last-child > button {
	border-bottom-right-radius: 0.75em;
	border-bottom-left-radius: 0.75em;
	border-bottom-width: 0.375em;
}
.gleo-buttongroup.horizontal > button.gleo-control {
	border-left-width: 0.1875em;
	border-right-width: 0.1875em;
	border-radius: 0;
}
.gleo-buttongroup.horizontal > button.gleo-control:first-child {
	border-top-left-radius: 0.75em;
	border-bottom-left-radius: 0.75em;
	border-left-width: 0.375em;
}
.gleo-buttongroup.horizontal > button.gleo-control:last-child {
	border-top-right-radius: 0.75em;
	border-bottom-right-radius: 0.75em;
	border-right-width: 0.375em;
}
`);

/**
 * @class ButtonGroup
 * @inherits Control
 * @relationship compositionOf Button, 0..1, 0..n
 *
 * A control for nesting `Button` controls inside.
 */
export default class ButtonGroup extends Control {
	constructor({
		// @option direction: String = 'vertical'
		// Whether the nested buttons align horizontally or vertically. Valid
		// values are `"horizontal"` and `"vertical"`.
		direction = "vertical",
		// @option buttons: Array of Button = []
		// The initial set of `Button` controls to be in this group.
		buttons = [],
		...opts
	}) {
		super(opts);

		this.buttons = buttons;
		this.element.classList.add("gleo-buttongroup");
		if (direction === "vertical") {
			this.element.classList.add("vertical");
		} else {
			this.element.classList.add("horizontal");
		}
	}

	addTo(map) {
		this.buttons.forEach((button) => {
			button.position = this.element;
			button.addTo(map);
		});
		super.addTo(map);
	}

	remove() {
		this.buttons.forEach((button) => button.remove());
	}
}
