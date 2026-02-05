import AcetateVertices from "./AcetateVertices.mjs";
import GleoMouseEvent from "../dom/GleoMouseEvent.mjs";
import GleoPointerEvent from "../dom/GleoPointerEvent.mjs";

/// TODO: Turn this into a mixin. There might be a need for interactive `AcetateDot`s.

/**
 * @class AcetateInteractive
 * @inherits AcetateVertices
 *
 * @relationship associated GleoEvent, 0..1, 0..n
 *
 * An `Acetate` that renders its symbols into an invisible framebuffer
 * so pointer events can query it to know the symbol that has been rendered
 * in a given pixel.
 */
export default class AcetateInteractive extends AcetateVertices {
	constructor(
		target,
		{
			/**
			 * @section AcetateInteractive Options
			 * @option interactive: Boolean = true
			 * When set to `false`, disables all interactivity of symbols in this
			 * acetate, regardless of the symgols' settings. *Should* improve
			 * performance a bit when off.
			 */
			interactive = true,

			/**
			 * @option pointerTolerance: Number = 3
			 * The distance, in CSS pixels, that the pointer can be away from
			 * a symbol to trigger a pointer event.
			 *
			 * (This is achieved internally by extruding vertices this
			 * extra amount; it does **not** perfectly buffer the visible
			 * symbol, but rather makes the clickable triangles slightly larger
			 * than the visible triangles)
			 *
			 * (This assumes that the sprite image somehow fills its space;
			 * transparent regions in the sprite image will shift and may behave
			 * unintuitively; a transparent border will effectively lower the
			 * extra tolerance)
			 */
			pointerTolerance = 3,

			...opts
		} = {}
	) {
		super(target, opts);

		if (interactive) {
			this.#isInteractive = true;
			this.#pointerTolerance = pointerTolerance;
			try {
				if (!(this.glii.gl instanceof WebGL2RenderingContext)) {
					// This enables the *creation* of floating point textures
					this.glii.loadExtension("OES_texture_float");
				}
				// This enables *rendering* to a floating point texture
				this.glii.loadExtension("EXT_color_buffer_float");

				// This enables *overlapping* triangles on the floating point texture
				this.glii.loadExtension("EXT_float_blend");

				this.#webgl2 = true;
			} catch (ex) {
				this.#webgl2 = false;
			}

			if (this.#webgl2) {
				// The default is to use R32F textures, which needs WebGL2 or a
				// bunch of extensions.
				this.#ids = new this.glii.SingleAttribute({
					size: 1,
					growFactor: 1.2,
					usage: this.glii.STATIC_DRAW,

					glslType: "float",
					type: Float32Array,
					normalized: true,
				});
			} else {
				// IDs are always integers, and are stored as 1-byte vec4s.
				// This is done in order to simplify the GL workflow: when rendering
				// into a texture (into a framebuffer with a texture as its 0th
				// colour attachment), the texture must be RGBA/UNSIGNED_BYTE.
				// In order to avoid as many calculations inside the shaders,
				// the JS layer will hack int32s into int24s into 3x int8s
				// into 3x 8-bit RGBA, the shaders will only handle 8-bit vec3s,
				// filling the alpha byte to 0xff; reading
				// from the framebuffer will return 4x 8-bit RGBA, and the JS
				// layer will glue together the int24 from that.
				this.#ids = new this.glii.SingleAttribute({
					size: 1,
					growFactor: 1.2,
					usage: this.glii.STATIC_DRAW,

					glslType: "vec4",
					type: Uint8Array,
					normalized: true,
				});
			}

			// The internal `Map` from numerical ID to `GleoSymbol` instance.
			this.#idMap = new Map();

			this.#nextSymbolId = 1;
		} else {
			this.#isInteractive = false;
		}
	}

	#isInteractive;
	#pointerTolerance;
	#webgl2;
	#ids;
	#idMap;
	#nextSymbolId;

	#idsTexture;
	#idsFramebuffer;
	#idProgram;

	// Needed for setPointerCapture/releasePointerCapture functionality.
	#pointerCaptureMap = new Map();

	/**
	 * @property isInteractive: Boolean
	 * Whether the acetate has interactivty (pointer events) enabled. Read-only.
	 */
	get isInteractive() {
		return this.#isInteractive;
	}

	resize(x, y) {
		super.resize(x, y);
		if (!this.#isInteractive) {
			return this;
		}

		const glii = this.glii;
		const opts = this.glIdProgramDefinition();

		if (!this.#idsFramebuffer) {
			// 		this.#idsTexture && this.#idsTexture.destroy();
			// 		this.#idsFramebuffer && this.#idsFramebuffer.destroy();
			if (this.#webgl2) {
				// R32F texture
				this.#idsTexture = new glii.Texture({
					format: glii.gl.RED,
					internalFormat: glii.gl.R32F,
					type: glii.FLOAT,
				});
			} else {
				// Default RGBA8 output texture
				this.#idsTexture = new glii.Texture({});
			}

			this.#idsFramebuffer = new glii.FrameBuffer({
				color: [this.#idsTexture],
				depth: new glii.RenderBuffer({
					width: x,
					height: y,
					internalFormat: glii.DEPTH_COMPONENT16,
				}),
				width: x,
				height: y,
			});
		} else {
			this.#idsFramebuffer.resize(x, y);
		}

		if (this.#idProgram) {
			this.#idProgram.setTarget(this.#idsFramebuffer);
		} else {
			opts.fragmentShaderSource += opts.fragmentShaderMain
				? `\nvoid main(){${opts.fragmentShaderMain}}`
				: "";
			opts.target = this.#idsFramebuffer;
			this.#idProgram = new glii.WebGL1Program(opts);

			this._programs.addProgram(this.#idProgram);
		}

		// Assuming that a resize means that the devicePixelRatio
		// might have changed. This is common in desktop browsers with
		// Ctrl+'+' / Ctrl+'-'
		this.#idProgram.setUniform(
			"uPointerTolerance",
			this.#pointerTolerance * (devicePixelRatio ?? 1)
		);

		const depthClear =
			opts.depth === glii.LEQUAL || opts.depth === glii.LESS ? 1 : -1;

		this._idClear = new glii.WebGL1Clear({
			color: [0, 0, 0, 0],
			// color: [1, 1, 1, 1],
			target: this.#idsFramebuffer,
			// depth: 1 << 15,
			depth: depthClear,
			// depth: 0.6,
		});

		return this;
	}

	/**
	 * @property idsTexture: Texture
	 * Read-only accessor to the Glii `Texture` holding the internal IDs of
	 * interactive symbols.
	 */
	get idsTexture() {
		return this.#idsTexture;
	}

	/**
	 * @section Subclass interface
	 * @uninheritable
	 *
	 * Subclasses of `Acetate` can provide/define the following methods:
	 *
	 * @method glIdProgramDefinition(): Object
	 * Returns a set of `WebGL1Program` options, that will be used to create the WebGL1
	 * program to be run at every refresh to create the texture containing the symbol IDs.
	 * This is needed for the `getSymbolAt()` functionality to work.
	 *
	 * By default, the non-interactive program (i.e. the result of `glProgramDefinition`)
	 * is reused. The ID attribute `aId` is added, a new varying `vId` is added,
	 * the vertex shader is modified to dump the new attribute into the new varying,
	 * and the fragment shader is replaced.
	 *
	 * The default interactive fragment shader dumps `vId` to the RGB component and
	 * sets the alpha value. It looks like:
	 * ```glsl
	 * void main() {
	 * 	gl_FragColor.rgb = vId;
	 * 	gl_FragColor.a = 1.0;
	 * }`
	 * ```
	 *
	 * Subclasses can redefine the fragment shader source if desired; a use case is to apply
	 * masking so that transparent fragments of the `GleoSymbol` are `discard`ed
	 * within the interactive fragment shader (see the implementation of `AcetateSprite`).
	 * It's recommended that subclasses don't change any other bits of this program definition.
	 */
	glIdProgramDefinition() {
		const def = this.glProgramDefinition();

		/// TODO: Modify this shader so that if aID.a is (lower than) zero, then
		/// the vertex's gl_Position is (0,0,0,0) and no fragments are spawned.

		const regexpExtrude = /(\W)aExtrude(\W)/g;
		const replacementExtrude = function replacement(_, pre, post) {
			return `${pre}(aExtrude + uPointerTolerance * sign(aExtrude))${post}`;
		};

		return {
			...def,
			attributes: {
				aId: this.#ids,
				...def.attributes,
			},
			uniforms: {
				uPointerTolerance: "float",
				...def.uniforms,
			},
			vertexShaderSource:
				def.vertexShaderSource.replace(regexpExtrude, replacementExtrude) +
				(this.#webgl2
					? `
				void main() {
					// if (aId > 0.0) {
						vId = aId;
						${def.vertexShaderMain}
					// }
				}`
					: `
				void main() {
					if (aId.a > 0.0) {
						vId = aId;
						${def.vertexShaderMain}
					}
				}`),
			varyings: {
				vId: this.#webgl2 ? "float" : "vec4",
				...def.varyings,
			},
			fragmentShaderMain: this.#webgl2
				? `gl_FragColor.r = vId;`
				: `gl_FragColor = vId;`,
			// depth: this.glii.LESS,
			// depth: this.glii.GREATER,
			unusedWarning: false,
			blend: {
				equationRGB: this.glii.FUNC_ADD,
				equationAlpha: this.glii.FUNC_ADD,

				srcRGB: this.glii.ONE,
				srcAlpha: this.glii.ONE,
				dstRGB: this.glii.ZERO,
				dstAlpha: this.glii.ZERO,
			},
		};
	}

	/**
	 * @method multiAddIds(symbols: Array of GleoSymbol, baseVtx: Number, maxVtx?: Number): this
	 * Given an array of symbols (as per `multiAdd`) and a base vertex index,
	 * assigns a numerical ID to each symbol (unique in the acetate), packs
	 * the data, and updates the corresponding attribute buffer.
	 *
	 * This is meant to be called from within the `multiAdd()` method of subclasses.
	 */
	multiAddIds(symbols, baseVtx, maxVtx) {
		if (!this.#isInteractive) {
			return this;
		}

		if (!maxVtx) {
			maxVtx = 0;
			symbols.forEach(
				(s) => (maxVtx = Math.max(maxVtx, s.attrBase + s.attrLength))
			);
		}

		/// TODO: Use a Glii `Allocator` instead of an incrementing counter.
		/// There is no foreseeable case where 2^24 (16m777k216) symbols will
		/// be added to an acetate, though.

		// const strideIds = this.#ids.asStridedArray(baseVtx + symbols.length);
		const strideIds = this.#ids.asStridedArray(maxVtx);

		if (this.#webgl2) {
			symbols.forEach((s) => {
				if (s.interactive) {
					const id = s._id ?? (this.#nextSymbolId += 1);
					this.#idMap.set(id, s);
					s._id = id;
					for (
						let i = s.attrBase, end = s.attrBase + s.attrLength;
						i < end;
						i++
					) {
						strideIds.set([id], i);
					}
				}
			});
		} else {
			let packedId = new Uint8Array(4);
			symbols.forEach((s) => {
				if (s.interactive) {
					const id = s._id ?? (this.#nextSymbolId += 1);
					s._id = id;
					this.#idMap.set(id, s);
					// packedId[0] = id & 0x3f;
					// packedId[1] = (id >> 6) & 0x3f;
					// packedId[2] = (id >> 12) & 0x3f;
					packedId[0] = id & 0xff;
					packedId[1] = (id & 0xff00) >> 8;
					packedId[2] = (id & 0xff0000) >> 16;
					packedId[3] = 0xff;
				} else {
					packedId.fill(0);
				}
				for (let i = s.attrBase, end = s.attrBase + s.attrLength; i < end; i++) {
					strideIds.set(packedId, i);
				}
			});
		}
		this.#ids.commit(baseVtx, maxVtx - baseVtx);

		return this;
	}

	/**
	 * @method deallocate(symbol: GleoSymbol): this
	 * Deallocates the symbol from this acetate (so it's not drawn on the next refresh),
	 * and removes the reference from its numerical ID.
	 */
	remove(symbol) {
		if (!this.#isInteractive) {
			return super.remove(symbol);
		}
		if (symbol === this.#hoveredSymbol && symbol.cursor) {
			this.platina.unqueueCursor(symbol.cursor);
			this.#hoveredSymbol = undefined;
		}
		this.#idMap.delete(symbol._id);
		symbol._id = undefined;
		return super.remove(symbol);
	}

	clear() {
		if (this.#isInteractive) {
			this._idClear?.run();
		}
		return super.clear();
	}

	/**
	 * @section Internal Methods
	 * @uninheritable
	 *
	 * @method getSymbolAt(x: Number, y: Number): GleoSymbol
	 * Given the (CSS) pixel coordinates of a point (relative to the upper-left corner of
	 * the `GleoMap`'s `<canvas>`), returns the `GleoSymbol` that has been drawn
	 * at that pixel.
	 *
	 * Returns `undefined` if there is no `GleoSymbol` being drawn at the given
	 * pixel.
	 */
	getSymbolAt(x, y) {
		if (!this.#isInteractive || !this.#idsFramebuffer) {
			return undefined;
		}

		// Textures are inverted in the Y axis because WebGL shenanigans. I know.
		const h = this.#idsFramebuffer.height;
		const dpr = devicePixelRatio ?? 1;
		const [r, g, b, a] = this.#idsFramebuffer.readPixels(dpr * x, h - dpr * y, 1, 1);

		if (this.#webgl2) {
			return this.#idMap.get(r);
		} else {
			if (!a) {
				return undefined;
			}

			const id = r + 0x100 * g + 0x10000 * b;
			return this.#idMap.get(id);
		}
	}

	#hoveredSymbol;

	/**
	 * @method setPointerCapture(pointerId: Number, symbol: GleoSymbol): this
	 * Sets pointer capture for the given pointer ID to the given symbol.
	 * When pointer capture is set (for a pointer ID), normal hit detection is
	 * skipped, and the capturing symbol will receive the event instead.
	 *
	 * See [`Element.setPointerCapture()`](https://developer.mozilla.org/docs/Web/API/Element/setPointerCapture)
	 */
	setPointerCapture(pointerId, symbol) {
		this.#pointerCaptureMap.set(pointerId, symbol);
		return this;
	}

	/**
	 * @method releasePointerCapture(pointerId: Number): this
	 * Clears pointer capture for the given pointer ID. Inverse of `setPointerCapture`.
	 *
	 * See [`Element.releasePointerCapture()`](https://developer.mozilla.org/docs/Web/API/Element/releasePointerCapture)
	 */
	releasePointerCapture(pointerId) {
		/// TODO: Should this take a `GleoSymbol` as well, to provide a sanity check?
		/// i.e. only a capturing symbol should release capture on a pointer ID.
		this.#pointerCaptureMap.delete(pointerId);
		return this;
	}

	/**
	 * @method dispatchPointerEvent(ev: GleoPointerEvent, evInit: Object): Boolean
	 * Given a event, finds what symbol in this acetate should receive the event
	 * and, if any, makes that symbol dispatch the event. If the symbol event
	 * is not `preventDefault`ed, the acetate itself dispatches the event afterwards.
	 *
	 * This is meant to be called *only* from the containing `GleoMap`.
	 *
	 * Since a `GleoPointerEvent` of type `pointermove` might mean entering/leaving
	 * a symbol, extra `pointerenter`/`pointerover`/`pointerout`/`pointerleave`
	 * might be dispatched as well. To internally ease that, dispatching an event
	 * requires a `evInit` dictionary with which to instantiate these new
	 * synthetic events.
	 *
	 * Return value as per `EventTarget`'s `dispatchEvent`: Boolean `false`
	 * if the event is `preventDefault()`ed, at either the symbol or the acetate level.
	 *
	 * FIXME: This **assumes** that the bbox of the acetate and the containing map
	 * are the same. It should be needed to modify the logic and rely on the `geom`
	 * property of the decorated `GleoPointerEvent` instead.
	 *
	 * That would involve caching the (direct) CRS affine matrix from the last
	 * time the acetate was drawn, apply it to the event's `geom` and then round the
	 * resulting pixel coordinate.
	 */
	dispatchPointerEvent(ev, init) {
		if (this.queryable) {
			ev.colour = init.colour = this.getColourAt(ev.canvasX, ev.canvasY);
		}
		if (!this.#isInteractive) {
			return this.dispatchEvent(ev);
		}

		let symbol;
		if (
			ev.type !== "pointercancel" &&
			ev.type !== "pointerleave" &&
			ev.type !== "pointerout"
		) {
			const capturingSymbol = this.#pointerCaptureMap.get(ev.pointerId);
			if (capturingSymbol) {
				symbol = capturingSymbol;
			} else {
				symbol = this.getSymbolAt(ev.canvasX, ev.canvasY);
			}
		}

		if (ev.type === "pointerup" || ev.type === "pointercancel") {
			this.releasePointerCapture(ev.pointerId);
		}

		// Same as map's _onPointerEvent, events should always be MouseEvent, but
		// Firefox doesn't respect that bit of the standard (as of now)
		const EventProto = ev instanceof PointerEvent ? GleoPointerEvent : GleoMouseEvent;

		if (this.#hoveredSymbol && this.#hoveredSymbol !== symbol) {
			this.#hoveredSymbol.dispatchEvent(new EventProto("pointerout", init));
			this.#hoveredSymbol.dispatchEvent(new EventProto("pointerleave", init));
			if (this.#hoveredSymbol.cursor) {
				this.platina.unqueueCursor(this.#hoveredSymbol.cursor);
			}
		}

		if (symbol) {
			if (this.#hoveredSymbol !== symbol) {
				symbol.dispatchEvent(new EventProto("pointerenter", init));
				symbol.dispatchEvent(new EventProto("pointerover", init));
				if (symbol.cursor) {
					this.platina.queueCursor(symbol.cursor);
				}
			}
			if (!symbol.dispatchEvent(ev)) {
				return false;
			}
			if (!symbol._eventParents.every((s) => s.dispatchEvent(ev))) {
				return false;
			}
			// } else {
			// 	this.releasePointerCapture(ev.pointerId);
		}

		this.#hoveredSymbol = symbol;

		return this.dispatchEvent(ev);
	}

	/// DEBUG
	get _idsTexture() {
		return this.#idsTexture;
	}
}
