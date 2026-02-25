import AcetateVertices from "./AcetateVertices.mjs";
// import { registerDefaultAcetate } from "../Platina.mjs";
// import Allocator from "../glii/src/Allocator.mjs";

/**
 * @class AcetateStitchedTiles
 * @inherits AcetateVertices
 *
 * @relationship compositionOf TilePyramid, 0..n, 1..1
 *
 * An `Acetate` that draws rectangular conformal (i.e. matching the display CRS)
 * RGB(A) raster images, all of which fit together inside a Glii texture (and
 * so they share it). Users should not use this acetate directly; look at
 * `MercatorTiles` and `RasterTileLoader` and instead.
 *
 * This acetate will **not** hold an indefinite number of tiles; rather,
 * a tile might overwrite an existing tile. The (maximum) number of tiles at
 * any given moment depends on the size of the WebGL texture used.
 */

export default class AcetateStitchedTiles extends AcetateVertices {
	#MRULevels; // Most Recently Used levels
	#texFilter; // Either glii.NEAREST or glii.LINEAR

	/**
	 * Info about tile pyramid levels. Looks like:
	 * "8": {
	 * 	scale: 9.26,
	 * 	resX: 256,	// size of tiles in raster px
	 * 	resY: 256,	// size of tiles in raster px
	 * 	wrapX: 16,	// amount of tiles fitting in the texture
	 * 	wrapY: 16,	// amount of tiles fitting in the texture
	 * 	texSizeX: 4096,	// (desired) Size of texture
	 * 	texSizeY: 4096,	// (desired) Size of texture
	 * 	baseVtx: 348	// Index of the first vertex attribute for the level
	 * 	valid: true,	// Whether should be drawn or not
	 * }
	 */
	#levels = {};
	#levelNames = [];

	#uvAttr;
	#timestampAttr;
	#fadeInDuration;

	constructor(
		glii,
		{
			/**
			 * @section AcetateStitchedTiles Options
			 * @option pyramid: TilePyramid
			 * The tile pyramid to use
			 * @option tileResX: Number = 256; Horizontal size, in pixels, of each tile.
			 * @option tileResY: Number = 256; Vertical size, in pixels, of each tile.
			 * @option minTextureSize: Number = 2048
			 * Minimum size of the textures used to cache tile data. This should
			 * be set to the maximum expected size of the map (`RasterTileLoader`
			 * does so).
			 *
			 * Lower values might save some GPU memory, but will cause tiles to
			 * be culled prematurely.
			 *
			 * Higher values will keep more tiles cached in GPU textures, but
			 * will use more GPU memory and can cause browsers (notably
			 * chrome/chromium) to spend more time allocating the textures. Texture
			 * size is ultimately bound by the WebGL capabilities of the
			 * browser/OS/GPU, which usually can support textures 8192 or 16384
			 * pixels wide/high.
			 * @option interpolate: Boolean = false
			 * Whether to use bilinear pixel interpolation or not.
			 *
			 * In other words: `false` means pixellated, `true` means smoother.
			 * @option fadeInDuration: Number = 250
			 * Duration, in milliseconds, of the tile fade-in animation.
			 * @option maxLoadedLevels: Number = 3
			 * Number of maximum tile levels to keep loaded in their textures.
			 * Higher values can provide a slightly better experience when
			 * zooming in and out, but will use more GPU RAM.
			 * @option resizablePlatina: Boolean = true
			 * Whether the platina can be expected to be resized up to the size
			 * of the screen. When `false`, less GPU RAM is used for the textures.
			 */
			pyramid,
			tileResX = 256,
			tileResY = 256,
			minTextureSize = 2048,
			// minTextureSize = 1024,
			interpolate = false,
			fadeInDuration = 250,
			maxLoadedLevels = 4,
			...opts
		} = {}
	) {
		super(glii, opts);

		// this._texture = new glii.Texture();
		this._pyramid = pyramid;
		this.#levelNames = pyramid.mapLevels((name) => name);

		this.#MRULevels = new Array(maxLoadedLevels);

		this.#fadeInDuration = fadeInDuration;

		// Timestamp when the fade-in animation must stop.
		this._fadeTimeout = undefined;

		this._textures = {};

		this._crs = pyramid.crs;

		this._indices = new glii.LoDIndices({
			type: glii.UNSIGNED_INT,
			size: 0,
			growFactor: 1,
		});

		this.#texFilter = !!interpolate ? this.glii.LINEAR : this.glii.NEAREST;
		// const attrs = new Float32Array(levelCount * this._tilesPerLevel * 3);
		const uvs = [];
		const idxs = [];
		let vtx = 0;

		this._scales = {};

		const maxTexSize = glii.Texture.getMaxSize();

		this.#levelNames.forEach((levelName) => {
			const level = this._pyramid.getLevelDef(levelName);

			const resX = isFinite(tileResX) ? tileResX : tileResX[levelName];
			const resY = isFinite(tileResY) ? tileResY : tileResY[levelName];

			if (resX > maxTexSize || resY > maxTexSize) {
				throw new Error(
					`Resolution of tiles (${resX}, ${resY}) cannot be greater than the maximum size of textures (${maxTexSize})`
				);
			}
			if (resX < 0 || resY < 0) {
				throw new Error(
					`Resolution of tiles (${resX}, ${resY}) cannot be negative`
				);
			}

			// Scale is used as an sttribute to prevent z-fighting, so their
			// log2s work just as well and prevent float precision issues
			const scale = Math.log2(this._pyramid.getLevelDef(levelName).scale);
			this._scales[levelName] = scale;

			/// FIXME: What happens with maps with a yaw rotation of 45°??? Might need
			/// to multiply by sqrt(2).

			// Ideally, the size of a StitchedTiles texture would be the
			// maximum that the GPU allows - that's easily 8k x 8k pixels or
			// 16k x 16k.
			// Unfortunately, big framebuffers hog GPU RAM and cause browsers
			// (chromium/chrome in particular) to hang up during framebuffer
			// initialization.

			// The final size of the textures used will be:
			// - A power of 2 (hardcoded, in order to wrap textures across the
			//   antimeridian)
			// - Enough to fit tiles worth `minTextureSize` pixels, plus one
			//   extra tile.

			// Furthermore, since AcetateStitchedTiles allocates several textures
			// (one per pyramid level), big texture sizes can mean *a lot* of memory.
			// This is a problem for some old-ish or mobile GPUs, where allocating
			// more than ~128MiB of GPU RAM is a problem.
			/// FIXME: The X/Y tile span of each level must be a multiple of
			/// _tileWrapX/Y. Otherwise, loading tiles around the antimeridian will
			/// glitch (tiles ask to be stored in an offset modulo _tileWrapX/Y,
			/// and the last tile doesn't map to _tileWrapX/Y - 1, leading to
			/// tiles overwritting visible tiles). This might mean upping the textures
			/// to 4k :-/

			let tilesFitX = Math.min(level.spanX, Math.ceil(minTextureSize / resX));
			let tilesFitY = Math.min(level.spanY, Math.ceil(minTextureSize / resY));
			let texSizeX = tilesFitX * resX;
			let texSizeY = tilesFitY * resY;

			/// TODO: Fix non-power-of-two textures. Somehow disabling the p-o-2
			/// logic scrambles tiles around.

			// const forcePowerOfTwo = (this.interpolate || !this.glii instanceof WebGL2RenderingContext);
			// if (forcePowerOfTwo || isFinite(pyramid.crs.wrapPeriodX)) {
			texSizeX = 1 << Math.ceil(Math.log2(texSizeX));
			tilesFitX = Math.floor(texSizeX / resX);
			// }
			// if (forcePowerOfTwo || isFinite(pyramid.crs.wrapPeriodY)) {
			texSizeY = 1 << Math.ceil(Math.log2(texSizeY));
			tilesFitY = Math.floor(texSizeY / resY);
			// }

			texSizeX = Math.min(texSizeX, resX * level.spanX);
			texSizeY = Math.min(texSizeY, resY * level.spanY);

			// console.log(levelName, tilesFitX, tilesFitY );

			this.#levels[levelName] = {
				scale: scale,
				resX: resX,
				resY: resY,
				wrapX: tilesFitX,
				wrapY: tilesFitY,
				texSizeX: texSizeX,
				texSizeY: texSizeY,
				baseVtx: vtx,
				valid: false,
			};

			// console.log("level", levelName, this.#levels[levelName]);

			for (let y = 0; y < tilesFitY; y++) {
				for (let x = 0; x < tilesFitX; x++) {
					// Fill up the **static** values for the UV attribute, and the triangle indices
					// TODO: Consider using strided arrays??

					//prettier-ignore
					uvs.push(...[
						// UV map
						x/tilesFitX      , y/tilesFitY,
						(x + 1)/tilesFitX, y/tilesFitY,
						(x + 1)/tilesFitX, (y + 1)/tilesFitY,
						x/tilesFitX      , (y + 1)/tilesFitY,
					]/*, i * 3*/);

					// prettier-ignore
					idxs.push(
						vtx, vtx+1, vtx+2,
						vtx, vtx+2, vtx+3
					);

					vtx += 4;
				}
			}

			this._indices.allocateSet(levelName, idxs);

			idxs.splice(0); // Truncate idxs.
		});

		// UV attribute
		this.#uvAttr = new glii.SingleAttribute({
			usage: glii.STATIC_DRAW,
			size: vtx,
			growFactor: false,

			// UV map
			glslType: "vec2",
			type: Float32Array,
			// normalized: false,
		});
		this.#uvAttr.setBytes(0, 0, Float32Array.from(uvs));

		// Attribute to hold timestamps for the fade-in animation
		this.#timestampAttr = new this.glii.SingleAttribute({
			usage: glii.DYNAMIC_DRAW,
			size: vtx,
			growFactor: false,

			glslType: "float",
			type: Float32Array,
		});

		// Setting the coordinates for the last vertex will allocate and
		// fill in with zeroes all previous ones
		this._coords.setArray(vtx - 1, [0, 0]);
	}

	// Similar to AcetateConformalRaster
	glProgramDefinition() {
		const opts = super.glProgramDefinition();
		return {
			...opts,
			attributes: {
				...opts.attributes,
				aUV: this.#uvAttr,
				aTimestamp: this.#timestampAttr,
			},
			uniforms: {
				uNow: "float", // Current timestamp
				...opts.uniforms,
			},
			textures: {
				uRasterTexture: undefined,
			},
			vertexShaderMain: `
				vUV = aUV;
				vAlpha = min(1., ((uNow - aTimestamp) / ${this.#fadeInDuration}.));
				gl_Position = vec4(vec3(aCoords, 1.0) * uTransformMatrix, 1.0);
			`,
			varyings: { vUV: "vec2", vAlpha: "float" },
			fragmentShaderMain: `
				gl_FragColor = texture2D(uRasterTexture, vUV);
				gl_FragColor.a *= vAlpha;
			`,
			blend: {
				equationRGB: this.glii.FUNC_ADD,
				equationAlpha: this.glii.FUNC_ADD,

				srcRGB: this.glii.SRC_ALPHA,
				dstRGB: this.glii.ONE_MINUS_SRC_ALPHA,
				srcAlpha: this.glii.ONE,
				dstAlpha: this.glii.ONE_MINUS_SRC_ALPHA,
			},
		};
	}

	/**
	 * @section
	 * @method multiAdd(tiles: Array of Tile): this
	 * Adds the tiles to this acetate (so they're drawn on the next refresh).
	 *
	 * The images for the tiles are dumped into the acetate's texture.
	 *
	 * Unlike most other acetates, tiles are added on an individual basis and
	 * their data might not be stored adjacently in the attribute/primitive
	 * buffers.
	 */
	multiAdd(tiles) {
		/// TODO: Keep track of loaded tiles, in order to fire the `symbolsremoved`
		/// event whenever tiles are overwritten.

		// tiles.forEach(this.allocate.bind(this));
		tiles.forEach((t) => {
			this.allocate(t);
		});

		return super.multiAdd(tiles);
	}

	/**
	 * @method add(tile: Tile): this
	 *
	 * Adds a single tile. The tile will be slotted in a specific portion
	 * of the available space, depending on its X and Y coordinates within its pyramid level.
	 */
	allocate(tile) {
		const levelInfo = this.#levels[tile.level];
		const x = tile.tileX % levelInfo.wrapX;
		const y = tile.tileY % levelInfo.wrapY;

		// console.log("allocate tile:", tile.level, x, y);

		const baseVtx = levelInfo.baseVtx + (y * levelInfo.wrapY + x) * 4;
		const baseIdx = baseVtx * 1.5; // ratio is 4 vertices to 6 primitive slots

		tile.updateRefs(this, baseVtx, baseIdx);

		this._knownSymbols[baseVtx] = tile;

		this.reproject(baseVtx, 4);

		/// Perform MRU/LRU logic
		this.#loadLevelTexture(tile.level);

		this._textures[tile.level].texSubImage2D(
			tile.image,
			x * levelInfo.resX,
			y * levelInfo.resY
		);

		this._fadeTimeout = performance.now() + this.#fadeInDuration;
		// this._fadeTime.multiSet(baseVtx, new Array(4).fill(this._fadeTimeout));
		this.#timestampAttr.multiSet(baseVtx, new Array(4).fill(performance.now()));

		// console.log("Allocated", tile.level, tile.tileX, tile.tileY, performance.now());

		this.#levels[tile.level].valid = true;
		this.dirty = true;
		return this;
	}

	/**
	 * Redefinition of the default. Render must happen once per level, in order
	 * to load the appropriate textures. This leverages Glii's LoDIndices, by
	 * using a LoD per level of the pyramid.
	 */
	runProgram() {
		//this._clear();
		const now = performance.now();
		const platinaScale = Math.log2(this._platina.scale);
		this._programs.setUniform("uNow", now);

		// console.log("drawing levels", 		this.#levelNames .filter((name) => this.isLevelAvailable(name)).join (" , "))

		this.#levelNames
			.filter((name) => this.isLevelAvailable(name))
			.sort(
				(a, b) =>
					Math.abs(this._scales[b] - platinaScale) -
					Math.abs(this._scales[a] - platinaScale)
			)
			.forEach((name) => {
				this._programs.setTexture("uRasterTexture", this._textures[name]);
				this._programs.run(name);
			});

		if (now < this._fadeTimeout) {
			this.dirty = true;
		}
	}

	/**
	 * @method reproject(start: Number, length: Number): Array of Number
	 * Dumps a new set of values to the `this._coords` attribute buffer, based on the known
	 * set of symbols added to the acetate (only those which have their attribute offsets
	 * between `start` and `start+length`.
	 *
	 * Returns the data set into the attribute buffer: a plain array of coordinates
	 * in the form `[x1,y1, x2,y2, ... xn,yn]`.
	 *
	 * This implementation does not assume that the attribute allocation block
	 * contains a compact set of symbols (since tiles are statically allocated at
	 * instantiation time, then overwritten at runtime).
	 */
	reproject(start, length) {
		/// FIXME: Filtering needs optimization. Bisect search?
		/// Optimization only applies to chrome/chromium.
		let relevantSymbols = this._knownSymbols.filter((symbol, attrIdx) => {
			return attrIdx >= start && attrIdx + symbol.attrLength <= start + length;
		});

		let idx = start;

		const coordData = relevantSymbols
			.map((symbol) => {
				const gapLength = (symbol.attrBase - idx) * 2;
				const gap = new Array(gapLength).fill(0);
				idx = symbol.attrBase + symbol.attrLength;
				return gap.concat(symbol.geometry.toCRS(this._crs).coords);
			})
			.flat();

		//console.log("Symbol reprojected:", coordData);
		this.multiSetCoords(start, coordData);

		return coordData;
	}

	/**
	 * @section Acetate interface
	 * @method getLevelsInfo(): Object of Object
	 * Returns a data structure containing information about tile levels:
	 * tile resolution, expected texture size, number of tiles fitting in the
	 * texture, etc.
	 *
	 * Meant for debugging and communication with a `RasterTileLoader` only.
	 */
	getLevelsInfo() {
		return this.#levels;
	}

	// Ensures that the texture for given level is available.
	// If not, expels the LRU level from the MRU list, and reuses that texture;
	// or initializes a texture if the level expelled was `undefined`.
	#loadLevelTexture(levelName) {
		if (!this._textures[levelName]) {
			// console.log("load texture at level", levelName);
			const expel = this.#MRULevels.shift();
			this.#MRULevels.push(levelName);

			const sizeX = this.#levels[levelName].texSizeX;
			const sizeY = this.#levels[levelName].texSizeY;

			let currentX, currentY;

			if (expel !== undefined) {
				/**
				 * @section Acetate interface
				 * @event levelexpelled: Event
				 * Fired whenever a texture for a level of tiles is expelled, and
				 * thus all tiles from that level should be marked as unusable.
				 * The event's `detail` contains the name of the expelled level.
				 */
				this.fire("levelexpelled", { levelName: expel });
				currentX = this._textures[expel]?.width;
				currentY = this._textures[expel]?.height;
			}

			if (currentX == sizeX && currentY == sizeY) {
				// Texture from expelled level can be reused.
				this._textures[levelName] = this._textures[expel];
			} else {
				// Texture must be allocated.
				this._textures[levelName] = new this.glii.Texture({
					minFilter: this.#texFilter,
					magFilter: this.#texFilter,
					wrapS: this.glii.REPEAT,
					wrapT: this.glii.REPEAT,
				});
				if (expel !== undefined) {
					this._textures[expel]?.destroy();
				}
			}

			if (expel !== undefined) {
				delete this._textures[expel];
			}

			// No matter if the texture is reused or newly allocated,
			// it has to be zeroed out.
			this._textures[levelName].texArray(
				sizeX,
				sizeY,
				new Uint8Array(sizeX * sizeY * 4)
			);
		}

		return this._textures[levelName];
	}

	/**
	 * @section Acetate interface
	 * @method isLevelAvailable(levelName: String): Boolean
	 * Returns whether the texture for the given level name is available.
	 * In other words: when the given level has never been loaded, or it has
	 * been expelled from the MRU list, this returns `false`.
	 */
	isLevelAvailable(levelName) {
		return this.#levels[levelName].valid && !!this._textures[levelName];
	}

	/**
	 * @method destroyHigherScaleLevels(levelName: String): Boolean
	 * Searches all levels with a scale lower than the given one (i.e. those with
	 * "higher zoom levels") and marks them as invalid; will not be re-rendered
	 * until a tile for that level is allocated.
	 */
	destroyHigherScaleLevels(levelName) {
		const scale = this.#levels[levelName].scale;
		// const str = Object.values(this.#levels).map(l=>l.valid?"1":"0").join("");

		Object.entries(this.#levels).forEach(([name, level]) => {
			if (level.scale < scale && level.valid) {
				// console.log("invalidate", level);
				level.valid = false;

				const sizeX = level.texSizeX;
				const sizeY = level.texSizeY;

				this._textures[name]?.texArray(
					sizeX,
					sizeY,
					new Uint8Array(sizeX * sizeY * 4)
				);
			}
		});
		// console.log(str + "\n" + Object.values(this.#levels).map(l=>l.valid?"1":"0").join(""));
	}

	destroy() {
		// Ignore missing/incomplete tile symbols that do exist as
		// empty slots in _knownSymbols
		// this._knownSymbols = this._knownSymbols.filter((s) => !!s);
		Object.values(this._textures).forEach((t) => t.destroy());
		return super.destroy();
	}

	reprojectAll() {
		// This acetate does not use an attribute allocator like most others,
		// and must rely on the available tile levels to reproject existing
		// tiles
		for (const level of Object.values(this.#levels)) {
			this.reproject(level.baseVtx, level.wrapX * level.wrapY * 4);
		}
	}
}
