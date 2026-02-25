/**
 * @class ExpandBox
 *
 * Minimalistic, simplistic, 2-dimensional, *expanding* bounding box implementation.
 *
 * This class is needed only when there's a need to calculate a bbox that covers
 * a given set of points. Bounding boxes that can be trivially calculated are best
 * handled manually as 4-element arrays.
 *
 * This implementation also assumes that the bounds are parallel to the CRS's axes,
 * and is not suitable for boundign boxes whenever there's a yaw rotation involved.
 */

export default class ExpandBox {
	constructor() {
		this.reset();
	}

	/**
	 * @method reset(): this
	 * Resets all properties to their `Infinity`/`-Infinity` default values.
	 */
	reset() {
		/**
		 * @property minX: Number = Infinity
		 * @property maxX: Number = -Infinity
		 * @property minY: Number = Infinity
		 * @property maxY: Number = -Infinity
		 */
		this.minX = this.minY = Infinity;
		this.maxX = this.maxY = -Infinity;
		return this;
	}

	/**
	 * @method clone(): ExpandBox
	 * Returns a cloned copy of this box.
	 */
	clone() {
		const newBox = new ExpandBox();
		newBox.minX = this.minX;
		newBox.minY = this.minY;
		newBox.maxX = this.maxX;
		newBox.maxY = this.maxY;
		return newBox;
	}

	/**
	 * @method expandPair(xy: Array of Number): this
	 *
	 * Expands the bounding box to cover the given coordinate pair. The coordinate
	 * pair is expected to have the form `[x, y]`.
	 */
	expandPair([x, y]) {
		this.minX = Math.min(this.minX, x);
		this.maxX = Math.max(this.maxX, x);
		this.minY = Math.min(this.minY, y);
		this.maxY = Math.max(this.maxY, y);
		return this;
	}

	/**
	 * @method expandXY(x: Number, y: Number): this
	 *
	 * Expands the bounding box to cover the given coordinate pair.
	 */
	expandXY(x, y) {
		this.minX = Math.min(this.minX, x);
		this.maxX = Math.max(this.maxX, x);
		this.minY = Math.min(this.minY, y);
		this.maxY = Math.max(this.maxY, y);
		return this;
	}

	/**
	 * @method expandGeometry(geom: RawGeometry): this
	 * Expands the bounding box to cover all points of the given `Geometry`.
	 * Note that no reprojection is performed, and that an `ExpandBox` is
	 * CRS-agnostic.
	 */
	expandGeometry(geom) {
		const coords = geom.coords;
		for (let i = 0, l = coords.length; i < l; i += 2) {
			this.expandXY(coords[i], coords[i + 1]);
		}
		return this;
	}

	/**
	 * @method expandPercentage(p: Number): this
	 *
	 * Expand the bounding box by the given percentage **on four sides**.
	 *
	 * e.g. a value of `0.1` will raise the top by 10%, lower
	 * the bottom by 10% (idem for left & right), increasing the height
	 * by 20% (idem for width).
	 */
	expandPercentage(p) {
		const h = this.maxX - this.minX;
		const w = this.maxY - this.minY;
		if (isFinite(w)) {
			this.minX -= w * p;
			this.maxX += w * p;
		}
		if (isFinite(h)) {
			this.minY -= h * p;
			this.maxY += h * p;
		}

		return this;
	}

	/**
	 * @method expandPercentages(px: Number, py: Number): this
	 *
	 * Expand the bounding box by the given percentages, to the left and right by
	 * `px`, and to the top and bottom by `py`
	 */
	expandPercentages(px, py) {
		const h = this.maxX - this.minX;
		const w = this.maxY - this.minY;
		if (isFinite(w)) {
			this.minX -= w * px;
			this.maxX += w * px;
		}
		if (isFinite(h)) {
			this.minY -= h * py;
			this.maxY += h * py;
		}

		return this;
	}

	/**
	 * @method intersectsBox(b: ExpandBox): Boolean
	 * Returns `true` if the given `ExpandBox` has at least one point in
	 * common.
	 */
	intersectsBox(b) {
		return (
			b.maxX > this.minX &&
			b.minX < this.maxX &&
			b.maxY > this.minY &&
			b.minY < this.maxY
		);
	}

	/**
	 * @method containsBox(b: ExpandBox): Boolean
	 * Returns `true` if the given `ExpandBox` completely fits.
	 */
	containsBox(b) {
		return (
			b.maxX < this.maxX &&
			b.minX > this.minX &&
			b.maxY < this.maxY &&
			b.minY > this.minY
		);
	}
}
