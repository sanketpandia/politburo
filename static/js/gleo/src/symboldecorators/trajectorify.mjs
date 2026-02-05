import { factory } from "../geometry/DefaultGeometry.mjs";

/**
 * @namespace trajectorify
 * @inherits Symbol Decorator
 * @relationship associated ExtrudedPoint
 *
 * Turns a point symbol into a "moving point" symbol, that moves along a linear
 * trajectory.
 *
 * Instead of taking a point geometry, the symbol will now take a linestring
 * geometry, along with an array of M-coordinates (one for each vertex of the
 * linestring).
 *
 * (Ideally the geometry would be a XYM or XYZM geometry, with values across
 * the measurement ("M") dimension. But that's not implemented yet).
 *
 * The typical use case is time-based: the M-coordinate for the linestring
 * vertices would be a timestamp, when the feature passed through that
 * specific vertex.
 *
 * In a manner similar to the `intervalify` symbol decorator, the
 * "trajectorified" acetate has the capability to set the M coordinate to
 * any value, and will draw the symbols in the appropriate interpolated position.
 *
 */

/*
 * TODO: https://gitlab.com/IvanSanchez/gleo/-/issues/106
 *
 * - Split into chunks of m-coord
 *
 * - Put each segment in *all* the chunks it overlaps with
 *
 * - Do `drawPartial()` with the appropriate chunk.
 *
 */

export default function trajectorify(base) {
	// Check that the class' name is either `ExtrudedPoint` or `VertexDot`,
	// and iterate through the prototype chain (i.e. check that any of the
	// parent classes is named that way).
	let valid = false;
	let proto = base;
	while (!valid) {
		if (!proto) {
			throw new Error(
				"The 'trajectorify' symbol decorator can only be applied to extruded points or to VertexDots"
			);
		}
		valid |= proto.name === "ExtrudedPoint" || proto.name === "VertexDot";
		proto = proto.__proto__;
	}

	class TrajectorifiedAcetate extends base.Acetate {
		constructor(target, opts) {
			super(target, opts);

			// One LoD per M-coord chunk
			this._indices = new this.glii.LoDIndices({
				type: this.glii.UNSIGNED_INT,

				// Copy parent's drawMode. Mainly for trajectorifying `Dot`s
				drawMode: this._indices._drawMode,
			});

			// This will store the endpoint of each segment,
			// whereas this._coords will store the start point.
			this._2coords = new this.glii.SingleAttribute({
				size: 1,
				growFactor: 1.2,
				usage: this.glii.DYNAMIC_DRAW,
				glslType: "vec2",
				type: Float32Array,
			});

			// M-coordinates. M-coordinate of the segment start in the first
			// (.x) component, M-coordinate *length* of the segment in the
			// second (.y) component.
			this._mcoords = new this.glii.SingleAttribute({
				size: 1,
				growFactor: 1.2,
				usage: this.glii.DYNAMIC_DRAW,
				glslType: "highp vec2",
				type: Float32Array,
			});
			// console.log("constructed trajectorified acetate");
		}

		glProgramDefinition() {
			const opts = super.glProgramDefinition();

			return {
				...opts,
				attributes: {
					...opts.attributes,
					aMCoord: this._mcoords,
					a2Coords: this._2coords,
				},
				uniforms: {
					...opts.uniforms,
					uMValue: "float",
				},
				vertexShaderMain: `
					float segmentPercentage = (uMValue - aMCoord.x) / aMCoord.y;
					if (segmentPercentage < 0. || segmentPercentage > 1.) {
						return;
					}
					vec2 interCoords = mix(aCoords, a2Coords, segmentPercentage);
					${opts.vertexShaderMain.replace(/aCoords/g, "interCoords")}
				`,
			};
		}

		_getStridedArrays(maxVtx, maxIdx) {
			return [
				// M-coordinates
				this._mcoords.asStridedArray(maxVtx),

				// and everything from the parent
				...super._getStridedArrays(maxVtx, maxIdx),
			];
		}

		_commitStridedArrays(baseVtx, vtxCount) {
			super._commitStridedArrays(baseVtx, vtxCount);
			this._mcoords.commit(baseVtx, vtxCount);
		}

		reproject(start, length, symbols) {
			let relevantSymbols =
				symbols ??
				this._knownSymbols.filter((symbol, attrIdx) => {
					return (
						attrIdx >= start && attrIdx + symbol.attrLength <= start + length
					);
				});

			let addr = 0;

			const coordData = new Float64Array(length * 2);
			const coord2Data = new Float64Array(length * 2);

			relevantSymbols.forEach((symbol) => {
				const projected = symbol.geometry.toCRS(this.platina.crs).coords;

				for (let i = 0; i < symbol.segmentCount; i++) {
					const startPoint = projected.slice(i * 2, i * 2 + 2);
					const endPoint = projected.slice(i * 2 + 2, i * 2 + 4);

					for (let j = 0; j < symbol.segmentAttrLength; j++) {
						coordData.set(startPoint, addr);
						coord2Data.set(endPoint, addr);
						addr += 2;
					}
				}
			});

			this.multiSetCoords(start, coordData);

			// Akin to multiSetCoords(), but a bit more manual
			this._2coords.multiSet(start, coord2Data);
			this.expandBBox(coord2Data);

			this.platina.dirty = true;

			return coordData;
		}

		#mValue;
		#dirtyMValue;
		get mValue() {
			return this.#mValue;
		}
		set mValue(m) {
			this.#mValue = m;
			this.dirty = this.#dirtyMValue = true;
		}

		redraw() {
			if (this.#dirtyMValue) {
				this._programs.setUniform("uMValue", this.#mValue);
				this.#dirtyMValue = false;
			}
			return super.redraw.apply(this, arguments);
		}
	}

	/**
	 * @miniclass Trajectorified GleoSymbol (trajectorify)
	 *
	 * A "trajectorified" symbol accepts this additional constructor options:
	 */
	return class TrajectorifiedSymbol extends base {
		static Acetate = TrajectorifiedAcetate;

		constructor(
			geom,
			{
				/**
				 * @option mcoords: Array of Number = []
				 *
				 * The values for the M-coordinates of the geometry. This **must** have one
				 * M-coordinate per point in the symbol's geometry.
				 */
				mcoords = [],

				...opts
			} = {}
		) {
			// Avoid automatic de-duplication of geometry points - it's feasible
			// that a trajectorified symbol will repeat coordinates.
			const geometry = factory(geom, { deduplicate: false });

			super(geometry, opts);

			/// TODO: Handle multilinestrings as well
			this.segmentCount = this.geometry.coords.length / 2 - 1;

			this.segmentAttrLength = this.attrLength;
			this.segmentIdxLength = this.idxLength;
			this.mcoords = mcoords;

			this.attrLength *= this.segmentCount;
			this.idxLength *= this.segmentCount;
		}

		// Skip geometry assertion - geometries don't need to be points.
		_assertGeom() {}

		_setGlobalStrides(strideMCoord, ...arrs) {
			// Store the attribute and index base and length - this will need
			// to lie to the parent functionality so it runs _setGlobalStrides()
			// once per segment, with the right offsets. That means turning
			// these values from symbol-relative to segment-relative.
			const attrBase = this.attrBase;
			const attrLength = this.attrLength;
			const idxBase = this.idxBase;
			const idxLength = this.idxLength;

			this.attrLength = this.segmentAttrLength;
			this.idxLength = this.segmentIdxLength;

			/// TODO: handle multilinestrings, i.e. do geom.forEachRing()
			for (let i = 0; i < this.segmentCount; i++) {
				let startMCoord = this.mcoords[i];
				let endMCoord = this.mcoords[i + 1];
				let mCoordLength = endMCoord - startMCoord;
				for (let j = 0; j < this.attrLength; j++) {
					strideMCoord.set([startMCoord, mCoordLength], this.attrBase + j);
				}

				super._setGlobalStrides(...arrs);

				this.attrBase += this.attrLength;
				this.idxBase += this.idxLength;
			}

			// Restore values back to symbol-relative (at this point they're
			// relative to the last segment)
			this.attrBase = attrBase;
			this.attrLength = attrLength;
			this.idxBase = idxBase;
			this.idxLength = idxLength;
		}
	};
}
