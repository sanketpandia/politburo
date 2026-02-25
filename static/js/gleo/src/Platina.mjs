// import Glii from 'glii';
import Glii from "./glii/src/index.mjs";
// import epsg3857 from "./crs/epsg3857.mjs";
// import epsg4326 from "./crs/epsg4326.mjs";
import OffsetCRS from "./crs/OffsetCRS.mjs";
import Geometry from "./geometry/Geometry.mjs";
import Evented from "./dom/Evented.mjs";
import Loader from "./loaders/Loader.mjs";
// import GleoSymbol from "./symbols/Symbol.mjs";
import {
	invert,
	transpose,
	//fromTranslation,
	//scale as mat3Scale,
} from "./3rd-party/gl-matrix/mat3.mjs";
import { transformMat3 } from "./3rd-party/gl-matrix/vec3.mjs";
import { getMousePosition } from "./dom/Dom.mjs";
import GleoMouseEvent from "./dom/GleoMouseEvent.mjs";
import GleoPointerEvent from "./dom/GleoPointerEvent.mjs";
import ExpandBox from "./geometry/ExpandBox.mjs";
import { factory } from "./geometry/DefaultGeometry.mjs";
import parseColour from "./3rd-party/css-colour-parser.mjs";
import Acetate from "./acetates/Acetate.mjs";
import RawGeometry from "./geometry/RawGeometry.mjs";

const { log2, abs, max } = Math;

const pointerEvents = [
	"click",
	"dblclick",
	"auxclick",
	"contextmenu",
	"pointerover",
	"pointerenter",
	"pointerdown",
	"pointermove",
	"pointerup",
	"pointercancel",
	"pointerout",
	"pointerleave",
	"gotpointercapture",
	"lostpointercapture",
];

/**
 * @class Platina
 *
 * @inherits Evented
 * @relationship compositionOf Acetate, 1..1, 0..n
 * @relationship compositionOf Loader, 1..1, 0..n
 * @relationship compositionOf BaseCRS, 0..n, 1..1
 *
 * @relationship associated GleoPointerEvent, 1..1, 0..n
 * @relationship associated ExpandBox, 1..1, 0..n
 * @relationship associated dom
 *
 * In printing, a "platen" (or "platine" or "platina") is the glass flatbed
 * of a photocopier or scanner where pages are laid down, and in an
 * overhead projector, it's the glass flatbed where an acetate sheet is laid down.
 *
 * In Gleo, a `Platina` is the `<canvas>` where the map is shown (without any
 * map controls). The platina has a state similar to a map (center/scale/etc), and
 * when it changes it tells all acetates to redraw themselves, then flattens
 * all acetates together.
 *
 * A `Platina` boils down to:
 * - A collection of `Acetate`s, stacked and composable
 * - A `<canvas>` and its related WebGL context
 * - A view, with:
 *   - CRS (`BaseCRS`)
 *   - Center (`Geometry`)
 *   - Scale factor
 *   - Yaw rotation angle
 *
 * A `Platina` can ve used standalone to draw `GleoSymbol`s in `Acetate`s, but
 * does not offer interactivity (e.g. map drag, mousewheel zoom, etc); that is
 * left to `Actuator`s in a `GleoMap`.
 */

export default class Platina extends Evented {
	#glii;
	#precisionThreshold;

	#crs;
	#yaw = 0;
	#center;
	#scale;
	#map;
	#canvas;
	#target;
	#backgroundColour;
	#renderLoop;

	#resizable;
	#boundOnPointerEvent;

	#boundRedraw;
	#boundMultiAdd;
	#boundMultiRemove;

	#invalidViewWarningTimeout;

	/**
	 * @constructor Platina(canvas: HTMLCanvasElement, options?: Platina Options)
	 * @alternative
	 * @constructor Platina(canvasID: string, options?: Platina Options)
	 */
	/// TODO: Allow a WebGLRenderingContext. This is problematic for
	/// the ResizeObserver and the DOM events.
	constructor(
		canvas,
		{
			/**
			 * @section Platina Options
			 * @option resizable: Boolean = true
			 * Whether the map should react to changes in the size of its DOM
			 * container. Setting to `false` enables some memory optimizations.
			 */
			resizable = true,
			/**
			 * @option backgroundColour: Colour = [0, 0, 0, 0]
			 * Self-explanatory. The default transparent black should work for
			 * most use cases.
			 * @alternative
			 * @option backgroundColour: null
			 * If the background is explicitly set to `null`, then it won't
			 * be cleared between redraws. This might trigger an "infinite mirror"
			 * artifact if the canvas is not otherwise cleared between redraws.
			 */
			backgroundColour = [0, 0, 0, 0],

			/**
			 * @option preserveDrawingBuffer: Boolean = false
			 * Whether the rendering context created from the canvas shall
			 * be able to be read back. See the `preserveDrawingBuffer` option
			 * of [`HTMLCanvasElement.getContext()`](https://developer.mozilla.org/en-US/docs/Web/API/HTMLCanvasElement/getContext)
			 */
			preserveDrawingBuffer = false,

			/**
			 * @option precisionThreshold: Number = *
			 * The order of magnitude (in terms of significant bits, or base-2
			 * logarithm) that triggers a CRS offset.
			 *
			 * The default value depends on the floating point precision reported
			 * by the GPU, typically 22 (for GPUs which internally use `float32`)
			 * or 15 for older GPUs (which internally use `float24`).
			 *
			 * Raising this value may prevent spurious CRS offsets and *might*
			 * alleviate CRS-offset-related delays and artifacts, at the cost
			 * of possible precision artifacts. A value lower than the default
			 * has no positive effects.
			 */
			precisionThreshold = undefined,

			/**
			 * @option renderLoop: Boolean = true
			 * Enable/disable frame-by-frame renderloop. The default is to
			 * trigger a redraw call on every render frame. Disable this when
			 * manually trigering redraw calls.
			 */
			renderLoop = true,

			// Hidden option. Only used within GleoMap, and meant to let
			// symbols & acetates know what GleoMap instance they belong to,
			// if any.
			map = undefined,

			...options
		} = {}
	) {
		super();

		this.options = options;
		this.#backgroundColour = backgroundColour;

		this.#target =
			typeof canvas === "string" ? document.getElementById(canvas) : canvas;

		this.#map = map;

		const glii = (this.#glii = new Glii(this.#target, {
			//premultipliedAlpha: false,
			// premultipliedAlpha: true,
			depth: true,
			preserveDrawingBuffer,
			alpha: true,
		}));

		// Possibly extract the canvas from a WebGLRenderingContext, fall back to
		// "we have been given a canvas"
		this.#canvas = this.#target?.canvas ?? this.#target;

		if (!glii) {
			throw new Error(
				"No WebGL context: wrong canvas, or no WebGL support on this browser."
			);
		}

		this.#resizable = resizable;
		if (resizable) {
			glii.addEventListener("resized", this.#onResize.bind(this));
		}

		const gl = glii.gl;

		if (precisionThreshold) {
			this.#precisionThreshold = precisionThreshold;
		} else {
			this.#precisionThreshold =
				gl.getShaderPrecisionFormat(gl.VERTEX_SHADER, gl.HIGH_FLOAT).precision -
				1;
		}

		this._acetates = [];
		// Prepare data structs for acetate quads:
		// - Triangle indices (two per quad)
		// - Texture corners and z-index (as *static* attributes)
		// - CRS coords (as dynamic attributes)
		this._acetateQuads = new glii.TriangleIndices({
			size: 6,
			growFactor: 1,
		});
		this._acetateAttrs = new glii.InterleavedAttributes(
			{
				size: 4,
				growFactor: 1,
				usage: glii.STATIC_DRAW,
			},
			[
				{
					// z-index
					glslType: "float",
					type: Int16Array,
					normalized: true,
				},
				{
					// Texture coords
					glslType: "vec2",
					type: Int8Array,
				},
			]
		);
		this._acetateCoords = new glii.SingleAttribute({
			size: 4,
			growFactor: 1,
			usage: glii.DYNAMIC_DRAW,
			glslType: "vec2",
			type: Float32Array,
		});

		this._bbox = new ExpandBox();
		this.#onResize();

		this.rebuildCompositor();

		// Hook up event decorators
		/**
		 * @section Pointer events
		 *
		 * All [DOM `PointerEvent`s](https://developer.mozilla.org/en-US/docs/Web/API/Pointer_events)
		 * to a platina's `<canvas>` are handled by Gleo.
		 * Besides all of `PointerEvent`'s properties and methods, Gleo adds
		 * the `Geometry` corresponding to the pixel the event took place in.
		 *
		 * Most events are `GleoPointerEvent`s, but some browsers fire
		 * exclusively `MouseEvent`s for `click`/`auxclick`/`contextmenu`. In
		 * that case, expect a `GleoMouseEvent` instead.
		 *
		 * @event click: GleoPointerEvent
		 * Akin to the [DOM `click` event](https://developer.mozilla.org/en-US/docs/Web/API/Element/click_event)
		 * @event dblclick: GleoPointerEvent
		 * Akin to the [DOM `dblclick` event](https://developer.mozilla.org/en-US/docs/Web/API/Element/dblclick_event)
		 * @event auxclick: GleoPointerEvent
		 * Akin to the [DOM `auxclick` event](https://developer.mozilla.org/en-US/docs/Web/API/Element/auxclick_event)
		 * @event contextmenu: GleoPointerEvent
		 * Akin to the [DOM `contextmenu` event](https://developer.mozilla.org/en-US/docs/Web/API/Element/contextmenu_event)
		 * @event pointerover: GleoPointerEvent
		 * Akin to the [DOM `pointerover` event](https://developer.mozilla.org/en-US/docs/Web/API/HTMLElement/pointerover_event)
		 * @event pointerenter: GleoPointerEvent
		 * Akin to the [DOM `pointerenter` event](https://developer.mozilla.org/en-US/docs/Web/API/HTMLElement/pointerenter_event)
		 * @event pointerdown: GleoPointerEvent
		 * Akin to the [DOM `pointerdown` event](https://developer.mozilla.org/en-US/docs/Web/API/HTMLElement/pointerdown_event)
		 * @event pointermove: GleoPointerEvent
		 * Akin to the [DOM `pointermove` event](https://developer.mozilla.org/en-US/docs/Web/API/HTMLElement/pointermove_event)
		 * @event pointerup: GleoPointerEvent
		 * Akin to the [DOM `pointerup` event](https://developer.mozilla.org/en-US/docs/Web/API/HTMLElement/pointerup_event)
		 * @event pointercancel: GleoPointerEvent
		 * Akin to the [DOM `pointercancel` event](https://developer.mozilla.org/en-US/docs/Web/API/HTMLElement/pointercancel_event)
		 * @event pointerout: GleoPointerEvent
		 * Akin to the [DOM `pointerout` event](https://developer.mozilla.org/en-US/docs/Web/API/HTMLElement/pointerout_event)
		 * @event pointerleave: GleoPointerEvent
		 * Akin to the [DOM `pointerleave` event](https://developer.mozilla.org/en-US/docs/Web/API/HTMLElement/pointerleave_event)
		 * @event gotpointercapture: GleoPointerEvent
		 * Akin to the [DOM `gotpointercapture` event](https://developer.mozilla.org/en-US/docs/Web/API/HTMLElement/gotpointercapture_event)
		 * @event lostpointercapture: GleoPointerEvent
		 * Akin to the [DOM `lostpointercapture` event](https://developer.mozilla.org/en-US/docs/Web/API/HTMLElement/lostpointercapture_event)
		 */
		this.#boundOnPointerEvent = this._onPointerEvent.bind(this);
		for (let evName of pointerEvents) {
			this.#canvas.addEventListener(evName, this.#boundOnPointerEvent);
		}

		/**
		 * @section View initialization Options
		 *
		 * A `Platina` can take a set of [`SetView` options](#setview-options),
		 * just as the ones for a `setView` call.
		 *
		 * @option crs: BaseCRS = undefined
		 * Initial CRS of the platina.
		 * @option yawDegrees: Number = 0
		 * Initial yaw rotation of the platina, in clockwise degrees.
		 * @option yawRadians: Number = 0
		 * Initial yaw rotation of the platina, in counter-clockwise radians.
		 * @option center: Geometry = undefined
		 * Initial center of the platina.
		 * @option scale: Number = undefined
		 * Initial scale of the platina, in CRS units per CSS pixel.
		 * @option span: Number = undefined
		 * Initial span of the platina, in CRS units per diagonal.
		 */
		this.setView(options);

		this.#boundRedraw = this.redraw.bind(this);
		this.#boundMultiAdd = (ev) => {
			this.multiAdd(ev.detail.symbols);
		};
		this.#boundMultiRemove = (ev) => {
			this.multiRemove(ev.detail.symbols);
		};

		// Start the main render loop
		this.#renderLoop = renderLoop;
		if (renderLoop) {
			this.#queueRedraw();
		}

		// Queue a console warning message, for developers who forget to set the
		// platina's center/scale/CRS. 5 seconds should be a nice time.
		if (typeof window !== "undefined") {
			this.#invalidViewWarningTimeout = setTimeout(() => {
				console.warn("platina does not have crs+center+scale");
			}, 5000);
		}
	}

	/**
	 * @section
	 * @method destroy(): this
	 * Destroys the platina, freeing the rendering context. Should free all
	 * used GPU resources.
	 *
	 * No methods should be called on a destroyed platina.
	 */
	destroy() {
		cancelAnimationFrame(this.#animFrame);

		this._clear.run();

		this._acetateQuads.destroy();
		this._acetateAttrs.destroy();
		this._acetateCoords.destroy();
		this._compositor.destroy();

		this._acetates.forEach((ac) => ac.destroy());

		for (let evName of pointerEvents) {
			this.#canvas.removeEventListener(evName, this.#boundOnPointerEvent);
		}

		this.#canvas = undefined;
	}

	/**
	 * @section DOM properties
	 * @property canvas: HTMLCanvasElement
	 * The `<canvas>` element this platina is attached to. Read-only.
	 */
	get canvas() {
		return this.#canvas;
	}

	/**
	 * @section Internal methods
	 * @method addAcetate(ac: Acetate): this
	 * Adds a new `Acetate` to the map.
	 *
	 * There's no need to call this manually - acetates will be added to a
	 * `Platina` (or `GleoMap`) automatically then they're instantiated. Do
	 * remember to pass the `Platina` as the first parameter to the `Acetate`
	 * constructor.
	 */
	addAcetate(acetate) {
		if (this._acetates.includes(acetate)) {
			return;
		}

		// The resize() initialization of an acetate is delayed with a
		// setTimeout() because:
		// - This is called during the base acetate constructor
		// - Acetate subclass code can initialize stuff they need *after*
		//   this call
		// - Calling resize() without all data structures ready will throw errors
		// - Here is the place which causes the least distress
		// (Ideally this would use `setImmediate()` instead, if browsers had it).
		this.once("prerender", () => {
			const dpr = devicePixelRatio ?? 1;
			acetate.resize(
				this._pxWidth * dpr,
				this._pxHeight * dpr,
				this._pxWidth,
				this._pxHeight
			);
		});

		if (
			acetate.constructor.PostAcetate === undefined ||
			acetate.constructor.PostAcetate === Acetate
		) {
			/// RGBA acetate, add directly to self
			const zIndex = acetate.zIndex;
			let i = this._acetates.length * 4;
			let quad = new this._acetateQuads.Quad();
			quad.setVertices(i, i + 1, i + 2, i + 3);
			this._acetateAttrs.setFields(i, [[zIndex], [0, 0]]);
			this._acetateAttrs.setFields(i + 1, [[zIndex], [0, 1]]);
			this._acetateAttrs.setFields(i + 2, [[zIndex], [1, 1]]);
			this._acetateAttrs.setFields(i + 3, [[zIndex], [1, 0]]);

			this._acetates.push(acetate);
			/// TODO: Leftover initialization of acetates - fetch its `Texture`, maybe more?
			/// TODO: Match the acetate with its index, explicitly?
			/// TODO: Allow for RGBA acetates to not be bound to the Platina's
			/// render loop (when they're meant to be post-processed by another
			/// acetate)

			this._acetates.sort((a, b) => a.zIndex - b.zIndex);
		} else {
			// Search for a scalar field acetate (or similar) that can hold
			// this acetate.
			// console.warn("Non-RGBA8 acetate");

			let fitScalarField = this._acetates.filter(
				(candidate) => candidate instanceof acetate.constructor.PostAcetate
			)[0];

			if (fitScalarField) {
				fitScalarField.addAcetate(acetate);
			} else {
				new acetate.constructor.PostAcetate(this).addAcetate(acetate);
			}
		}

		acetate._map = acetate._platina = this;

		/// TODO: Save i,quad into acetate
		/// TODO: Method for removing an acetate (dealloc attribs from
		/// i, triangles from quad)

		/**
		 * @section Symbol/loader management events
		 * @event acetateadded
		 * Fired whenever an `Acetate` is added to the platina.
		 * @event symbolsadded
		 * Fired whenever symbols are added to any of the platina's acetates.
		 * @event symbolsremoved
		 * Fired whenever symbols are removed from any of th platina's acetates.
		 */
		this.fire("acetateadded", acetate);
		acetate.on("symbolsadded", (ev) => {
			this.fire("symbolsadded", ev.detail);
		});
		acetate.on("symbolsremoved", (ev) => {
			this.fire("symbolsremoved", ev.detail);
		});

		// this.#queueRedraw();
		return this;
	}

	/**
	 * @section
	 * @method getAcetateOfClass(proto: Prototype of Acetate): Acetate
	 * Given a specific `Acetate` class (e.g. `getAcetateOfClass(Sprite.Acetate)`),
	 * returns an acetate instance where that kind of symbol can be drawn.
	 * Will create an acetate of the given class if it doesn't exist in the map yet.
	 */
	getAcetateOfClass(acetateClass) {
		function recurse(ac) {
			return ac.subAcetates ? [ac, ...ac.subAcetates.map(recurse).flat()] : [ac];
		}
		let allAcetates = Array.from(this._acetates, recurse).flat();

		let ac = allAcetates.find(
			(a) => Object.getPrototypeOf(a).constructor === acetateClass
		);
		if (ac) {
			return ac;
		}

		ac = new acetateClass(this.#glii);
		this.addAcetate(ac);
		return ac;
	}

	#animFrame;

	#queueRedraw() {
		this.#animFrame ?? cancelAnimationFrame(this.#animFrame);

		if (this.#renderLoop) {
			this.#animFrame = requestAnimationFrame(this.#boundRedraw);
		}
		return this;
	}

	/**
	 * @method redraw(): this
	 * Redraws acetates that need to do so, and composes them together.
	 *
	 * There is no need to call this manually, since it will be called once per
	 * animation frame.
	 */
	redraw(timestamp) {
		if (!this.#canvas) {
			// Do not redraw if the platina has already been destroyed. Do not queue redraw.
			return;
		}

		if (!timestamp) {
			// This function is usually called with a timestamp, meaning
			// it's been called from a requestAnimationFrame().
			// If that's not the case, cancel anim frame to prevent race
			// conditions (queuing more than one call per frame)
			cancelAnimationFrame(this.#animFrame);
		}

		if (!this._bbox || !this._crsMatrix) {
			return this.#queueRedraw();
		}

		let updatedAcetates = 0;

		this.#glii.refreshDrawingBufferSize();

		/**
		 * @section Rendering events
		 * @event prerender
		 * Fired prior to performing a render (rendering `Acetate`s plus compositing them).
		 * Can be used to set the map's viewport during animations (as long as there's
		 * only one animation logic running).
		 */
		this.dispatchEvent(new Event("prerender"));

		/// Trigger a full redraw of all acetates
		/// TODO: Do not redraw all acetates all the times; limit one acetate per frame,
		/// and rely on the acetate quads and compositor.
		this._acetates.forEach((ac, i) => {
			if (ac.dirty && ac.redraw(this.#crs, this._crsMatrix, this._bbox)) {
				// Reset the CRS coordinates of the just-redrawn quad for this
				// acetate. This assumes the indices of those quads don't change.
				this._acetateCoords.multiSet(i * 4, this._viewportCorners);

				updatedAcetates++;
			}
		});

		if (updatedAcetates === 0) {
			return this.#queueRedraw();
		}

		// Compose all acetates.
		// Compositor uses a depth buffer, so acetates are composed in their given z-indexes.
		/// FIXME: That's not true. Debug, debug, debug.
		if (this.backgroundColour !== null) {
			this._clear.run();
		}
		this._compositor.setUniform("uTransformMatrix", this._crsMatrix);
		this._acetates.forEach((ac, i) => {
			// The compositor has to run once per acetate due to the
			// inability to choose a texture in the frag shader.
			// (Ideally composition should be just one draw call,
			// and z-composition would be done via the z coordinate of fragments)

			this._compositor.setTexture("uAcetateTex", ac.asTexture());
			this._compositor.runPartial(i * 6, 6);
		});

		/**
		 * @event render
		 * Fired just after performing a render (rendering `Acetate`s plus compositing them).
		 */
		this.dispatchEvent(new Event("render"));
		return this.#queueRedraw();
	}

	/**
	 * @section Internal methods
	 *
	 * @method rebuildCompositor()
	 * Rebuilds the WebGL program in charge of compositing the acetates.
	 *
	 * Should only be needed to run once.
	 *
	 * The compositor just dumps *one* texture from *one* acetate into the default renderbuffer
	 * (i.e. the target `<canvas>`). The "clear", then "bind texture"-"dump acetate" logic is
	 * implemented elsewhere.
	 *
	 */
	rebuildCompositor() {
		/// TODO: Somehow set the texture unit as a vertex attribute
		/// and have the frag shader map it to integer, choose the
		/// texture unit from there.
		/// https://stackoverflow.com/questions/51506704/webgl-pass-texture-from-vertex-shader-to-fragment-shader
		/// https://stackoverflow.com/questions/19592850/how-to-bind-an-array-of-textures-to-a-webgl-shader-uniform
		/// TODO: Handle the case of limited texture units!! (if a multi-texture
		/// compositor is viable)

		const glii = this.#glii;

		this._compositor = new glii.WebGL1Program({
			attributes: {
				aZIndex: this._acetateAttrs.getBindableAttribute(0),
				aUV: this._acetateAttrs.getBindableAttribute(1),
				aCoords: this._acetateCoords,
			},
			uniforms: { uTransformMatrix: "mat3" },
			textures: {
				// uAccumulator: this._accumulatorTexture,
				uAcetateTex: undefined,
			},
			vertexShaderSource: /* glsl */ `
				void main() {
					gl_Position = vec4(
						(vec3(aCoords, 1.0) * uTransformMatrix).xy,
						aZIndex + 0.5,
						1.0
					);
					vUV = aUV;
				}
			`,
			varyings: { vUV: "vec2" },
			fragmentShaderSource: /* glsl */ `
				void main() {
					// gl_FragColor = texture2D(uAcetateTex, vUV);
					vec4 texel = texture2D(uAcetateTex, vUV);
					//if (texel.a > 0.0) {
					gl_FragColor = texel;
					//}
					//gl_FragColor.r += gl_FragCoord.z;
				}
			`,
			indexBuffer: this._acetateQuads,
			depth: this.#glii.LOWER,
			blend: {
				equationRGB: glii.FUNC_ADD,
				equationAlpha: glii.FUNC_ADD,

				srcRGB: glii.SRC_ALPHA,
				dstRGB: glii.ONE_MINUS_SRC_ALPHA,
				srcAlpha: glii.ONE,
				dstAlpha: glii.ONE_MINUS_SRC_ALPHA,
			},
		});

		this.backgroundColour = this.#backgroundColour;
	}

	/**
	 * @section View setters
	 * @method setView(opts: SetView Options): this
	 *
	 * (Re-)sets the platina view to the given center/crs, scale, and yaw.
	 *
	 * Can trigger a redraw. Changes to the view state (center/crs/scale/yaw)
	 * are atomic.
	 */
	setView({
		/**
		 * @miniclass SetView Options (Platina)
		 * @section
		 * Calls to the `setView` method (of `GleoMap` and `Platina`) take an object
		 * with any of the following properties. e.g.:
		 *
		 * ```
		 * map.setView({ center: [ 100, 10 ], redraw: false });
		 * map.setView({ scale: 1500, yawDegrees: 90 });
		 * ```
		 *
		 * @option center: RawGeometry = undefined
		 * The desired map center, as an instantiated `RawGeometry`/`Geometry`.
		 * @alternative
		 * @option center: Array of Number = undefined
		 * The desired map center, as an `Array` of `Number`s. They will be
		 * converted into a `Geometry` by means of `DefaultGeometry`.
		 *
		 * @option scale: Number = undefined
		 * The desired map scale (**in CRS units per CSS pixel**). Mutually exclusive
		 * with `span`.
		 *
		 * @option span: Number = undefined
		 * The desired span of the map (in **CRS units** on the **diagonal of the viewport**).
		 * Mutually exclusive with `scale`.
		 *
		 * @option yawDegrees: Number = 0
		 * The desired yaw rotation, in degrees relative to "north up", clockwise.
		 * Mutually exclusive with `yawRadians`.
		 *
		 * @option yawRadians: Number = 0
		 * The desired yaw rotation, in radians relative to "north up", counterclockwise.
		 * Mutually exclusive with `yawDegrees`.
		 *
		 * @option crs: BaseCRS = undefined
		 * The desired CRS of the map.
		 **/
		center,
		crs,
		scale,
		span,
		yawDegrees,
		yawRadians,
	} = {}) {
		const [w, h] = this.pxSize;

		if (span && !scale) {
			/// TODO: Set some kind of flag, so that resizing the canvas
			/// will keep either the span or the scale.
			scale = span / Math.sqrt(w * w + h * h);
		}

		if (crs && crs !== this.#crs) {
			/**
			 * @class Platina
			 * @section View change events
			 * @event crschange
			 * Dispatched when the CRS changes explicitly (by setting the platina's
			 * CRS, or passing a `crs` option to a `setView` call)
			 */
			this.fire("crschange", {
				oldCRS: this.#crs,
				newCRS: crs,
			});
		}

		//const prevCRS = this.#crs;
		this.#crs = crs || this.#crs;
		this.#center = center ? factory(center) : this.#center;
		this.#scale = scale || this.#scale;

		// If platina is not fully initialized (crs + center + zoom),
		// fail silently
		if (
			this.#crs === undefined ||
			this.#center === undefined ||
			this.#scale === undefined
		) {
			return this;
		} else if (this.#invalidViewWarningTimeout) {
			clearTimeout(this.#invalidViewWarningTimeout);
			this.#invalidViewWarningTimeout = undefined;
		}

		if (yawRadians !== undefined) {
			this.#yaw = yawRadians;
		} else if (yawDegrees !== undefined) {
			this.#yaw = -yawDegrees * (Math.PI / 180);
		}

		if (this.#center.crs !== this.#crs) {
			this.#center = this.#center.toCRS(this.#crs);
		}

		if (!isFinite(this.#scale)) {
			throw new Error("Scale must have a finite value");
		}
		if (this.#scale <= 0) {
			throw new Error("Scale must be a positive number");
		}

		// Cover edge cases where the center is very near to the CRS's
		// wrapping period.
		// 		center.xy[0] %= this.#crs.wrapPeriodX;
		// 		center.xy[1] %= this.#crs.wrapPeriodY;
		this.#center.coords = this.crs.wrap(this.#center.coords, [0, 0]);

		if (isNaN(this.#center.coords[0]) || isNaN(this.#center.coords[1])) {
			throw new Error("New center must be finite numbers");
		}

		/// Check if the center Coord can be represented with enough
		/// precision in its own CRS; as well as whether a coordinate
		/// one pixel away is still within precision ("the floating point
		/// precision is smaller than the coordinate delta between two
		/// adjacent pixels").
		/// If not, create a `OffsetCRS`.

		const log2scale = log2(this.#scale);
		const log2size = log2(max(h, w));
		const log2distance = log2(
			max(abs(this.#center.coords[0]), abs(this.#center.coords[1]))
		);

		if (
			Number.isFinite(log2distance) &&
			log2distance - log2scale /*+ log2size*/ > this.#precisionThreshold
		) {
			console.warn(
				"Requested CRS center and scale would cause floating point artifacts. An offset CRS shall be created."
			);
			console.log(
				"scale/size/center log2 / threshold",
				log2scale,
				log2size,
				log2distance,
				this.#precisionThreshold
			);

			// The new CRS offset is *absolute* to the base CRS, not relative to it.
			let newOffset = this.#crs.offsetToBase(this.#center.coords);

			const newCRS = new OffsetCRS(new Geometry(this.#crs, newOffset));

			/**
			 * @event crsoffset
			 * Dispatched when the CRS undergoes an implicit offset to avoid precision loss.
			 */
			this.fire("crsoffset", {
				oldCRS: this.#crs,
				newCRS: newCRS,
				offset: newOffset,
			});

			this.#crs = newCRS;
			this.#center = new Geometry(this.#crs, [0, 0]);
		}

		// console.log("Centers; ", center.xy, center.toCRS(epsg4326).xy, log2distance);

		let [cX, cY] = this.#center.coords;

		// Raster pixel fidelity, assuming raster coordinates are always aligned
		// to the [0,0] origin of CRS coordinates.
		// FIXME: Implement raster fidelity scales, and adjust to them instead
		// to always trusting the current scale.
		// TODO: Fidelity should also work on 90-degree rotations.
		if (this.#yaw > -1e-10 && this.#yaw < 1e-10) {
			const [crsOffsetX, crsOffsetY] = this.#crs.offset ? this.#crs.offset : [0, 0];
			cX = cX - (cX % this.#scale) - (crsOffsetX % this.#scale);
			if (w % 2) {
				cX += this.#scale / 2;
			}

			cY = cY - (cY % this.#scale) - (crsOffsetY % this.#scale);
			if (h % 2) {
				cY += this.#scale / 2;
			}
		}

		const sX = 2 / (w * this.#scale);
		const sY = 2 / (h * this.#scale);

		/// Yaw rotation
		const cosYaw = Math.cos(this.#yaw);
		const sinYaw = Math.sin(this.#yaw);

		// Scale, translate, rotate around point
		// See https://www.wolframalpha.com/input/?i2d=true&i=Composition%5C%2840%29+ScalingTransform%5C%2891%29%5C%2840%29%CF%87%5C%2844%29%CF%88%5C%2841%29%5C%2893%29%5C%2844%29+TranslationTransform%5C%2891%29%7B-x%2C-y%7D%5C%2893%29%5C%2844%29+RotationTransform%5C%2891%29alpha%5C%2844%29+%7Bx%2Cy%7D%5C%2893%29%5C%2841%29
		// prettier-ignore
		this._crsMatrix = [
			sX * cosYaw, -sX * sinYaw,  sX * cY * sinYaw - sX * cX * cosYaw,
			sY * sinYaw,  sY * cosYaw, -sY * cY * cosYaw - sY * cX * sinYaw,
			          0,            0,                                    1,
		];

		// Strip the offset from the CRS matrix. Used for the drag actuator and
		// the acetate wrapping.
		// prettier-ignore
		this._rotationScaleMatrix = [
			this._crsMatrix[0], this._crsMatrix[1], 0,
			this._crsMatrix[3], this._crsMatrix[4], 0,
			                 0,                  0, 1,
		];

		// Cache the boundaries of the visible bounds. This will be used for
		// re-setting the acetate's vertex coordinates.
		const invMatrix = invert(new Array(9), this._crsMatrix);

		// Damn glmatrix notation difference, again.
		transpose(invMatrix, invMatrix);

		const vec = [];
		this._bbox.reset();

		// prettier-ignore
		const corners = [
			[-1, -1, 1],
			[-1,  1, 1],
			[ 1,  1, 1],
			[ 1, -1, 1],
		].map(corner => transformMat3(vec, corner, invMatrix).slice(0,2));

		corners.forEach((corner) => this._bbox.expandPair(corner));
		this._viewportCorners = corners.flat();

		/**
		 * @event viewchanged
		 * Fired whenever the viewport changes - center, scale or yaw.
		 *
		 * Details inclide the center, scale, and the affine matrix for converting
		 * CRS coordinates into clipspace coordinates.
		 *
		 * This event might fire at every frame during interactions and animations.
		 */
		this.fire("viewchanged", {
			center: this.#center,
			scale: this.#scale,
			matrix: this._crsMatrix,
		});

		this._acetates.forEach((ac) => (ac.dirty = true));

		return this;
	}

	/**
	 * @section View setters
	 * @method fitBounds(bounds: Array of Number, opts?: SetView Options): this
	 * Sets the platina's center and scale so that the given bounds (given in
	 * `[minX, minY, maxX, maxY]` form, and in the platina's CRS) are fully
	 * visible.
	 *
	 * Any other given `SetView Options` will be merged with the calculated center&scale.
	 * @alternative
	 * @method fitBounds(bounds: ExpandBox, opts?: SetView Options): this
	 * Idem, but using an `ExpandBox` instead.
	 * @alternative
	 * @method fitBounds(bounds: RawGeometry, opts?: SetView Options): this
	 * Idem, but fitting to the bbox of a `Geometry` instead.
	 *
	 * This performs an implicit reprojection, so that it works as expected
	 * then the geometry's CRS is different than the platina's CRS.
	 */
	fitBounds(bounds, opts = {}) {
		/// FIXME: Currently sets the yaw to zero. Instead it should respect it.
		let minX, minY, maxX, maxY;
		if (bounds instanceof ExpandBox) {
			({ minX, minY, maxX, maxY } = bounds);
		} else if (bounds instanceof RawGeometry) {
			({ minX, minY, maxX, maxY } = bounds.toCRS(this.crs).bbox());
		} else {
			[minX, minY, maxX, maxY] = bounds;
		}

		const [w, h] = this.pxSize;

		const center = new Geometry(this.crs, [(minX + maxX) / 2, (minY + maxY) / 2]);
		const scale = Math.max((maxX - minX) / w, (maxY - minY) / h);
		return this.setView({
			...opts,
			center: center,
			scale: scale,
			yaw: 0,
		});
	}

	/**
	 * @method zoomInto(geometry: Geometry, scale: Number, opts?: SetView Options): this
	 * Performs a `setView` operation so that the given geometry stays at the
	 * same pixel.
	 *
	 * Meant for user interactions on the map, including double-clicking and
	 * zooming into clusters from a `Clusterer`. Akin to Leaflet's `zoomAround`.
	 */
	zoomInto(geometry, scale, opts = {}) {
		const [canvasX, canvasY] = this.geomToPx(factory(geometry));
		const [w, h] = this.pxSize;
		let clipX = (canvasX * 2) / w - 1;
		let clipY = (canvasY * -2) / h + 1;

		const scaleFactor = scale / this.scale;

		clipX -= clipX * scaleFactor;
		clipY -= clipY * scaleFactor;

		const vec = [clipX, clipY, 1];
		const invMatrix = invert(new Array(9), this._crsMatrix);
		transpose(invMatrix, invMatrix);
		transformMat3(vec, vec, invMatrix);

		const targetCenter = new Geometry(this.crs, [vec[0], vec[1]], { wrap: false });

		return this.setView({ ...opts, center: targetCenter, scale: scale });
	}

	/**
	 * @section View setter/getter properties
	 * These properties allow to fetch the state of the view then read, *and*
	 * modify it. Setting the value of any of these properties has the same
	 * effect as calling `setView()` with appropriate values.
	 *
	 * Setting a value does not guarantee that the final value will be the
	 * given one. For example, when setting the center and immediatly then
	 * querying the center, the actual center can be a reprojection of the
	 * given one.
	 *
	 * @property center: RawGeometry
	 * The center of the map, as a point geometry.
	 * @property scale: Number
	 * The scale, in CRS units per CSS pixel.
	 * @property span: Number
	 * The span, in CRS units per diagonal.
	 * @property crs: BaseCRS
	 * The CRS being used by the platina.
	 * @property yawDegrees: Number
	 * The yaw rotation angle, in clockwise degrees.
	 * @property yawRadians: Number
	 * The yaw rotation angle, in counter-clockwise radians.
	 * @property backgrounColour: Colour
	 * Self-explanatory
	 */
	get center() {
		return this.#center;
	}
	set center(c) {
		return this.setView({ center: c });
	}

	get scale() {
		return this.#scale;
	}
	set scale(s) {
		return this.setView({ scale: s });
	}

	get span() {
		const [w, h] = this.pxSize;
		return this.#scale * Math.sqrt(w * w + h * h);
	}
	set span(s) {
		return this.setView({ span: s });
	}

	get crs() {
		return this.#crs;
	}
	set crs(c) {
		return this.setView({ crs: c });
	}

	get yawRadians() {
		return this.#yaw;
	}
	get yawDegrees() {
		return (-this.#yaw * 180) / Math.PI;
	}
	set yawRadians(y) {
		return this.setView({ yawRadians: y });
	}
	set yawDegrees(y) {
		return this.setView({ yawDegrees: y });
	}

	get backgroundColour() {
		return this.#backgroundColour;
	}
	set backgroundColour(c) {
		if (c === null) {
			this._clear = {
				run: function run() {
					/*noop*/
				},
			};
		} else {
			this.#backgroundColour = parseColour(c).map((n) => n / 255);
			this._clear = new this.#glii.WebGL1Clear({
				color: this.#backgroundColour,
				// 			depth: -1,
			});
		}
	}

	/**
	 * @property bbox: ExpandBox
	 * A rectangular bounding box that completely covers the map
	 * viewport. This box is aligned to the CRS, not to the viewport. Setting its
	 * value is akin to running `fitBounds`.
	 */
	get bbox() {
		return this._bbox;
	}
	set bbox(b) {
		this.fitBounds(b);
	}

	/**
	 * @section View getter properties
	 * @property pxSize: Array of Number
	 * The size of the canvas, in CSS pixels, in `[width, height]` form. Read-only.
	 */
	get pxSize() {
		return [this._pxWidth, this._pxHeight];
	}

	/**
	 * @property deviceSize: Array of Number
	 * The size of the canvas, in device pixels, in `[width, height]` form. Read-only.
	 */
	get deviceSize() {
		return [this._devWidth, this._devHeight];
	}

	/**
	 * @section
	 * @property glii: GliiFactory
	 * The Glii instance used by the platina. Read-only.
	 */
	get glii() {
		return this.#glii;
	}

	/**
	 * @property glii: GleoMap
	 * The `GleoMap` instance used to spawn this platina. If the platina
	 * was created stand-alone, this will be `undefined` instead.
	 */
	get map() {
		return this.#map;
	}

	/**
	 * @property resizable: Boolean
	 * Whether the platina reacts to changes in its DOM container. Read-only.
	 */
	get resizable() {
		return this.#resizable;
	}

	/**
	 * @section Conversion methods
	 * @method pxToGeom(xy: Array of Number, wrap?: Boolean): Geometry
	 * Given a (CSS) pixel coordinate relative to the `<canvas>` of the map,
	 * in the form `[x, y]`, returns the point `Geometry` (in the map's CRS)
	 * which corresponds to that pixel, at the map's current center/scale.
	 *
	 * The resulting geometry will be wrapped by default. To avoid this,
	 * set `wrap` to `false`.
	 *
	 * This is akin to Leaflet's `containerPointToLatLng()`. Inverse of `geomToPx`.
	 */
	pxToGeom([x, y], wrap = true) {
		if (!this._crsMatrix) {
			// Edge case - pointer events before center/scale has been set.
			if (this.#crs) {
				return new Geometry(this.#crs, [NaN, NaN]);
			} else {
				return undefined;
			}
		}

		const dpr = devicePixelRatio ?? 1;
		const w = this._devWidth;
		const h = this._devHeight;
		// Convert the px to clipspace, then multiply by this._crsMatrix.
		//const [w, h] = this.#glii.refreshDrawingBufferSize();

		const clipX = (dpr * x * 2) / w - 1;
		const clipY = (dpr * y * -2) / h + 1;

		const vec = [clipX, clipY, 1];

		const invMatrix = invert(new Array(9), this._crsMatrix);

		if (!invMatrix) {
			// There's some NaN values somewhere
			debugger;
		}

		// The matrix transposition is needed only because gl-matrix's notation
		// is transposed relative to WebGL's (or glii's) notation.
		transpose(invMatrix, invMatrix);
		// 		transformMat3(vec, vec, this._invMatrix);
		transformMat3(vec, vec, invMatrix);
		return new Geometry(this.#crs, [vec[0], vec[1]], { wrap });
	}

	/**
	 * @method geomToPx(Geometry): Array of Number
	 * Given a point `Geometry`, returns the `[x, y]` coordinates of the (CSS)
	 * pixel relative to the `<canvas>` of the map corresponding to that geometry
	 * (for the map's current center/scale).
	 *
	 * This is akin to Leaflet's `latLngToContainerPoint()`. Inverse of `pxToGeom`.
	 */
	geomToPx(geom) {
		if (!this._crsMatrix) {
			// Edge case - pins before center/scale have been set
			return [NaN, NaN];
		}

		let projectedGeom = geom.toCRS(this.#crs);

		// Matrix transposition stuff because of
		// https://gitlab.com/IvanSanchez/gleo/-/issues/10
		const transMatrix = transpose(new Array(9), this._crsMatrix);
		const vec = [projectedGeom.coords[0], projectedGeom.coords[1], 1];
		transformMat3(vec, vec, transMatrix);

		const dpr = devicePixelRatio ?? 1;

		return [
			(vec[0] / 2 + 0.5) * this._pxWidth * dpr,
			(0.5 - vec[1] / 2) * this._pxHeight * dpr,
		];
	}

	#loaders = [];

	/**
	 * @section Symbol/Loader management
	 * @method add(symbol: GleoSymbol): this
	 * Adds the given `GleoSymbol` to the appropriate acetate.
	 *
	 * Users should note that repeated calls to `add()` are, in performance terms,
	 * **much worse** than a single call to `multiAdd()`. Try to avoid repeated calls
	 * to `add()` inside a loop.
	 *
	 * @alternative
	 * @method add(loader: Loader): this
	 * Attaches the given `Loader` to the map.
	 */
	add(symbol) {
		return this.multiAdd([symbol]);
	}

	/**
	 * @method multiAdd(symbols: Array of GleoSymbol): this
	 * Adds the given `GleoSymbol`s to the appropriate acetate(s).
	 * @alternative
	 * @method multiAdd(loaders: Arrary of Loader): this
	 * Adds the given `Loader`s to the platina.
	 */
	multiAdd(symbols) {
		const bins = new Map();

		// Just for MultiSymbol class
		symbols = symbols
			.map((s) => {
				if (s instanceof Loader) {
					this.#loaders.push(s);

					// Loaders need to trigger their functionality.
					// s.addTo(this);
					s._addToPlatina(this);
					s.on("symbolsadded", this.#boundMultiAdd);
					s.on("symbolsremoved", this.#boundMultiRemove);
					return []; // Skip from acetate-finding logic.
				} else if (s.symbols) {
					// MultiSymbol
					s.target = this;
					return s.symbols;
				} else {
					return [s];
				}
			})
			.flat();

		symbols.forEach((s) => {
			const ac = s.constructor.Acetate;
			if (ac) {
				const bin = bins.get(ac);
				if (bin) {
					bin.push(s);
				} else {
					bins.set(ac, [s]);
				}
			} else if (!!s.expand) {
				// Very very likely a Spider
				s.addTo(this);
			} else {
				debugger;
			}
		});

		for (let [ac, syms] of bins.entries()) {
			this.getAcetateOfClass(ac).multiAdd(syms);
		}
		return this;
	}

	/**
	 * @method remove(symbol: GleoSymbol): this
	 * Removes one symbol from this map.
	 */
	remove(symbol) {
		symbol.remove();
		if (symbol instanceof Loader) {
			this.#loaders = this.#loaders.filter((l) => l !== symbol);

			symbol.off("symbolsadded", this.#boundMultiAdd);
			symbol.off("symbolsremoved", this.#boundMultiRemove);
		}

		return this;
	}

	/**
	 * @method multiRemove(symbols: Array of GleoSymbol): this
	 * Removes several symbols from this map.
	 */
	multiRemove(symbols) {
		const bins = new Map();

		symbols
			.filter((s) => s instanceof Loader)
			.forEach((s) => {
				this.#loaders = this.#loaders.filter((l) => l !== s);

				s.off("symbolsadded", this.#boundMultiAdd);
				s.off("symbolsremoved", this.#boundMultiRemove);
			});

		// Just for MultiSymbol class
		symbols.forEach((s) => {
			if (s.symbols) {
				symbols = symbols.concat(s.symbols);
			}
		});

		/// Group by acetate, let the acetate do the multiRemove().
		symbols
			.filter((s) => s._inAcetate)
			.forEach((s) => {
				const ac = s._inAcetate;
				if (ac) {
					const bin = bins.get(ac);
					if (bin) {
						bin.push(s);
					} else {
						bins.set(ac, [s]);
					}
				}
			});

		// Handle Spiders
		symbols.filter((s) => !!s.expand).forEach((s) => s.remove());

		bins.forEach((symbols, acetate) => acetate.multiRemove(symbols));

		return this;
	}

	/**
	 * @method has(symbol: GleoSymbol): Boolean
	 * Returns `true` if this platina contains the given symbol, false otherwise.
	 * @alternative
	 * @method has(symbol: Loader): Boolean
	 * Returns `true` if this platina contains the given loader, false otherwise.
	 */
	has(s) {
		if (s instanceof Loader) {
			return this.#loaders.includes(s);
		} else {
			const matches = this._acetates.filter(
				(a) => a instanceof s.constructor.Acetate
			);
			return matches.some((a) => a.has(s));
		}
	}

	// Internal use only. Called by the resize observer; resizes the framebuffers of
	// each acetate and triggers a re-render.
	#onResize(ev) {
		let x_css, y_css, x_device, y_device;

		if (!this.#canvas) {
			return;
		}

		if (ev) {
			x_css = ev.detail.x_css;
			y_css = ev.detail.y_css;
			x_device = ev.detail.x_device;
			y_device = ev.detail.y_device;
		} else {
			let rect = this.#canvas.getClientRects && this.#canvas.getClientRects()[0];
			if (rect) {
				// Canvas is in the DOM, possibly with applied CSS
				x_css = rect.width;
				y_css = rect.height;
			} else if (this.#canvas.width) {
				// Canvas is *not* in the DOM, so trust its width/height
				/// FIXME: What if this.#canvas is a WebGLRenderingContext?
				x_css = this.#canvas.width;
				y_css = this.#canvas.height;
			} else if (this.#canvas.drawingBufferWidth) {
				x_css = this.#canvas.drawingBufferWidth;
				y_css = this.#canvas.drawingBufferHeight;
			}

			const dpr = devicePixelRatio ?? 1;
			x_device = x_css * dpr;
			y_device = y_css * dpr;
		}

		if (x_css === 0 || y_css === 0) {
			throw new Error("Map size is zero");
		}

		this._pxWidth = this.#canvas.width = x_css = Math.floor(x_css);
		this._pxHeight = this.#canvas.height = y_css = Math.floor(y_css);

		this._devWidth = x_device;
		this._devHeight = y_device;

		/**
		 * @event resize: Event
		 * Fired when the platina is resized. Detail
		 */
		this.fire("resize", {
			x_css,
			y_css,
			x_device,
			y_device,
		});

		this._acetates.forEach((ac) => {
			ac.resize(x_device, y_device, x_css, y_css);
		});

		if (this.#center && this.#scale) {
			this.setView({});
		}
	}

	/**
	 * @section Scale and pixel fidelity methods
	 * Several use cases call for re-using a set of scale values.
	 *
	 * In particular, raster symbols (including tiles) have a preferred (or
	 * set of preferred) scale factors to be shown as, so that they are shown at
	 * a 1:1 raster pixel / screen pixel ratio.
	 *
	 * A `Platina` does not enforce these scale values; the usual way to enforce
	 * them is by using a `ZoomYawSnapActuator`.
	 */
	#scaleStopsPerCRS = new Map();
	/**
	 * @method setScaleStop(crsName: String, scale: Number): this
	 * Sets a scale stop for the given CRS **name**.
	 */
	setScaleStop(crsName, scale) {
		if (!this.#scaleStopsPerCRS.has(crsName)) {
			this.#scaleStopsPerCRS.set(crsName, new Map());
		}
		const stops = this.#scaleStopsPerCRS.get(crsName);
		if (stops.has(scale)) {
			stops.set(scale, stops.get(scale) + 1);
		} else {
			stops.set(scale, 1);
		}
		return this;
	}

	/**
	 * @method removeScaleStop(crsName: String, scale: Number): this
	 * Reverse of `setScaleStop`.
	 */
	removeScaleStop(crsName, scale) {
		if (!this.#scaleStopsPerCRS.has(crsName)) {
			return this;
		}
		const stops = this.#scaleStopsPerCRS.get(crsName);
		if (!stops.has(scale)) {
			return this;
		} else {
			const usageCount = stops.get(scale);
			if (usageCount === 1) {
				stops.delete(scale);
			} else {
				stops.set(scale, usageCount - 1);
			}
		}
		return this;
	}

	/**
	 * @method getScaleStops(crsName): Array of Number
	 * Returns the scale stops for the given CRS name
	 */
	getScaleStops(crsName) {
		const crsStops = this.#scaleStopsPerCRS.get(crsName);
		return crsStops ? Array.from(this.#scaleStopsPerCRS.get(crsName).keys()) : [];
	}

	// Will hold the canvas-relative pixel coordinates of the last `pointerDown`
	// event, on a per-pointer basis.
	#onPointerDownCoords = {};

	// Internal use only. Decorates a DOM event, so that it has the CRS coordinates
	// in the event's properties.
	// It then dispatches on all acetates (so they can dispatch on the appropriate
	// symbol)
	// Additionally prevents dispatching a `click` event if there's been a drag.
	// Needs to *not* be private, for leaflet-gleo compatibility
	_onPointerEvent(ev) {
		if ("canvasX" in ev) {
			return;
		}
		const [canvasX, canvasY] = getMousePosition(ev, this.#canvas);
		const geometry = this.pxToGeom([canvasX, canvasY], false);

		// Prevent click on drag
		if (ev.type === "pointerdown") {
			this.#onPointerDownCoords[ev.pointerId] = [canvasX, canvasY];
		} else if (ev.type === "click" || ev.type === "auxclick") {
			if ("pointerId" in ev) {
				// Non-firefox branch: compare against the pointerdown coords
				// for the event's pointerId

				const [downX, downY] = this.#onPointerDownCoords[ev.pointerId];

				if (Math.abs(downX - canvasX) > 3 || Math.abs(downY - canvasY) > 3) {
					// The click happened far away from the pointerdown, ignore this click
					return;
				}
			} else {
				// Firefox branch: compare against *all* pointerdown canvas coords
				if (
					Object.values(this.#onPointerDownCoords).every(
						([downX, downY]) =>
							Math.abs(downX - canvasX) > 3 || Math.abs(downY - canvasY) > 3
					)
				) {
					return;
				}
			}
		}

		// Properties of MouseEvents/PointerEvents are not enumerable, so doing {...ev} is
		// unfortunately not an option.
		let init = {
			// EventInit
			bubbles: ev.bubbles,
			cancelable: ev.cancelable,
			composed: ev.composed,
			// 			target: this,

			// UIEventInit
			detail: ev.detail,
			view: ev.view,

			// mouseEventInit
			screenX: ev.screenX,
			screenY: ev.screenY,
			clientX: ev.clientX,
			clientY: ev.clientY,
			ctrlKey: ev.ctrlKey,
			shiftKey: ev.shiftKey,
			altKey: ev.altKey,
			metaKey: ev.metaKey,
			button: ev.button,
			buttons: ev.buttons,
			relatedTarget: ev.relatedTarget,
			region: ev.region,

			// GleoPointerEvent
			geometry,
			canvasX,
			canvasY,
		};

		if (ev instanceof PointerEvent) {
			// i.e. do not add these properties to the `click`/`auxclick`/
			// `contextmenu` `MouseEvent`s, even though they *should* be
			// `PointerEvent`s as per https://w3c.github.io/pointerevents/#the-click-auxclick-and-contextmenu-events
			// (These events are `MouseEvent`s in Firefox; Chrom[ium|e] correctly
			// dispatches `PointerEvent`s).
			init = {
				...init,
				pointerId: ev.pointerId,
				width: ev.width,
				height: ev.height,
				pressure: ev.pressure,
				tangentialPressure: ev.tangentialPressure,
				tiltX: ev.tiltX,
				tiltY: ev.tiltY,
				twist: ev.twist,
				pointerType: ev.pointerType,
				isPrimary: ev.isPrimary,
			};
		}

		// Create a new synthetic pointer event with the data from the real one,
		// so that it can be dispatched again with the decorated detail.
		const EventProto = ev instanceof PointerEvent ? GleoPointerEvent : GleoMouseEvent;
		const decoratedEvent = new EventProto(ev.type, init);

		let canDefault = true;
		this._acetates.every((ac) => {
			if (decoratedEvent._canPropagate) {
				canDefault &= ac.dispatchPointerEvent(decoratedEvent, init);
				return decoratedEvent._canPropagate;
			} else {
				return true;
			}
		});

		if (decoratedEvent._canPropagate) {
			canDefault &= this.dispatchEvent(decoratedEvent);
		}

		if (!decoratedEvent._canPropagate) {
			ev.stopPropagation();
		}
		if (!canDefault) {
			ev.preventDefault();
		}
		return !decoratedEvent._canPropagate;
	}

	#cursorQueue = [];
	/**
	 * @section Internal methods
	 * @method queueCursor(cursor: String): this
	 * Called when hovering over an interactive symbol with a `cursor`; adds the
	 * given cursor to an internal queue. If there's only one cursor in the queue
	 * the CSS property of the platina's `<canvas>` will be set to it.
	 */
	queueCursor(cursor) {
		/*
		 * The reason for having a queue is that there might be several
		 * interactive acetates in the platina, and therefore several
		 * interactive overlapping symbols in any given pixel. The way that
		 * Gleo event handling works means that the pointer will fire
		 * `pointerover` and `pointerout` events in all of those overlapping
		 * symbols, with possibly conflicting cursors.
		 *
		 * Having a queue might not be the best way - maybe there's a need for
		 * a map of acetate to cursor, ordered by z-index. Race conditions in
		 * symbol cursors shouldn't be too problematic, though.
		 */
		if (this.#cursorQueue.length === 0) {
			this.canvas.style.cursor = cursor;
		}
		this.#cursorQueue.push();
	}

	/**
	 * @method unqueueCursor(cursor: String): this
	 * Called when unhovering out of an interactive symbol with a `cursor`; removes
	 * the given cursor from an internal queue. Resets the `cursor` CSS property
	 * of the platina's `<canvas>` to next item in that queue, or unsets it if the
	 * stack is empty.
	 */
	unqueueCursor(cursor) {
		this.#cursorQueue.splice(this.#cursorQueue.indexOf(cursor), 1);
		this.canvas.style.cursor =
			this.#cursorQueue.length === 0 ? "" : this.#cursorQueue[0];
	}
}
