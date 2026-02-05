// Shims the `geotiff` browser bundle, so it can be loaded as an ES module via
// importmaps.

// The browser bundle must be loaded beforehand, with something like e.g.
// <script src="https://unpkg.com/geotiff@2.1.3/dist-browser/geotiff.js"></script>

export const GeoTIFF = window.GeoTIFF.GeoTIFF;
export const GeoTIFFImage = window.GeoTIFF.GeoTIFFImage;
export const fromUrl = window.GeoTIFF.fromUrl;
