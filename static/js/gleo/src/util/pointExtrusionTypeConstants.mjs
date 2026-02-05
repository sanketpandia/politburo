/// These constants are used in the `_setPerPointStrides` method of symbols.

// A point extrudes as a line join - a miter, bevel or outbevel.
// If here's a centerline, it's the 2nd vertex (offset 1)
export const LINEJOIN = Symbol("LINEJOIN");

// A point extrudes as a line bevel join at the start of a geometry linear ring
// - skipping a vertex for the bevel (which will be accounted for at the end
// of the ring loop.
// If there's a centerline, it's the 1st vertex (offset 0)
export const LINELOOP = Symbol("LINELOOP");

// A point extrudes as a line cap - a butt or square
// If here's a centerline, it's the 2nd vertex (offset 1)
export const LINECAP = Symbol("LINECAP");

/// The following is used for extruded points, where the amount of WebGL vertices
/// spawned per geometry point depends on the specific symbol and its options.

export const EXTRUDED_POINT = Symbol("EXTRUDED_POINT");

/// The following is used for `Fill`s, `Mesh`es and also `Hair`s - any symbol
/// where each geometry point spawns one and just one WebGL vertex.

export const MESH = Symbol("MESH");
