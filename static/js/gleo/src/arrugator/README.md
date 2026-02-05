# Arrugator

A tool for subdividing triangular meshes for GIS reprojection purposes.

See https://ivan.sanchezortega.es/development/2021/03/08/introducing-arrugator.html

### Usage

The inputs are:

-   A `projector` function (which takes an `Array` of 2 `Number`s, and returns an `Array` of 2 `Number`s). Typically this is meant to be a [`proj4js`](proj4js.org) forward projection function, like `proj4(srcCRS, destCRS).forward`; however, arrugator has no hard dependency on proj4js, so other projection methods could be used.
-   The unprojected coordinates (an `Array` of `Array`s of 2 `Number`s, typically NW-SW-NE-SE)
-   The [UV-mapping](https://en.wikipedia.org/wiki/UV_mapping) coordinates (an `Array` of `Array`s of 2 `Number`s, typically `[[0,0],[0,1],[1,0],[1,1]]`)
-   The vertex indices of the triangles composing the initial mesh (an `Array` of `Array`s of 3 `Number`s, typically `[[0,1,3],[0,3,2]]`).

Note that the _typical_ input is four vertices, but there's no hard requirement on that. Any triangular mesh should do (and _maybe_ there are edge cases I haven't think of where it's required so things work for _weird_ projections like polyhedral ones).

And the ouputs are:

-   The unprojected vertex coordinates (an `Array` of `Array`s of 2 `Number`s)
-   The projected vertex coordinates (an `Array` of `Array`s of 2 `Number`s)
-   The [UV-mapping](https://en.wikipedia.org/wiki/UV_mapping) coordinates (an `Array` of `Array`s of 2 `Number`s)
-   The vertex indices of the triangles composing the mesh (an `Array` of `Array`s of 3 `Number`s).

### Usage example

Initialize some data (assuming `proj4` has already been set up):

```js
// These are the corner coordinates of a Spanish 1:2.000.000 overview map in ETRS89+UTM30N:
let epsg25830coords = [
	[-368027.127, 4880336.821], // top-left
	[-368027.127, 3859764.821], // bottom-left
	[1152416.873, 4880336.821], // top-right
	[1152416.873, 3859764.821], // bottom-right
];

let sourceUV = [
	[0, 0], // top-left
	[0, 1], // bottom-left
	[1, 0], // top-right
	[1, 1], // bottom-right
];

let arruga = new Arrugator(
	proj4("EPSG:25830", "EPSG:3034").forward,
	epsg25830coords,
	sourceUV,
	[
		[0, 1, 3],
		[0, 3, 2],
	] // topleft-bottomleft-bottomright ; topleft-bottomright-topright
);
```

Then, subdivide once:

```js
arruga.step();
```

Or subdivide several times:

```js
for (let i = 0; i < 10; i++) {
	arruga.step();
}
```

Or subdivide until epsilon is lower than a given number (**square** of distance in map units of the projected CRS - in this example, EPSG:3034 map units):

```js
arruga.lowerEpsilon(1000000); // 1000 "meter"s, squared
```

If there are antimeridian artefacts, or an "epsilon stall" warning appears on your console, you might want to "force" subdividing every segment before running the default subdivisions:

```js
arruga.force();
```


Once you're happy with the subdivisions, fetch the mesh state:

```js
let arrugado = arruga.output();

let unprojectedCoords = arrugado.unprojected;
let projectedCoords = arrugado.projected;
let uvCoords = arrugado.uv;
let trigs = arrugado.trigs;
```

The output are `Array`s of `Array`s, so the use case of dumping the data into a `TypedArray` to use it in a WebGL buffer needs them to be `.flat()`tened before.

How to do this depends on how you're usign WebGL (or what WebGL framework you're using). For example, my [glii](https://gitlab.com/IvanSanchez/glii) examples work like:

```js
const pos = new glii.SingleAttribute({ glslType: "vec2", growFactor: 2 });
const uv = new glii.SingleAttribute({ glslType: "vec2", growFactor: 2 });
const indices = new glii.TriangleIndices({ growFactor: 2 });

pos.setBytes(0, 0, Float32Array.from(arrugado.projected.flat()));
uv.setBytes(0, 0, Float32Array.from(arrugado.uv.flat()));
solidIndices.allocateSlots(arrugado.trigs.length * 3);
solidIndices.set(0, arrugado.trigs.flat());
wireIndices.allocateSlots(arrugado.trigs.length * 3);
wireIndices.set(0, arrugado.trigs.flat());
```

### Demos

See the [`demo` branch](https://gitlab.com/IvanSanchez/arrugator/-/tree/demo) of this git repository; there are some [glii](https://gitlab.com/IvanSanchez/glii)-powered examples there, including demo raster data.


## `LineArrugator`

There is also `LineArrugator`, a lightweight form of `Arrugator` designed to work
on segment lists ("polylines") instead of working on triangle meshes.

The input is just a list of `[x,y]` coordinates, and the output is another list of
`[x,y]` coordinates, projected.

```
let arruga = new LineArrugator(
	proj4('EPSG:4326','EPSG:25830').forward,
	[[-50, 0], [40, 25]]
);

arruga.lowerEpsilon(10000);

console.log(arruga.output());
```


### Legalese

Released under the General Public License, v3. See the LICENSE file for details.
