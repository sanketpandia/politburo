
import VectorTileLayer from "./vectortilelayer.mjs";


export default class VectorTile{
	constructor (pbf, end) {
	this.layers = pbf.readFields(readTile, {}, end);
}
}

function readTile(tag, layers, pbf) {
	if (tag === 3) {
		var layer = new VectorTileLayer(pbf, pbf.readVarint() + pbf.pos);
		if (layer.length) layers[layer.name] = layer;
	}
}
