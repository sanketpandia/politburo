/**
 * @namespace wirify
 * @inherits Symbol Decorator
 *
 * Turns a symbol into its wireframe representation. Useful for debugging purposes.
 *
 * Can only work on symbols which are drawn as triangles (i.e. not on `Dot`,
 * `Hair` nor `MonteCarloFill`).
 */

export default function wirify(base) {
	class WirifiedAcetate extends base.Acetate {
		constructor(target, opts) {
			super(target, opts);

			if (this._indices._drawMode !== this.glii.TRIANGLES) {
				throw new Error(
					"Cannot wirify symbol: it is not being drawn as triangles"
				);
			}
			this._indices = new this.glii.WireframeTriangleIndices({
				type: this.glii.UNSIGNED_INT,
			});
		}
	}

	return class wirifiedSymbol extends base {
		static Acetate = WirifiedAcetate;
	};
}
