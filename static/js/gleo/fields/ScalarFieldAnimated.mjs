import { ScalarField } from "./Field.mjs";

/**
 * @class ScalarFieldAnimated
 * @inherits ScalarField
 *
 * Abstract helper class for animated scalar fields (`AcetateTwinkleField`,
 * `AcetateHeatMirage`). Handles `clear()` and the `dirty` flag.
 */
export default class ScalarFieldAnimated extends ScalarField {
	clear() {
		if (this.#deepDirty) {
			// Clear everything, including the scalar field.
			super.clear();
			this.#deepDirty = false;
		} else {
			// Only clear the RGBA output texture
			this._clear.run();
		}
		return this;
	}

	#deepDirty = false;
	// An animated Acetate is always dirty, meaning it wants to render at every
	// frame.
	get dirty() {
		return true;
	}
	set dirty(d) {
		this.#deepDirty = d;
		super.dirty = d;
	}
}
