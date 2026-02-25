import parseColour from "../3rd-party/css-colour-parser.mjs";
import AcetateInteractive from "../acetates/AcetateInteractive.mjs";

/**
 * @namespace edgify
 * @inherits Symbol Decorator
 *
 * Takes interactive symbols, and adds a solid colour edge to them.
 *
 * This is based on a trivial edge-detection algorithm that runs on the
 * internal interactivity IDs. Thus edges will be shown only around
 * interactive symbols.
 */

export default function edgify(base) {
	if (!base.Acetate instanceof AcetateInteractive) {
		throw new Error(
			`The symbol class to be edgified (${base.constructor.name}) doesn't seem to implement interactivity`
		);
	}

	class EdgifiedAcetate extends base.Acetate {
		#attrs;
		#edgecolour;

		constructor(
			target,
			{
				/**
				 * @option edgeColour: Colour = 'black'
				 * The colour for edge highlighting.
				 */
				edgeColour = "black",

				...opts
			} = {}
		) {
			super(target, { ...opts, interactive: true });

			this.#edgecolour = parseColour(edgeColour);

			// The post-processing step will need a static set of four vertices.
			this.#attrs = new this.glii.InterleavedAttributes(
				{
					usage: this.glii.STATIC_DRAW,
					size: 4,
					growFactor: false,
				},
				[
					{
						// Vertex position
						glslType: "vec2",
						type: Int8Array,
						normalized: false,
					},
					{
						// Texel coords
						glslType: "vec2",
						type: Int8Array,
						normalized: false,
					},
				]
			);

			// prettier-ignore
			this.#attrs.multiSet(0, [
				[ [-1, -1], [0, 0], ],
				[ [-1, +1], [0, 1], ],
				[ [+1, -1], [1, 0], ],
				[ [+1, +1], [1, 1], ],
			]);
		}

		glEdgeProgramDefinition() {
			return {
				attributes: {
					aPos: this.#attrs.getBindableAttribute(0),
					aUV: this.#attrs.getBindableAttribute(1),
				},
				uniforms: {
					uPixelSize: "vec2",
					uEdgeColour: "vec4",
				},
				vertexShaderSource: `void main() {
					gl_Position = vec4(aPos, 0., 1.);
					vUV = aUV;
				}`,
				varyings: { vUV: "vec2" },
				textures: {
					uInteractives: this.idsTexture,
				},
				indexBuffer: new this.glii.SequentialIndices({
					size: 4,
					drawMode: this.glii.TRIANGLE_STRIP,
				}),
				target: this.framebuffer,
				// blend: false,
				fragmentShaderSource: `void main() {
					vec4 sample1 = texture2D(uInteractives, vUV);
					vec4 sample2 = texture2D(uInteractives, vUV + vec2(uPixelSize.x, 0.));
					vec4 sample3 = texture2D(uInteractives, vUV + vec2(0., uPixelSize.y));

					if (sample1 != sample2 || sample1 != sample3) {
						gl_FragColor = uEdgeColour;
					} else {
						gl_FragColor = vec4(0.);
					}
				}`,
			};
		}

		#edgeProgram;

		resize(x, y) {
			super.resize(x, y);

			// Handle both acetates that render to RGBA and to scalar fields
			const targetFramebuffer = this._inAcetate
				? this._inAcetate.framebufferRGBA
				: this.framebuffer;

			if (this.#edgeProgram) {
				this.#edgeProgram.setTarget(targetFramebuffer);
			} else {
				const opts = this.glEdgeProgramDefinition();
				opts.target = targetFramebuffer;
				this.#edgeProgram = new this.glii.WebGL1Program(opts);

				if (this._inAcetate) {
					// If this belongs to a scalar field, make the program run
					// after the scalar field has drawn to its RGBA framebuffer
					this._inAcetate._programs.addProgram(this.#edgeProgram);
				} else {
					// If this is a stand-alone acetate, make the edge program
					// run after the others (RGBA, interactive IDs)
					this._programs.addProgram(this.#edgeProgram);
				}
			}

			this.#edgeProgram.setTexture("uInteractives", this.idsTexture);
			this.#edgeProgram.setUniform("uPixelSize", [1 / x, 1 / y]);
			this.#edgeProgram.setUniform(
				"uEdgeColour",
				this.#edgecolour.map((b) => b / 255)
			);

			return this;
		}

		destroy() {
			this.#attrs.destroy();
			return super.destroy();
		}
	}

	class EdgifiedSymbol extends base {
		static Acetate = EdgifiedAcetate;
	}

	return EdgifiedSymbol;
}
