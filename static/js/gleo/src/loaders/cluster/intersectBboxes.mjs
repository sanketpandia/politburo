import intersectSegments from "./intersectSegments.mjs";
import ExpandBox from "../../geometry/ExpandBox.mjs";

/**
 * Runs intersectSegments on both vertical and horizontal components of the bboxes
 * Returns an array of bboxes, which might be empty.
 */
export default function intersectBboxes(a, b, crs) {
	let horizontal = intersectSegments(a.minX, a.maxX, b.minX, b.maxX, crs.wrapPeriodX);
	let vertical = intersectSegments(a.minY, a.maxY, b.minY, b.maxY, crs.wrapPeriodY);

	let boxes = horizontal
		.map(([x1, x2]) =>
			vertical.map(([y1, y2]) => new ExpandBox().expandXY(x1, y1).expandXY(x2, y2))
		)
		.flat(2);

	return boxes;
}
