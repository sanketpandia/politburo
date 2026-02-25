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
				// console.log(self.fonts);

				postMessage({
					workId,
				});
			});
	} else if (command === "setDevicePixelRatio") {
		devicePixelRatio = data.devicePixelRatio;
	}
};
