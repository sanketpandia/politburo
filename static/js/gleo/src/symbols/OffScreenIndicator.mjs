import HeadingTriangle from "./HeadingTriangle.mjs";

/**
 * @class AcetateOffScreenIndicator
 * @inherits AcetateHeadingTriangle
 */
class AcetateOffScreenIndicator extends HeadingTriangle.Acetate {
	#margin;

	constructor(
		target,
		{
			/**
			 * @option margin: Array of Number = [8, 8, 8, 8]
			 * The margin for the symbols, in CSS pixels, in
			 * `[top, right, bottom, left]` form.
			 */
			margin = [8, 8, 8, 8],

			...opts
		} = {}
	) {
		super(target, opts);

		this.#margin = margin;
		this.bbox.expandXY(-Infinity, -Infinity);
		this.bbox.expandXY(Infinity, Infinity);
	}

	glProgramDefinition() {
		const opts = super.glProgramDefinition();

		const clampFn = `vec4 clampPosition(vec3 pos, float heading) {

				if (pos.x > uScreenEdgeMargin[3] &&	// left
					pos.x < uScreenEdgeMargin[1] &&	// right
					pos.y > uScreenEdgeMargin[2] &&	// down
					pos.y < uScreenEdgeMargin[0]   	// up
				) {
					// Geometry is inside viewport
					// return vec4(pos, 1.0);
					return vec4(0.0);
				}

				if (heading < uCornerAngles[2] ||
					heading > uCornerAngles[3]
				) {
					// Off to the *left*
					return vec4(
						uScreenEdgeMargin[3],
						pos.y * uScreenEdgeMargin[3] / pos.x,
						pos.z,
						1.0
					);
				}

				if (heading < uCornerAngles[1]) {
					// Off to *down*
					return vec4(
						pos.x * uScreenEdgeMargin[2] / pos.y,
						uScreenEdgeMargin[2],
						pos.z,
						1.0
					);
				}

				if (heading < uCornerAngles[0]) {
					// Off to the *right*
					return vec4(
						uScreenEdgeMargin[1],
						pos.y * uScreenEdgeMargin[1] / pos.x,
						pos.z,
						1.0
					);
				}

				if (heading < uCornerAngles[3]) {
					// Off to the *top*
					return vec4(
						pos.x * uScreenEdgeMargin[0] / pos.y,
						uScreenEdgeMargin[0],
						pos.z,
						1.0
					);
				}

			}`;
		// return vec4(
		// 	clamp( pos.x, uScreenEdgeMargin.w, uScreenEdgeMargin.y),
		// 	clamp( pos.y, uScreenEdgeMargin.z, uScreenEdgeMargin.x),
		// 	pos.z,
		// 	1.0
		// );

		return {
			...opts,
			vertexShaderSource: opts.vertexShaderSource + clampFn,
			vertexShaderMain: `
					vFillColour = aFillColour;
					vBorderColour = aBorderColour;
					vBorder = aBorder;
					vEdge = aEdge;

					vec3 position = vec3(aCoords, 1.0) * uTransformMatrix;
					float heading = atan( position.y, position.x * uScreenRatio);
					float cosHeading = cos(heading);
					float sinHeading = sin(heading);

					mat2 headingRotation = mat2(
						cosHeading, sinHeading,
						-sinHeading, cosHeading
					);

					gl_Position =
						clampPosition(position, heading) +
						vec4(headingRotation * aExtrude * uPixelSize, 0.0, 0.0);
				`,
			uniforms: {
				uScreenEdgeMargin: "vec4",
				uScreenRatio: "float",
				uCornerAngles: "vec4",
				...opts.uniforms,
			},
		};
	}

	resize(x, y) {
		super.resize(x, y);

		this._programs.setUniform("uScreenEdgeMargin", [
			+1 - this.#margin[0] / y, // top
			+1 - this.#margin[1] / x, // right
			-1 + this.#margin[2] / y, // bottom
			// reprojectAll() also resets the bounding box, so reapply the hack.
			-1 + this.#margin[3] / x, // left
		]);

		this._programs.setUniform("uScreenRatio", [x / y]);
		this._programs.setUniform("uCornerAngles", [
			Math.atan2(y, x),
			Math.atan2(-y, x),
			Math.atan2(-y, -x),
			Math.atan2(y, -x),
		]);

		return this;
	}

	reprojectAll() {
		super.reprojectAll();
		this.bbox.expandXY(-Infinity, -Infinity);
		this.bbox.expandXY(Infinity, Infinity);
	}
}

/**
 * @class OffScreenIndicator
 * @inherits HeadingTriangle
 * @relationship drawnOn AcetateOffScreenIndicator
 *
 * A `HeadingTriangle` that displays only when its point geometry is off-screen.
 * It appears at the edge of the viewport, heading towards the geometry.
 *
 * The use case is to call attention to other point symbols when they go off-screen.
 *
 * See also the `screenedgify` decorator for a similar concept.
 *
 * @example
 *
 * You can manually instantiate the `AcetateOffScreenIndicator` to customize
 * the margin of the indicators.
 *
 * ```js
 * new OffScreenIndicator.Acetate(map, {margin: [8,8,60,8]});
 *
 * new OffScreenIndicator(geometry, { colour: "red" }).addTo(map);
 * ```
 *
 */
export default class OffScreenIndicator extends HeadingTriangle {
	static Acetate = AcetateOffScreenIndicator;

	constructor(geom, { distance = -10, width = 20, length = 12, ...opts } = {}) {
		super(geom, { ...opts, distance, width, length, yaw: 90 });
	}
}
