// @namespace dom

// Pretty much ripped off Leaflet's `L.DomEvent.getMousePosition`

// @function getMousePosition(ev: MouseEvent, element?: HTMLElement): Array of Number
// Gets normalized mouse position from a DOM mouse/pointer event relative to the
// `element` (border excluded) or to the whole page if not specified.
//
// The return value is an array of the form `[x, y]`, in CSS pixels
export function getMousePosition(ev, element) {
	if (!element) {
		return [ev.clientX, ev.clientY];
	}

	var scale = getScale(element),
		offset = scale.boundingClientRect; // left and top  values are in page scale (like the event clientX/Y)

	return [
		// offset.left/top values are in page scale (like clientX/Y),
		// whereas clientLeft/Top (border width) values are the original values (before CSS scale applies).
		(ev.clientX - offset.left) / scale.x - element.clientLeft,
		(ev.clientY - offset.top) / scale.y - element.clientTop,
	];
}

// Pretty much ripped off Leaflet's `L.DomUtil.getScale`

// @function getScale(el: HTMLElement): Object
// Computes the CSS scale currently applied on the element.
// Returns an object with `x` and `y` members as horizontal and vertical scales respectively,
// and `boundingClientRect` as the result of [`getBoundingClientRect()`](https://developer.mozilla.org/en-US/docs/Web/API/Element/getBoundingClientRect).
export function getScale(element) {
	var rect = element.getBoundingClientRect(); // Read-only in old browsers.

	return {
		x: rect.width / element.offsetWidth || 1,
		y: rect.height / element.offsetHeight || 1,
		boundingClientRect: rect,
	};
}
