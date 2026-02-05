import Mesh from "./Mesh.mjs";
import { factory } from "../geometry/DefaultGeometry.mjs";

import Delaunator from "../3rd-party/delaunator.js";

/**
 * @class DelaunayMesh
 * @inherits Mesh
 *
 * A `Mesh` which is calculated from a multipoint `Geometry`, leveraging Volodymir
 * Agafonkin's [`delaunator`](https://github.com/mapbox/delaunator)
 * implementation of the Delaunay triangulation.
 *
 * Works as a `Mesh`, but without the need of specifying the triangles array
 * at instantiation time.
 *
 * Note that the triangulation algorithm runs on the CRS of the data, and **not**
 * on the display CRS of the map.
 */

export default class DelaunayMesh extends Mesh {
	/**
	 * @constructor Mesh(geom: Geometry, opts?: DelaunayMesh Options)
	 */
	constructor(geom, opts = {}) {
		const geometry = factory(geom);

		/// TODO: sanity check on the dimension of the geometry. This should
		/// throw an error on geometries with dimension other than 2.

		// Gets fed the coordinates of a 2D `Geometry`, with coordinates already packed.
		const delaunated = new Delaunator(geometry.coords);

		super(geometry, opts, delaunated.triangles);
	}
}
