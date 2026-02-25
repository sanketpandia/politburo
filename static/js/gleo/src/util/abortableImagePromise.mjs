/**
 * @namespace Util
 *
 * @function abortableImagePromise(url: String, controller?: AbortController): Promise
 *
 * Returns a `Promise` to an `HTMLImageElement`, given a URL for the image.
 *
 * If an `AbortController` is given, the `Promise` will reject whenever its
 * signal is activated.
 *
 * @alternative
 * @function abortableImagePromise(url: URL, controller?: AbortController): Promise
 * As before, but can take an instance of `URL` instead of a `String`.
 */

/// TODO: Does using `fetch` offer any benefit?? The logic could be changed.

export default function abortableImagePromise(url, controller) {
	if (!controller) {
		controller = new AbortController();
	}

	const img = new Image();
	return new Promise((res, rej) => {
		img.addEventListener("load", (ev) => res(ev.target));
		img.addEventListener("error", rej);
		img.addEventListener("abort", rej);
		controller.signal.addEventListener("abort", (reason) => {
			img.src = "";
			rej(reason);
		});

		img.crossOrigin = true;
		img.src = url;
	});
}
