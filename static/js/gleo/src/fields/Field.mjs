import Acetate from "../acetates/Acetate.mjs";

/**
 * @class Field
 * @inherits Acetate
 * @relationship compositionOf Acetate, 1..1, 0..n
 *
 * Abstract acetate, basis for `ScalarField` (one 32-bit float per pixel)
 * and `VectorField` (two 32-bit floats per pixel).
 *
 */

class Field extends Acetate {
	#subAcetateAttrs;
	#fieldFramebuffer;
	#fieldTexture;
	#fieldClear;
	#clearValue;

	#subAcetates = [];

	#glFormat;
	#glInternalFormat;

	constructor(
		target,
		{
			/**
			 * @option clearValue: Array of Number = [0,0,0,0]
			 * The value of the scalar field prior to render data on it. It should
			 * be zero for most cases (where scalar symbols are using the `ADD` blend
			 * equation), but should be a different number when using the `MIN`
			 * blend equation.
			 */
			clearValue = [0, 0, 0, 0],

			// For subclassing only - the GL Format to be used when creating the
			// field texture/framebuffer
			glFormat,

			// For subclassing only - the GL "internal format" to be used when
			// creating the field texture/framebuffer
			glInternalFormat,

			...opts
		} = {}
	) {
		super(target, opts);

		if (this.constructor.name === "Field") {
			throw new Error(
				"Cannot instantiate Field. Use a subclass, like GreyscaleField or HeatMap"
			);
		}

		// This acetate shall spawn a floating-point texture, which is
		// not available in a default WebGL1 environment. Therefore,
		// this checks for availability of float point textures.
		try {
			if (!(this.glii.gl instanceof WebGL2RenderingContext)) {
				// This enables the *creation* of floating point textures
				this.glii.loadExtension("OES_texture_float");
			}
			// This enables *rendering* to a floating point texture
			this.glii.loadExtension("EXT_color_buffer_float");

			// This enables *overlapping* triangles on the floating point texture
			this.glii.loadExtension("EXT_float_blend");
		} catch (ex) {
			throw new Error(
				"Scalar fields require floating-point textures, but this browser/GPU does not support WebGL2, and does not support the OES_texture_float EXT_color_buffer_float and EXT_float_blend extensions."
			);
		}

		// The post-processing step will need a static set of four vertices.
		this.#subAcetateAttrs = new this.glii.InterleavedAttributes(
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
		this.#subAcetateAttrs.multiSet(0, [
			[ [-1, -1], [0, 0], ],
			[ [-1, +1], [0, 1], ],
			[ [+1, -1], [1, 0], ],
			[ [+1, +1], [1, 1], ],
		]);

		this.#clearValue = clearValue;
		this.#glFormat = glFormat;
		this.#glInternalFormat = glInternalFormat;
	}

	// INCOMPLETE program definition, lacking fragment shader.
	glProgramDefinition() {
		const opts = super.glProgramDefinition();

		return {
			...opts,
			attributes: {
				aPos: this.#subAcetateAttrs.getBindableAttribute(0),
				aUV: this.#subAcetateAttrs.getBindableAttribute(1),
			},
			vertexShaderMain: `
				gl_Position = vec4(aPos, 0., 1.);
				vUV = aUV;
			`,
			varyings: { vUV: "vec2" },
			textures: {
				uField: this.#fieldTexture,
			},
			indexBuffer: new this.glii.SequentialIndices({
				size: 4,
				drawMode: this.glii.TRIANGLE_STRIP,
			}),
			target: opts.target,
			blend: false,
		};
	}

	// Should not be explicitly called. Instantiating an `Acetate` will add it
	// to the specified target.
	addAcetate(ac) {
		if (
			!ac.constructor.PostAcetate ||
			!(this instanceof ac.constructor.PostAcetate)
		) {
			throw new Error(
				"Bad acetate subclass when adding sub-acetate to a scalar field"
			);
		}

		this.platina.fire("acetateadded", ac);

		this.#subAcetates.push(ac);
		ac._inAcetate = this;
		if (this.#fieldFramebuffer) {
			// Wait until the acetate to be added has been fully initialized
			requestAnimationFrame(() =>
				ac.resize(this.#fieldFramebuffer.width, this.#fieldFramebuffer.height)
			);
		}
		this.dirty = true;
		return this;
	}

	/**
	 * @property subAcetates: Array of Acetate
	 * List of acetates currently drawing into this scalar field. Read-only.
	 */
	get subAcetates() {
		return this.#subAcetates;
	}

	/**
	 * As `Platina`'s `multiAdd`: classifies the symbols and sends them to the
	 * appropriate sub-acetate.
	 */
	multiAdd(symbols) {
		const bins = new Map();

		// Just for MultiSymbol class
		// symbols.forEach((s) => {
		// 	if (s.symbols) {
		// 		symbols = symbols.concat(s.symbols);
		// 	}
		// });

		symbols.forEach((s) => {
			const ac = s.constructor.Acetate;
			if (ac) {
				const bin = bins.get(ac);
				if (bin) {
					bin.push(s);
				} else {
					bins.set(ac, [s]);
				}
			}
		});

		for (let [ac, syms] of bins.entries()) {
			this.getAcetateOfClass(ac).multiAdd(syms);
		}
		this.dirty = true;
		return this;
	}

	/**
	 * Remove symbols from the sub-acetates
	 */
	multiRemove(syms) {
		this.#subAcetates.forEach((ac) => {
			ac.multiRemove(syms.filter((s) => s._inAcetate === ac));
		});
		this.dirty = true;
		return this;
	}

	getAcetateOfClass(acetateClass) {
		let ac = this.#subAcetates.find(
			(a) => Object.getPrototypeOf(a).constructor === acetateClass
		);
		if (ac) {
			return ac;
		}

		ac = new acetateClass(this.glii);
		this.addAcetate(ac);
		return ac;
	}

	// Resizing a scalar field will resize both the output texture/framebuffer
	// and the input texture/framebuffer.
	resize(x, y) {
		const glii = this.glii;

		if (!this.#fieldFramebuffer) {
			//this.#outTexture && this.#outTexture.destroy();
			//this.#framebuffer && this.#framebuffer.destroy();
			this.#fieldTexture = new glii.Texture({
				format: this.#glFormat,
				internalFormat: this.#glInternalFormat,
				// format: glii.gl.RED,
				// internalFormat: glii.gl.R32F,
				type: glii.FLOAT,
			});
			this.#fieldFramebuffer = new glii.FrameBuffer({
				color: [this.#fieldTexture],
				// depth: new glii.RenderBuffer({
				// 	width: x,
				// 	height: y,
				// 	internalFormat: glii.DEPTH_COMPONENT16,
				// }),
				// stencil: renderbuffer,
				width: x,
				height: y,
			});
		} else {
			this.#fieldFramebuffer.resize(x, y);
		}
		super.resize(x, y);

		this.#fieldClear = new this.glii.WebGL1Clear({
			color: this.#clearValue,
			target: this.#fieldFramebuffer,
			//depth: 1,
		});

		this._program.setTexture("uField", this.#fieldTexture);
		// this._program.setUniform("uPixelSize", [2 / x, 2 / y]);

		this.#subAcetates.forEach((ac) => ac.resize(x, y));

		return this;
	}

	/**
	 * @section Acetate interface
	 * @property framebuffer: FrameBuffer
	 * The scalar field framebuffer. Read-only. Meant only to be read from
	 * `Acetate`s rendering into this scalar field.
	 */
	get framebuffer() {
		return this.#fieldFramebuffer;
	}

	/**
	 * @property framebufferRGBA: FrameBuffer
	 * The RGBA framebuffer. Read-only. Meant only to be used by some
	 * decorators.
	 */
	get framebufferRGBA() {
		return super.framebuffer;
	}

	clear() {
		if (this.dirty) {
			this.#fieldClear.run();
		}
		// super.clear();	// No need, since it overwrites values anyway
		// this.#subAcetates.forEach(ac=>ac.clear())	// No need: they draw into the field
		return this;
	}

	redraw(...args) {
		if (!this.dirty) {
			return;
		}

		// Will just set #dirty to false, and clear framebuffers
		// (since this._knownSymbols is empty)
		super.redraw(...args);

		this.#subAcetates.forEach((ac) => ac.redraw(...args));

		this.runProgram();
		return this;
	}

	set dirty(d) {
		super.dirty = d;
		this.#subAcetates.forEach((ac) => (ac.dirty ||= d));
	}
	get dirty() {
		return super.dirty;
	}

	// For compatibility
	get cellSize() {
		return 1;
	}

	get _fieldTexture() {
		return this.#fieldTexture;
	}

	destroy() {
		super.destroy();

		this.#subAcetateAttrs.destroy();
		this.#subAcetates.forEach((ac) => ac.destroy());
		this.#subAcetates = [];
		this.#fieldTexture.destroy();
		this.#fieldFramebuffer.destroy();
	}

	// Noop: Since there are no symbols, this won't be even called, but needs
	// to be defined for ScalarFields to be destroyed.
	deallocate() {}

	/**
	 * @section
	 * @method getFieldVaueAt(x: Number, y: Number): Number
	 * Returns the value of the scalar field at the given (CSS pixel) coordinates
	 *
	 * Used internally during event handling, so that the event can provide
	 * the field value at the coordinates of the pointer event.
	 *
	 * Returns `undefined` if the coordinates fall outside of the acetate.
	 */
	getFieldValueAt(x, y) {
		if (!this.#fieldFramebuffer) {
			return undefined;
		}

		const h = this.#fieldFramebuffer.height;
		const w = this.#fieldFramebuffer.width;

		if (y < 0 || y > h || x < 0 || x > w) {
			return this._nan;
		}

		const dpr = devicePixelRatio ?? 1;

		// Textures are inverted in the Y axis because WebGL shenanigans. I know.
		return this.#fieldFramebuffer.readPixels(dpr * x, h - dpr * y, 1, 1);
	}

	static _nan = NaN;

	dispatchPointerEvent(ev, init) {
		/**
		 * @class GleoEvent
		 * @property value: Number
		 * For scalar field acetates (e.g. `AcetateHeatMap`) marked as
		 * "queryable", this contains the value for the scalar field for the
		 * pixel where the event took place.
		 */
		if (this.queryable) {
			// The current implementation will query the framebuffer/texture
			// when the expression is evaluated, which might be too late
			// specially if the event is logged into the console. The following
			// is the previous implementation, which is immediate but
			// potentially very wasteful.
			// See https://gitlab.com/IvanSanchez/gleo/-/issues/112
			// ev.value = this.getFieldValueAt(ev.canvasX, ev.canvasY);

			let value;
			const getValue = function getValue() {
				if (value) {
					return value;
				}
				// console.log(
				// 	"getting field value at dispatchPointerEvent",
				// 	this.constructor.name,
				// 	ev
				// );
				return (value = this.getFieldValueAt(ev.canvasX, ev.canvasY));
			}.bind(this);
			Object.defineProperty(ev, "value", { get: getValue });
		}

		this.#subAcetates.forEach((ac) => {
			ac.dispatchPointerEvent(ev, init);
		});

		return super.dispatchPointerEvent(ev, init);
	}
}

/**
 * @class ScalarField
 * @inherits Field
 * @relationship compositionOf Acetate, 1..1, 0..n
 *
 * Abstract 1-component field. Use `HeatMap` or `GreyscaleField` instead.
 *
 * Holds a `float32` scalar field as a platina-sized texture & framebuffer,
 * but lacks a shader program to interpret it.
 *
 */

export class ScalarField extends Field {
	constructor(
		target,
		{
			/**
			 * @option clearValue: Number = 0
			 * The value of the scalar field prior to rendering data on it. It should
			 * be zero for most cases.
			 */
			clearValue = 0,

			...opts
		} = {}
	) {
		const glii = "glii" in target ? target.glii : target;

		super(target, {
			...opts,

			glFormat: glii.gl.RED,
			glInternalFormat: glii.gl.R32F,
			clearValue: [clearValue, 0, 0, 0],
		});

		if (this.constructor.name === "ScalarField") {
			throw new Error(
				"Cannot instantiate ScalarField. Use a subclass instead, such as HeatMap or GreyscaleField"
			);
		}
	}

	getFieldValueAt(x, y) {
		return super.getFieldValueAt(x, y)[0];
	}
	static _nan = NaN;
}

/**
 * @class VectorField
 * @inherits ScalarField
 * @relationship compositionOf Acetate, 1..1, 0..n
 *
 * Abstract 2-component field. Use `ArrowHeadField` or `ParticleTrailSimulator` instead.
 *
 * Holds a `float32` vector field as a platina-sized texture & framebuffer,
 * but lacks a shader program to interpret it. The framebuffer format is
 * `RG32F`
 *
 */
export class VectorField extends Field {
	constructor(
		target,
		{
			/**
			 * @option clearValue: Array of Number = [0, 0]
			 * The value of the scalar field prior to render data on it. It should
			 * be [zero, zero] for most cases.
			 */
			clearValue = [0, 0],

			...opts
		} = {}
	) {
		const glii = "glii" in target ? target.glii : target;

		super(target, {
			...opts,

			glFormat: glii.gl.RG,
			glInternalFormat: glii.gl.RG32F,
			clearValue: [clearValue[0], clearValue[1], 0, 0],
		});

		if (this.constructor.name === "VectorField") {
			throw new Error("Cannot instantiate VectorField. Use a subclass instead");
		}
	}

	getFieldValueAt(x, y) {
		return super.getFieldValueAt(x, y).subarray(0, 2);
	}

	static _nan = [NaN, NaN];
}
