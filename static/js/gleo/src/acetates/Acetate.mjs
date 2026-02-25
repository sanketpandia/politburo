import ExpandBox from "../geometry/ExpandBox.mjs";
import Evented from "../dom/Evented.mjs";

import {
	multiply,
	fromTranslation,
	transpose,
	//str,
} from "../3rd-party/gl-matrix/mat3.mjs";
import { transformMat3 } from "../3rd-party/gl-matrix/vec3.mjs";
import GleoSymbol from "../symbols/Symbol.mjs";

/**
 * @class Acetate
 * @inherits Evented
 * @relationship associated ExpandBox
 *
 * The name `Acetate` refers to the nickname given to transparent foil sheets
 * used in old-school projectors.
 *
 * The end result for the user is a stack of acetates (after they pass through a
 * image composition keeping alpha, etc).
 *
 * In `Gleo`/`Glii` terms, an `Acetate` is a collection of:
 * - A `Framebuffer`
 *   - Including a `Texture` that can (and should/will) be used for composing
 * - A `WebGL1Program`
 * - An `AttributeSet`
 * - A `IndexBuffer`
 *
 * Typically, an `Acetate` will receive symbols, given as triangles together
 * with the neccesary vertex attributes, e.g.:
 * - triangulized polygons,
 * - extrudable line countours
 * - extrudable points
 *
 * Any given `Acetate` will draw symbols of the same "kind". Subclasses shall be
 * an acetate for solid-filled polygons, another for line contours, another for circles,
 * etc (as long as symbols of the same "kind" are to be rendered with the same
 * `WebGL1Program`).
 *
 */

export default class Acetate extends Evented {
	#framebuffer;
	// #depthAttachment;
	#outTexture;
	#glii;
	#zIndex;
	#attribution;

	// Whether this acetate should be redrawn
	#dirty = false;

	/**
	 * @constructor Acetate(target: GliiFactory, opts: Acetate Options)
	 * @alternative
	 * @constructor Acetate(target: Platina, opts: Acetate Options)
	 * @alternative
	 * @constructor Acetate(target: GleoMap, opts: Acetate Options)
	 */
	constructor(
		target,
		{
			/**
			 * @option queryable: Boolean = false
			 * If set to `true`, pointer events dispatched by this acetate will
			 * contain information about the colour of the underlying pixel. This
			 * can negatively impact performance.
			 */
			queryable = false,

			/**
			 * @option zIndex: Number = 0
			 * The relative position of this acetate in terms of compositing it
			 * with other acetates. Must be a 16-bit signed integer
			 * (an integer between -32786 and 32785).
			 */
			zIndex = 0,

			/**
			 * @option attribution: String = undefined
			 * The attribution for the acetate, if any. Attribution can be
			 * delegated to symbols or loaders.
			 */
			attribution = undefined,
		} = {}
	) {
		super();

		this.#zIndex = zIndex;
		this.#attribution = attribution;

		if ("glii" in target) {
			// First parameter is a Platina, a GleoMap, or an ScalarField
			this.#glii = target.glii;
			target.addAcetate(this);
		} else {
			this.#glii = target;
		}

		/**
		 * @property queryable: Boolean
		 * If set to `true`, pointer events dispathed by this acetate will
		 * contain information about the colour of the underlying pixel. This
		 * can negatively impact performance.
		 *
		 * Can be updated at runtime. The initial value is given by the
		 * homonymous option.
		 */
		this.queryable = queryable;

		/**
		 * @section Subclass interface
		 * @uninheritable
		 *
		 * These properties are meant for internal use of an `Acetate` subclass.
		 *
		 * @property _coords: SingleAttribute
		 * A Glii data structure holding `vec2`s, for vertex CRS coordinates.
		 */
		this._coords = new this.glii.SingleAttribute({
			size: 1,
			growFactor: 1.2,
			usage: this.glii.DYNAMIC_DRAW,
			glslType: "vec2",
			type: Float32Array,
		});

		// CRS that is being used.
		// This is automatically updated from `redraw()` calls.
		this._crs = undefined;

		// CRS that was previously used.
		// This is relevant for `.reproject()`/`.reprojectAll()` calls, to check
		// whether the reprojection is a CRS offset or a full-fledged reprojection
		// by comparing the names of the CRSs.
		this._oldCrs = undefined;

		// Expanding bounding box for the known `_coords`
		this.bbox = new ExpandBox();

		// An array of symbols known to this acetate. They are indexed by the base
		// *attribute* of each symbol.
		this._knownSymbols = [];

		// An instance of `Glii.MultiProgram`. Some subclasses of acetate will
		// implement several WebGL programs (notably, `AcetateInteractive`) and will
		// need to update their uniforms/attributes at once.
		this._programs = new this.glii.MultiProgram();

		// The loaders added directly to this acetate (care must be taken to
		// ensure that the symbols spawned by the loader match the acetate).
		this._loaders = new Set();

		// Stores attributions of contained symbols, to check for changes.
		this._attributions = new Set();
	}

	/**
	 * @section
	 * @property glii: GliiFactory
	 * The underlying Glii instance. Read-only.
	 */
	get glii() {
		return this.#glii;
	}

	get platina() {
		return this._platina ? this._platina : this._inAcetate?._platina;
	}

	/**
	 * @property attribution: String
	 * The attribution text for the acetate (for use with the `Attribution`
	 * control). Can not be updated (consider delegating attribution
	 * to symbols instead)
	 */
	get attribution() {
		return this.#attribution;
	}

	/**
	 * @section Static properties
	 * @property PostAcetate: undefined
	 * Static property (implemented in the `Acetate` class prototype, not the
	 * instances). For `Acetate`s that render RGBA8 textures, this is
	 * `undefined`. For acetates that render non-RGBA8 textures, this
	 * shall be an `Acetate` prototype that can post-process this into RGBA8.
	 */
	static get PostAcetate() {
		return undefined;
	}

	/**
	 * @section Subclass interface
	 * @uninheritable
	 *
	 * Subclasses of `Acetate` must provide/define the following methods:
	 *
	 * @method glProgramDefinition(): Object
	 * Returns a set of `WebGL1Program` options, that will be used to create the WebGL1
	 * program to be run at every refresh.
	 *
	 * Subclasses can rely on the parent class definition, and decorate it as needed.
	 *
	 * The shader code (for both the vertex and fragment shaders) must be split
	 * between `vertex`/`fragmentShaderSource` and `vertex`/`fragmentShaderMain`.
	 * Do not define `void main() {}` in the shader source; this is done automatically
	 * by wrapping `*ShaderMain`.
	 */
	glProgramDefinition() {
		// This GL program definition in the abstract acetate includes only stuff
		// common to *all* acetates.
		return {
			attributes: {
				aCoords: this._coords,
			},
			uniforms: {
				uTransformMatrix: "mat3",
			},
			vertexShaderSource: "",
			vertexShaderMain: "",
			fragmentShaderSource: "",
			fragmentShaderMain: "",
			target: this.#framebuffer,
		};
	}

	/**
	 * @section
	 * @method add(symbol: GleoSymbol): this
	 * Adds the given symbol to self. Typically this will imply a call to
	 * `allocate(symbol)`. However, for symbols with some async load (e.g. `Sprite`s
	 * and `ConformalRaster`s) the call to `allocate()` might happen at a later
	 * time.
	 */
	add(symbol) {
		if (symbol instanceof GleoSymbol) {
			return this.multiAdd([symbol]);
		} else {
			// Assume it's a loader, and has been called through `.addTo(acetate)`
			this._loaders.add(symbol);
		}
	}

	/**
	 * @method multiAdd(symbols: Array of GleoSymbol): this
	 * Add the given symbols to self (i.e. start drawing them).
	 *
	 * Since uploading data to the GPU is relatively costly, implementations
	 * should make an effort to pack all the symbols' data together, and
	 * make as few calls to the Glii buffers' `set()`/`multiSet()` methods.
	 *
	 * Subclasses must call the parent `multiAdd` to ensure firing the `symbolsadded`
	 * event.
	 */
	multiAdd(symbols) {
		symbols.forEach((sym) => {
			sym._inAcetate = this;
		});
		/**
		 * @event symbolsadded
		 * Fired whenever symbols are added to the acetate. Event details include
		 * such symbols.
		 */
		this.fire("symbolsadded", { symbols });
		return this;
	}

	/**
	 * @property symbols: Array of GleoSymbol
	 * The symbols being drawn on this acetate.
	 *
	 * This is a shallow copy of the internal structure holding the symbols, so
	 * any changes to it won't affect which symbols are being drawn. Read-only.
	 */
	get symbols() {
		return this._knownSymbols.filter((s) => !!s);
	}

	/**
	 * @method multiAdd(symbols: Array of GleoSymbol): this
	 * Add the given symbols to self (i.e. start drawing them).
	 *
	 * Since uploading data to the GPU is relatively costly, implementations
	 * should make an effort to pack all the symbols' data together, and
	 * make as few calls to the Glii buffers' `set()`/`multiSet()` methods.
	 *
	 * Subclasses must call the parent `multiAdd` to ensure firing the `symbolsadded`
	 * event.
	 */
	has(s) {
		return this._knownSymbols.includes(s) || this._loaders.has(s);
	}

	/**
	 * @section Subclass interface
	 * @method multiAllocate(symbols: Array of GleoSymbol): this
	 * Allocates GPU RAM for the symbol, and asks the symbol to fill up that
	 * slice of GPU RAM.
	 *
	 * Whenever possible, use `multiAllocate()` instead of multiple calls to `allocate()`.
	 * Adding *lots* of symbols in a loop might cause *lots* of WebGL calls, which
	 * will impact performance. By contrast, implementations of `allocate()` should be
	 * prepared to make as few WebGL calls as possible.
	 *
	 * Subclasses shall call `multiAllocate` from `multiAdd`, either synchronously
	 * or asynchronously.
	 */
	// Implemented in `AcetateVertices`, `AcetateDot` and `AcetateFill`, due
	// to needing different handling of dumping vertex data and triangle index
	// data.

	/**
	 * @section Subclass interface
	 * @uninheritable
	 *
	 * @method _getStridedArrays(maxVtx: Number, maxIdx: Number): undefined
	 * Must return a plain array with all the `StridedTypedArrays that a symbol
	 * might need, as well as any other (pseudo-)constants that the symbol's
	 * `_setGlobalStrides()` method might need.
	 *
	 * This must allocate memory for attributes and vertex indices, and so
	 * its parameters are the topmost vertex and index needed.
	 *
	 * @method _commitStridedArrays(baseVtx: Number, vtxCount: Number, baseIdx: Number, idxCount: Number): undefined
	 * Called when committing data to attribute buffers. The default commits
	 * data to `this._extrusions` and `this._attrs`, so there's no need to
	 * redefine this if only those attribute storages are used.
	 *
	 *
	 * @method _getGeometryStridedArrays()
	 * As `_getStridedArrays()`, but applies only to strided arrays that need to be
	 * updated whenever the geometry of a symbol changes.
	 *
	 * @method _commitGeometryStridedArrays()
	 * As per `_commitStridedArrays()`, but applies only to the strided arrays
	 * returned to `_getGeometryStridedArrays()`.
	 *
	 * @method _getPerPointStridedArrays()
	 * As `_getStridedArrays()`, but applies only to strided arrays that contain
	 * data that has to be updated on a per-geometry-point basis.
	 *
	 * @method _commitPerPointStridedArrays()
	 * As per `_commitStridedArrays()`, but applies only to the strided arrays
	 * returned to `_getPerPointStridedArrays()`.
	 *
	 * @method deallocate(symbol: GleoSymbol): this
	 * Deallocate resources for the given symbol (attributes, primitive indices).
	 * Since the primitive indices are deallocated, the symbol will not be drawn.
	 *
	 * Deallocating symbols involves *marking* their primitives as not being used,
	 * in the CPU side of things. Since there is no data to upload into GPU memory,
	 * implementations don't need to worry (much) about efficiency.
	 *
	 * Deallocation must also reset the references to the acetate, base vertex
	 * and base index to `undefined`.
	 *
	 * @method reprojectAll(): this
	 * Triggers a reprojection of all the coordinates of all vertices of all symbols in
	 * the acetate. Called when `this._crs` changes. `AcetateDot` and `AcetateVertices`
	 * provide implementations.
	 */
	reprojectAll() {
		this.bbox.reset();
		this.dirty = true;
		return this;
	}

	/**
	 * @method remove(symbol: GleoSymbol): this
	 * Removes the given symbol from self (stops drawning it)
	 */
	remove(symbol) {
		if (symbol instanceof GleoSymbol) {
			this.deallocate(symbol);

			this.fire("symbolsremoved", { symbols: [symbol] });
			this.dirty = true;
		} else {
			// Assume a Loader
			symbol.remove();
			this._loaders.delete(symbol);
		}

		return this;
	}

	/**
	 * @method multiRemove(symbols: Array of GleoSymbol): this
	 * Removes the given symbols from self (i.e. stops drawing it).
	 */
	multiRemove(symbols) {
		/// TODO: *should* clean up this.bbox
		/// Right now it only clears on CRS change
		/// TODO: Check for adjacent symbols in order to do
		/// less deallocation calls.

		// Mark all symbols as not belonging to any acetate. This is independent
		// of a symbol being allocated or not.
		// This also handles the edge case of adding and removing a symbol
		// (e.g. a `Sprite` that takes time to load due to network or text
		// rendering) before it has been allocated. An unallocated symbol
		// will have its `attrBase` as undefined, but needs to be marked
		// as not belonging to any acetate as well.
		symbols.forEach((s) => (s._inAcetate = undefined));

		// Filter out symbols with no `idxBase` or `attrBase` - these haven't
		// been fully loaded before being removed
		this.multiDeallocate(
			symbols.filter((s) => s.attrBase !== undefined && s.idxBase !== undefined)
		);

		/**
		 * @event symbolsremoved
		 * Fired whenever symbols are removed from the acetate. Event details include
		 * such symbols.
		 */
		this.fire("symbolsremoved", { symbols });
		this.dirty = true;
		return this;
	}

	/**
	 * @method empty(): this
	 * Removes all symbols currently in this acetate
	 */
	empty() {
		return this.multiRemove(this._knownSymbols);
	}

	/**
	 * @method destroy(): this
	 * Destroys all resources used by the acetate and detaches itself from the
	 * containing platina.
	 */
	destroy() {
		// No need to individually remove/deallocate symbols - just mark them
		// as not belonging to any acetate, and as unallocated.
		this._knownSymbols.forEach((s) => s.updateRefs(undefined, undefined, undefined));

		/// Subclasses that allocate extra resources (e.g. Stroke uses
		/// an extra attribute buffer) must destroy them as well.
		this._indices?.destroy();
		this._coords.destroy();
		this._attrs?.destroy();

		this._programs.destroy();

		const i = this.platina?._acetates.indexOf(this);
		if (i !== -1) {
			this.platina._acetates.splice(i, 1);
		}
		return this;
	}

	/**
	 * @section
	 * @method getColourAt(x: Number, y: Number): Array of Number
	 * Returns a 4-element array with the red, green, blue and alpha
	 * values of the pixel at the given coordinates.
	 * The coordinates are in CSS pixels, and relative to the upper-left
	 * corner of the acetate.
	 *
	 * Used internally during event handling, so that the event can provide
	 * the pixel colour at the coordinates of the pointer event.
	 *
	 * Returns `undefined` if the coordinates fall outside of the acetate.
	 */
	getColourAt(x, y) {
		if (!this.#framebuffer) {
			return undefined;
		}

		const h = this.#framebuffer.height;
		const w = this.#framebuffer.width;

		if (y < 0 || y > h || x < 0 || x > w) {
			return undefined;
		}
		const dpr = devicePixelRatio ?? 1;

		// Textures are inverted in the Y axis because WebGL shenanigans. I know.
		return this.#framebuffer.readPixels(dpr * x, h - dpr * y, 1, 1);
	}

	/**
	 * @section Internal Methods
	 * @uninheritable
	 * @method multiDeallocate(symbols: Array of GleoSymbol): this
	 * Deallocates all given symbols. Subclasses can (and should!)
	 * provide an alternative implementation that performs only
	 * one deallocation.
	 */
	multiDeallocate(symbols) {
		symbols.forEach(this.deallocate.bind(this));
		symbols.forEach((symbol) => {
			if (symbol.attrBase !== undefined) delete this._knownSymbols[symbol.attrBase];
		});

		// Check whether the array is composed of only empty slots
		// (this happens when delete()ing the last non-empty item)
		// The check is made via an iterator: the iterator won't run if there aren't
		// any non-empty items, even when the length of the array is non-zero
		if (!this._knownSymbols.some(() => true)) {
			// Reset the array so its length is zero (and acetates skip the draw calls)
			this._knownSymbols = [];
		}

		this.dirty = true;

		return this;
	}

	#programDef;

	/**
	 * @section Redrawing methods
	 *
	 * These methods control when the acetate updates its internal texture. They
	 * are meant to be called internally.
	 *
	 * @method resize(x: Number, y: Number): this
	 * Resizes the internal framebuffer to the given size (in device pixels).
	 */
	resize(x, y) {
		const glii = this.#glii;
		const opts = this.#programDef ?? (this.#programDef = this.glProgramDefinition());

		if (!this._inAcetate) {
			if (!this.#framebuffer) {
				//this.#outTexture && this.#outTexture.destroy();
				//this.#framebuffer && this.#framebuffer.destroy();
				this.#outTexture = new glii.Texture({
					format: glii.RGBA,
					internalFormat: glii.RGBA,
				});

				this.#framebuffer = new glii.FrameBuffer({
					color: [this.#outTexture],
					depth:
						opts.depth && opts.depth !== glii.NEVER
							? new glii.RenderBuffer({
									width: x,
									height: y,
									internalFormat:
										glii.gl instanceof WebGL2RenderingContext
											? glii.DEPTH_COMPONENT24
											: glii.DEPTH_COMPONENT16,
							  })
							: undefined,
					// stencil: renderbuffer,
					width: x,
					height: y,
				});
			} else {
				this.#framebuffer.resize(x, y);
			}
		} else {
			this.#framebuffer = this._inAcetate.framebuffer;
		}

		if (this._program) {
			this._program._target = this.#framebuffer;
		} else {
			opts.vertexShaderSource += opts.vertexShaderMain
				? `\nvoid main(){${opts.vertexShaderMain}}`
				: "";
			opts.fragmentShaderSource += opts.fragmentShaderMain
				? `\nvoid main(){${opts.fragmentShaderMain}}`
				: "";
			opts.target = this.#framebuffer;
			this._program = new glii.WebGL1Program(opts);
			this._programs.addProgram(this._program);
			/**
			 * @section
			 * @event programlinked: Event
			 * Fired when the GL program is ready (has been compiled and linked)
			 */
			this.dispatchEvent(new Event("programlinked"));
		}

		const depthClear =
			opts.depth === glii.LEQUAL || opts.depth === glii.LESS ? 1 : -1;

		this._clear =
			this.#outTexture &&
			new glii.WebGL1Clear({
				color: [255, 255, 255, 0],
				target: this.#framebuffer,
				// depth: 1,
				//depth: (this.zIndex << 15 )
				depth: depthClear,
			});

		this.dirty = true;
		return this;
	}

	/**
	 * @section Redrawing methods
	 * @method redraw(crs: BaseCRS, matrix: Array of Number, viewportBbox: ExpandBox): Boolean
	 * Low-level redraw of the `Acetate`.
	 *
	 * The passed `crs` ensures display in that CRS, since it's used to either
	 * - Check that all coordinate data in the `Acetate` is using that CRS, or
	 * - Reproject all coordinate data in the `Acetate` to match the CRS.
	 *
	 * The 9-element matrix is expected to be a 2D transformation matrix, which is
	 * then fed to the shader program as a `mat3`.
	 *
	 * Note that redrawing a single `Acetate` does **not** trigger a re-composition
	 * of all acetates, i.e. the redraw is not visible until re-composition happens.
	 *
	 * Returns `true` when the acetate has been redrawn, or `false` if a redraw
	 * is not deemed needed.
	 */
	redraw(crs, matrix, viewportBbox) {
		if (!this.dirty) {
			return false;
		}
		this.clear();
		this.#dirty = false;
		if (this._knownSymbols.length === 0) {
			return true;
		}

		if (this._crs !== crs) {
			this._oldCrs = this._crs || {};
			this._crs = crs;
			// console.log("Acetate ", this.constructor.name, "reprojecting data into CRS ", this._crs);
			this.bbox = new ExpandBox();

			/**
			 * @section Subclass interface
			 * @uninheritable
			 * Subclasses of `Acetate` must provide/define the following:
			 * @method reprojectAll(): undefined
			 * Must dump a new set of values to `this._coords`, based on the known
			 * set of symbols added to the acetate.
			 */
			this.reprojectAll();
		}

		let x1 = Math.ceil((viewportBbox.minX - this.bbox.maxX) / crs.wrapPeriodX);
		let x2 = Math.floor((viewportBbox.maxX - this.bbox.minX) / crs.wrapPeriodX);

		let y1 = Math.ceil((viewportBbox.minY - this.bbox.maxY) / crs.wrapPeriodY);
		let y2 = Math.floor((viewportBbox.maxY - this.bbox.minY) / crs.wrapPeriodY);

		// 		console.log(
		// 			"Drawing acetate", this.constructor.name,"; must check viewport wrapping. Data bounds/viewport:",
		// 			this.bbox,
		// 			viewportBbox
		// 		);
		// console.log("X-wrap:", x1, x2, "Y-wrap", y1, y2);

		if (!Number.isFinite(x1) || !Number.isFinite(x2)) {
			x1 = 0;
			x2 = 0;
		}
		if (!Number.isFinite(y1) || !Number.isFinite(y2)) {
			y1 = 0;
			y2 = 0;
		}

		if (x2 > x1 + 10) {
			console.warn(
				"Map repeats more than 10 times horizontally. Check your scale factor."
			);
			x2 = x1 + 10;
		}
		if (y2 > y1 + 10) {
			console.warn(
				"Map repeats more than 10 times horizontally. Check your scale factor."
			);
			y2 = y1 + 10;
		}

		// Transpose of the CRS matrix to apply `glmatrix` functionality.
		// Damn different notations.
		const origMatrix = transpose(new Array(9), matrix);

		// Copy of the map's crsMatrix, but without the translation. Will
		// be used for calculating the wrap offsets. Note glmatrix notation.
		// prettier-ignore
		const scaleRotationMatrix = [
			matrix[0], matrix[3], 0,
			matrix[1], matrix[4], 0,
			        0,         0, 1,
		];

		const offsetVector = new Array(3);
		const offsetMatrix = new Array(9);

		/// TODO: Leverage instanced rendering instead.
		/// TODO: Does setting a smaller viewport, or a scissor test,
		/// help performance in any way?
		for (let x = x1; x <= x2; x++) {
			for (let y = y1; y <= y2; y++) {
				let offsetX = crs.wrapPeriodX * x;
				let offsetY = crs.wrapPeriodY * y;

				if (!Number.isFinite(offsetX)) {
					offsetX = 0;
				}
				if (!Number.isFinite(offsetY)) {
					offsetY = 0;
				}

				offsetVector[0] = offsetX;
				offsetVector[1] = offsetY;
				offsetVector[2] = 0;
				transformMat3(offsetVector, offsetVector, scaleRotationMatrix);

				fromTranslation(offsetMatrix, offsetVector.slice(0, 2));
				multiply(offsetMatrix, offsetMatrix, origMatrix);

				transpose(offsetMatrix, offsetMatrix);

				this._programs.setUniform("uTransformMatrix", offsetMatrix);
				this.runProgram();
			}
		}
		return true;
	}

	/**
	 * @section Redrawing methods
	 *
	 * @method clear(): this
	 * Clears the acetate: sets all pixels to transparent black.
	 */
	clear() {
		this._clear?.run();
		return this;
	}

	/**
	 * @method rebuildShaderProgram(): this
	 * Deletes the current main WebGL shader program for the acetate, and
	 * replaces it with a freshly compiled one.
	 *
	 * This is meant for internal user of subclasses, whenever they need to update
	 * the shader, e.g. to change some of its constants.
	 */
	rebuildShaderProgram() {
		const opts = (this.#programDef = this.glProgramDefinition());

		opts.vertexShaderSource += opts.vertexShaderMain
			? `\nvoid main(){${opts.vertexShaderMain}}`
			: "";
		opts.fragmentShaderSource += opts.fragmentShaderMain
			? `\nvoid main(){${opts.fragmentShaderMain}}`
			: "";
		opts.target = this.#framebuffer;
		const newProgram = new this.glii.WebGL1Program(opts);
		this._programs.replaceProgram(this._program, newProgram);

		this._program = newProgram;
		this.dirty = true;

		this.dispatchEvent(new Event("programlinked"));
		return this;
	}

	/**
	 * @section Redrawing properties
	 * @property dirty: Boolean = false
	 * Whether this acetate should be rendered at the next frame. Can only be
	 * set to `true`; a call to `redraw()` will reset this to `false`.
	 */
	get dirty() {
		return this.#dirty;
	}
	set dirty(d) {
		this.#dirty ||= d;
		if (this._inAcetate) {
			this._inAcetate.dirty ||= d;
		}
	}

	/**
	 * @section Internal Methods
	 * @uninheritable
	 * Meant to be used only from `GleoMap`.
	 * @method asTexture(): Texture
	 * Returns a reference to the Glii `Texture` holding the visible results of this acetate.
	 */
	asTexture() {
		return this.#outTexture;
	}

	/**
	 * @method runProgram(): this
	 * Runs the GL program for this acetate. Might be overwritten by subclasses
	 * when partial runs are needed (e.g. to set per-symbol textures, or
	 * selecting a LoD).
	 */
	runProgram() {
		this._programs.run();
		return this;
	}

	/**
	 * @method multiSetCoords(start: Number, coordData: Array of Number): this
	 * Sets a section of the internal `_coords` `SingleAttributeBuffer`, and expands the
	 * bounding box of known coords to cover the new ones.
	 *
	 * The second `coordData` argument must be a *flattened* array of x-y coordinates,
	 * of the form `[x1, y1, x2, y2, x3, y3, .... xn, yn]`.
	 */
	multiSetCoords(start, coordData) {
		this._coords.multiSet(start, coordData);

		return this.expandBBox(coordData);
	}

	/**
	 * @method expandBBox(coordData: Array of Number): this
	 * Each acetate keeps a bounding box to keep track of the extents of drawable
	 * items (to calculate antimeridian repetition).
	 *
	 * This expects an argument in the form of `[x1, y1, x2, y2, x3, y3, .... xn, yn]`.
	 */
	expandBBox(coordData) {
		for (let i = 0, l = coordData.length; i < l; i += 2) {
			if (Number.isFinite(coordData[i]) && Number.isFinite(coordData[i + 1])) {
				this.bbox.expandXY(coordData[i], coordData[i + 1]);
			}
		}
		return this;
	}

	/**
	 * @method dispatchPointerEvent(ev:GleoPointerEvent): Boolean
	 * Stub for interactive acetate logic. Alias for `dispatchEvent`.
	 */
	dispatchPointerEvent(ev) {
		if (this.queryable) {
			// The current implementation will query the framebuffer/texture
			// when the expression is evaluated, which might be too late
			// specially if the event is logged into the console. The following
			// is the previous implementation, which is immediate but
			// potentially very wasteful.
			// See https://gitlab.com/IvanSanchez/gleo/-/issues/112
			// ev.colour = this.getColourAt(ev.canvasX, ev.canvasY);

			let colour;
			const getColour = function getColour() {
				if (colour) {
					return colour;
				}
				return (colour = this.getColourAt(ev.canvasX, ev.canvasY));
			}.bind(this);
			Object.defineProperty(ev, "colour", { get: getColour });
		}
		return this.dispatchEvent(ev);
	}

	/**
	 * @section Internal properties
	 * Meant to be used only from within a `Platina`.
	 * @property zIndex: Number; The value of the `zIndex` constructor option. Read-only.
	 */
	get zIndex() {
		return this.#zIndex;
	}

	/**
	 * @section Subclass interface
	 * @uninheritable
	 * @property framebuffer: Framebuffer
	 * The output Glii framebuffer for this acetate. Read-only.
	 */
	get framebuffer() {
		return this.#framebuffer;
	}
}
