import Sprite from "./Sprite.mjs";

let textWorker;
let workId = 0;
let textDrawer, textDrawerCanvas;
let canUseExternalWorker;
try {
	// Use the external-file web worker for text rendering only if:
	// - OffscreenCanvas is a thing, and...
	const a = globalThis?.OffscreenCanvas;
	// - ...the name of this module is TextLabel.mjs (which won't happen if it's
	// been bundled)
	const b = import.meta?.url?.match(/TextLabel\.mjs/);
	canUseExternalWorker = !!a && !!b;
} catch (ex) {
	canUseExternalWorker = false;
}

if (canUseExternalWorker) {
	const ownURL = import.meta.url;
	try {
		textWorker = new Worker(ownURL.replace(/TextLabel.mjs$/, "text/textWorker.js"));
	} catch (ex) {
		canUseExternalWorker = false;
	}
}

if (!canUseExternalWorker) {
	if (globalThis?.OffscreenCanvas) {
		try {
			// If external workers are not available but offscreen canvases are,
			// then spawn a blob worker. The code for the blob is copy-pasted
			// from the external worker code.
			const blob = new Blob(
				[
					`
const textDrawerCanvas = new OffscreenCanvas(16, 16);
const textDrawer = textDrawerCanvas.getContext("2d", { willReadFrequently: true });

let devicePixelRatio = 1;

onmessage = function onmessage({
	data: {
		command, // either "render" or "loadFontFace"
		workId,

		str,
		font,
		colour,
		align,
		baseline,
		outlineWidth,
		outlineColour,
		// cache = false,

		family,
		source,
		descriptors,

		...data
	},
	...msg
}) {
	//console.log('Worker: Message received from main script', workId, str);

	if (command === "render") {
		font = font.replace(/\d+/, (n) => n * devicePixelRatio);

		textDrawer.font = font;
		textDrawer.textAlign = align;
		textDrawer.textBaseline = baseline;

		let metrics = textDrawer.measureText(str);

		const left = metrics.actualBoundingBoxLeft + outlineWidth + 1;
		const right = metrics.actualBoundingBoxRight + outlineWidth + 1;
		const up = metrics.actualBoundingBoxAscent + outlineWidth + 1;
		const down = metrics.actualBoundingBoxDescent + outlineWidth + 1;

		const width = Math.ceil(left + right) + 1;
		const height = Math.ceil((textDrawerCanvas.height = up + down)) + 1;

		if (textDrawerCanvas.width < width || textDrawerCanvas.height < height) {
			textDrawerCanvas.width = Math.max(width, textDrawerCanvas.width);
			textDrawerCanvas.height = Math.max(height, textDrawerCanvas.height);
			textDrawer.font = font;
			textDrawer.textAlign = align;
			textDrawer.textBaseline = baseline;
		} else {
			textDrawer.clearRect(0, 0, width, height);
		}

		if (outlineWidth > 0) {
			textDrawer.lineWidth = devicePixelRatio * outlineWidth * 2;
			textDrawer.strokeStyle = outlineColour;
			textDrawer.strokeText(str, left, up);
		}

		textDrawer.fillStyle = colour;
		textDrawer.fillText(str, left, up);

		const imageData = textDrawer.getImageData(0, 0, width, height);

		const returnMsg = {
			workId,
			imageData,
			left: left,
			up: up,
			width: width,
			height: height,
			scale: 1 / devicePixelRatio,
		};

		return postMessage(returnMsg);
	} else if (command === "loadFontFace") {
		const font = new FontFace(family, source, descriptors);
		self.fonts.add(font);
		font.load()
			.catch((ex) =>
				postMessage({
					workId,
					error: ex,
				})
			)
			.then(() => {
				console.log(self.fonts);

				postMessage({
					workId,
				});
			});
	} else if (command === "setDevicePixelRatio") {
		devicePixelRatio = data.devicePixelRatio;
	}
};
		`,
				],
				{ type: "text/javascript" }
			);

			textWorker = new Worker(window.URL.createObjectURL(blob));
		} catch (ex) {}
	}

	try {
		if (!textWorker || !textWorker?.onmessage) {
			textDrawerCanvas = document.createElement("canvas");
			textDrawerCanvas.width = 1;
			textDrawerCanvas.height = 1;
			textDrawer = textDrawerCanvas.getContext("2d", { willReadFrequently: true });

			// document.body.appendChild(textDrawerCanvas);
			// textDrawerCanvas.style.border = "2px solid blue";
		}
	} catch (ex) {
		console.warn("Cannot use TextLabel on headless environments");
	}
}

if (textWorker && textWorker?.postMessage) {
	textWorker?.postMessage({
		command: "setDevicePixelRatio",
		// devicePixelRatio: .1,
		devicePixelRatio: window.devicePixelRatio,
	});

	window.addEventListener("resize", () => {
		textWorker.postMessage({
			command: "setDevicePixelRatio",
			devicePixelRatio: window.devicePixelRatio,
		});
	});
}

const imageCache = new Map();

/**
 * @class TextLabel
 * @inherits Sprite
 * @relationship drawnOn AcetateSprite
 * @relationship compositionOf Bin, 1..1, 0..1
 *
 * A text label anchored to a point geometry.
 *
 * Internally treated like a `Sprite`, by rasterizing the text via 2D Canvas.
 *
 * @example
 * ```js
 * new TextLabel([0, 0], {
 * 	str: "Hello world!",
 * }).addTo(map);
 * ```
 */

export default class TextLabel extends Sprite {
	/**
	 * @constructor TextLabel(geom: Geometry, opts?: TextLabel Options)
	 */
	constructor(
		geom,
		{
			str,
			font = "16px Sans",
			colour,
			color,
			align = "start",
			baseline = "alphabetic",
			outlineWidth = 0,
			outlineColour = "white",
			cache = false,
			...opts
		} = {}
	) {
		/**
		 * @section
		 * @aka TextLabel options
		 * @option str: String
		 * The text itself
		 * @option font: String = "16px Sans"
		 * A definition of a [CSS font](https://developer.mozilla.org/docs/Web/CSS/font)
		 * @option colour: String = "black"
		 * The CSS colour for the text fill (**not** a gleo `Colour`!).
		 * @option align: String = "start"
		 * Text alignment, as per [2D canvas' `textAlign`](https://developer.mozilla.org/docs/Web/API/CanvasRenderingContext2D/textAlign).
		 * @option baseline: String = "alphabetic"
		 * Text baseline, as per [2D canvas' `textBaseline`](https://developer.mozilla.org/en-US/docs/Web/API/CanvasRenderingContext2D/textBaseline).
		 * @option outlineWidth: Number = 0
		 * Size, in CSS pixels, of the text outline
		 * @option outlineColour: String = "white"
		 * The CSS colour for the text outline (**not** a gleo `Colour`!)
		 * @option cache: Boolean = false
		 * Whether to cache the rendered text for later use. Should be
		 * set to `true` if the text label is expected to be removed/re-added,
		 * or if several `TextLabel`s with the exact same data exist.
		 */

		let key;
		if (cache) {
			key = JSON.stringify({
				str,
				font,
				colour,
				align,
				baseline,
				outlineWidth,
				outlineColour,
			});
			const cached = imageCache.get(key);
			if (cached) {
				return super(geom, {
					image: cached.imageData,
					spriteAnchor: [cached.left, cached.up],
					spriteSize: [cached.width, cached.height],
					spriteScale: cached.scale,
					...opts,
				});
			}
		}

		if (textWorker) {
			const myWorkId = workId++;

			textWorker.postMessage({
				command: "render",
				workId: myWorkId,
				str,
				font,
				colour: colour ?? color ?? "black",
				align,
				baseline,
				outlineWidth,
				outlineColour,
				cache,
				...opts,
			});

			const imageReady = new Promise((res, rej) => {
				function waitMessage(msg) {
					if (msg.data.workId === myWorkId) {
						//console.log("Done", myWorkId, msg);
						textWorker.removeEventListener("message", waitMessage);
						res(msg.data);
					}
				}
				textWorker.addEventListener("message", waitMessage);
			});

			super(geom, {
				image: imageReady.then((i) => {
					if (cache) {
						imageCache.set(key, i);
					}
					this._anchor = [i.left, i.up];
					this._spriteSize = [i.width, i.height];
					this.spriteScale = i.scale;

					return i.imageData;
				}),
				spriteAnchor: [0, 0],
				spriteSize: [16, 16],
				...opts,
			});
		} else {
			font = font.replace(/\d+/, (n) => n * (devicePixelRatio ?? 1));

			textDrawer.font = font;
			textDrawer.textAlign = align;
			textDrawer.textBaseline = baseline;

			let metrics = textDrawer.measureText(str);

			const left = metrics.actualBoundingBoxLeft + outlineWidth + 1;
			const right = metrics.actualBoundingBoxRight + outlineWidth + 1;
			const up = metrics.actualBoundingBoxAscent + outlineWidth + 1;
			const down = metrics.actualBoundingBoxDescent + outlineWidth + 1;

			const width = Math.ceil(left + right) + 1;
			const height = Math.ceil((textDrawerCanvas.height = up + down)) + 1;

			if (textDrawerCanvas.width < width || textDrawerCanvas.height < height) {
				textDrawerCanvas.width = Math.max(width, textDrawerCanvas.width);
				textDrawerCanvas.height = Math.max(height, textDrawerCanvas.height);
				textDrawer.font = font;
				textDrawer.textAlign = align;
				textDrawer.textBaseline = baseline;
			} else {
				textDrawer.clearRect(0, 0, width, height);
			}

			if (outlineWidth > 0) {
				textDrawer.lineWidth = (devicePixelRatio ?? 1) * outlineWidth * 2;
				textDrawer.strokeStyle = outlineColour;
				textDrawer.strokeText(str, left, up);
			}

			textDrawer.fillStyle = colour ?? color ?? "black";
			textDrawer.fillText(str, left, up);

			const imageData = textDrawer.getImageData(0, 0, width, height);
			if (cache) {
				imageCache.set(key, {
					imageData,
					left,
					up,
					width,
					height,
					scale: 1 / (devicePixelRatio ?? 1),
				});
			}

			super(geom, {
				image: imageData,
				spriteAnchor: [left, up],
				spriteSize: [width, height],
				spriteScale: 1 / (devicePixelRatio ?? 1),
				...opts,
			});
		}
	}

	/**
	 * @function addFontFace(family: String, source: String, descriptors?: Object): Promise
	 *
	 * Registers a new font face (AKA typeface) for use with `TextLabel`.
	 *
	 * Note that for technical reasons (i.e. "text is rendered inside a web worker"),
	 * typefaces defined in the document's CSS via `@font-face` are not available
	 * to Gleo.
	 *
	 * The parameters to this static function are the same as the
	 * [`FontFace` constructor](https://developer.mozilla.org/en-US/docs/Web/API/FontFace/FontFace).
	 *
	 * Beware: any relative URLs used in the `source` will be interpreted as
	 * being relative to *the URL of the web worker code module*. Usage of
	 * absolute URLs is therefore highly encouraged.
	 *
	 * Returns a `Promise` that resolves when the font face has been loaded.
	 */
	static addFontFace(family, source, descriptors) {
		if (textWorker) {
			const myWorkId = workId++;

			const fontReady = new Promise((res, rej) => {
				function waitMessage(msg) {
					if (msg.data.workId === myWorkId) {
						//console.log("Done", myWorkId, msg);
						textWorker.removeEventListener("message", waitMessage);
						if (msg.data.error) {
							rej(msg.data.error);
						} else {
							res();
						}
					}
				}
				textWorker.addEventListener("message", waitMessage);
			});

			textWorker.postMessage({
				command: "loadFontFace",
				workId: myWorkId,
				family,
				source,
				descriptors,
			});

			return fontReady;
		} else {
			const font = new FontFace(family, source, descriptors);
			document.fonts.add(font);
			return font.load();
		}
	}
}
