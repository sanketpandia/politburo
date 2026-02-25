import Button from "./Button.mjs";

import css from "../dom/CSS.mjs";

css(`
.gleo-ribbon-wrapper.active > button.gleo-control {
	box-shadow: inset 0 0px 7px 3px #1b74ff;
}

.gleo-ribbon-wrapper {
	display: flex;
	align-items: center;
}

.gleo-ribbon-wrapper > .gleo-button-ribbon {
	display: none;
}

.gleo-ribbon-wrapper.active > .gleo-button-ribbon {
	display: flex;
	background: black;
	color: white;
	white-space: nowrap;
	padding: 3px;
}

.gleo-button-ribbon > button {
	background: inherit;
	color: inherit;
	border: none;
}
`);

/**
 * @class ButtonToggle
 * @inherits Button
 * A `Button` which can be toggled active/inactive (or "pressed"/"depressed").
 *
 * @example
 *
 * ```
 * let myButton = new ButtonToggle( ...stuff... )
 * myButton.on('click', ()=>myButton.toggle());
 * ```
 */
export default class ButtonToggle extends Button {
	constructor({
		/**
		 * @option ribbons: Object of Function
		 * A plain `Object` containing label `String`s as the keys and
		 * `Function`s as values.
		 *
		 * When the button is toggled, these will show up by the side of the
		 * button. Clicking on a label will trigger the corresponding function.
		 */
		ribbons = {},
		...opts
	}) {
		super(opts);
		this.element = this.wrapper;
		this.ribbons = ribbons;
	}

	spawnElement() {
		this.button = document.createElement("button");
		this.button.className = "gleo-control";

		this.wrapper = document.createElement("div");
		this.wrapper.className = "gleo-control gleo-ribbon-wrapper";

		this.ribbonContainer = document.createElement("div");
		this.ribbonContainer.className = "gleo-button-ribbon";
		// this.ribbons.innerText = "TODO: ribbons"

		this.wrapper.appendChild(this.button);
		this.wrapper.appendChild(this.ribbonContainer);

		this.element = this.wrapper;
	}

	#active = false;
	#ribbons = {};

	/// @method toggle(): Boolean
	/// Toggles the pressed/depressed state of the button.
	/// Returns `true` when the new state is pressed, `false otherwise.
	toggle() {
		this.setPressed(!this.#active);
	}

	/// @method setActive(active: Boolean): this
	/// Set the pressed state (when passed `true`) or depressed (when `false`).
	setActive(a) {
		if ((this.#active = !!a)) {
			this.wrapper.classList.add("active");
		} else {
			this.wrapper.classList.remove("active");
		}
		return this;
	}

	/// @property ribbons
	/// Runtime value of the `ribbons` instantiation option. Can be overwritten.
	get ribbons() {
		return this.#ribbons;
	}

	set ribbons(r) {
		this.#ribbons = r;

		/// FIXME!!!
		while (this.ribbonContainer.firstChild) {
			this.ribbonContainer.removeChild(this.ribbonContainer.firstChild);
		}

		for (const [key, value] of Object.entries(r)) {
			const ribbonButton = document.createElement("button");
			ribbonButton.innerText = key;
			ribbonButton.addEventListener("click", value);
			this.ribbonContainer.appendChild(ribbonButton);
		}
	}
}
