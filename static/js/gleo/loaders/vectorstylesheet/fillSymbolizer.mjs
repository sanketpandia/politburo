import parseExpression from "./expression.mjs";
import Fill from "../../symbols/Fill.mjs";

// Aux function for the VectorStylesheetLoader.

export default function fillSymbolizer({ id, layout, paint }, interactive) {
	let fillColour = parseExpression(paint["fill-color"], true);
	let fillOpacity = paint["fill-opacity"]
		? parseExpression(paint["fill-opacity"])
		: undefined;

	if (fillColour) {
		return function solidFill(geom, attrs) {
			const colour = fillColour(attrs);
			colour[3] *= fillOpacity?.(attrs) ?? 1;

			return [
				new Fill(geom, {
					colour,
					interactive,
				}),
			];
		};
	}
}
