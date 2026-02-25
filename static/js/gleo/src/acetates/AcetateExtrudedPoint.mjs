import AcetateInteractive from "./AcetateInteractive.mjs";

/**
 * @class AcetateExtrudedPoint
 * @inherits AcetateInteractive
 *
 * Abstract class, containing functionality common to acetates that draw point
 * symbols (`Sprite`, `Pie`, etc).
 */
export default class AcetateExtrudedPoint extends AcetateInteractive {
	constructor(glii, opts) {
		super(glii, opts);

		this._extrusions = new this.glii.SingleAttribute({
			usage: this.glii.STATIC_DRAW,
			size: 1,
			growFactor: 1.2,

			glslType: "vec2",
			type: Float32Array,
			normalized: false,
		});
	}

	glProgramDefinition() {
		const opts = super.glProgramDefinition();
		return {
			...opts,
			attributes: {
				...opts.attributes,
				aExtrude: this._extrusions,
			},
		};
	}

	_commitStridedArrays(baseVtx, vtxCount, baseIdx, idxCount) {
		this._extrusions.commit(baseVtx, vtxCount);
		this._attrs.commit(baseVtx, vtxCount);
		return super._commitStridedArrays(baseVtx, vtxCount, baseIdx, idxCount);
	}

	multiAdd(syms) {
		super.multiAdd(syms);
		return super.multiAllocate(syms);
	}

	/**
	 * @section Internal Methods
	 * @uninheritable
	 *
	 * @method reproject(start: Number, length: Number, symbols: Array of GleoSymbol): Array of Number
	 * Dumps a new set of values to the `this._coords` attribute buffer, based on the known
	 * set of symbols added to the acetate (only those which have their attribute offsets
	 * between `start` and `start+length`. Each symbol will spawn as many
	 * coordinate `vec2`s as their `attrLength` property.
	 *
	 * Returns the data set into the attribute buffer: a plain array of coordinates
	 * in the form `[x1,y1, x2,y2, ... xn,yn]`.
	 */
	reproject(start, length, symbols) {
		let relevantSymbols =
			symbols ??
			this._knownSymbols.filter((symbol, attrIdx) => {
				return (
					symbol.attrBase !== undefined &&
					attrIdx >= start &&
					attrIdx + symbol.attrLength <= start + length
				);
			});

		let addr = 0;

		// In most cases, it's safe to assume that relevant symbols in the same
		// attribute allocation block have their vertex attributes in a
		// compacted manner.

		const coordData = new Float64Array(length * 2);

		relevantSymbols.forEach((symbol) => {
			const projected = symbol.geometry.toCRS(this.platina.crs).coords;

			/// TODO: Debug edge case when offsetting CRS with clusters.
			// if (symbol.attrBase !== start + addr/2) {debugger;}

			for (let i = 0; i < symbol.attrLength; i++) {
				coordData.set(projected, addr);
				addr += 2;
			}
		});

		//console.log("Symbol reprojected:", coordData);
		this.multiSetCoords(start, coordData);

		this.dirty = true;

		return coordData;
	}
}
