/**
 * @namespace Util
 *
 * @function imagifyFetchResponse(r: Response): Promise to HTMLImageElement
 *
 * Given a `Response` from a `fetch` call, wraps it into an image - meant for
 * `fetch` operations that are supposed to retrieve an image.
 *
 * If the parameter is not a `Response`, it will be passed through transparently.
 *
 */

export default function imagifyFetchResponse(r) {
	if (r instanceof Response) {
		return new Promise((res, rej) => {
			const img = new Image();
			img.addEventListener("load", (ev) => res(ev.target));
			img.addEventListener("error", rej);
			img.addEventListener("abort", rej);
			img.crossOrigin = true;
			r.blob().then((blob) => (img.src = URL.createObjectURL(blob)));
		});
	} else {
		return r;
	}
}
