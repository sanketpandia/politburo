import parseExpression from "./expression.mjs";
import Stroke from "../../symbols/Stroke.mjs";

// Aux function for the VectorStylesheetLoader.

export default function lineSymbolizer({ id, layout, paint }, interactive) {
	let strokeColour = parseExpression(paint["line-color"], true);
	let strokeDash = paint["line-dasharray"]
		? parseExpression(paint["line-dasharray"])
		: undefined;
	let strokeWidth = paint["line-width"]
		? parseExpression(paint["line-width"])
		: undefined;
	let strokeOpacity = paint["line-opacity"]
		? parseExpression(paint["line-opacity"])
		: undefined;

	if (strokeColour) {
		return function stroke(geom, attrs) {
			const colour = strokeColour(attrs);
			colour[3] *= strokeOpacity?.(attrs) ?? 1;

			return [
				new Stroke(geom, {
					colour,
					dashArray: strokeDash ? strokeDash(attrs) : undefined,
					width: strokeWidth ? strokeWidth(attrs) : undefined,
					interactive,
				}),
			];
		};
	}
}
