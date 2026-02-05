#!/bin/bash

# Download Gleo library files from unpkg
# This script downloads the necessary Gleo files for local use

set -e

GLEO_VERSION="1.1.0"
BASE_URL="https://unpkg.com/gleo@${GLEO_VERSION}/src"
TARGET_DIR="./static/js/gleo"

echo "Downloading Gleo library files..."

# Create target directory
mkdir -p "${TARGET_DIR}"

# Function to download a file
download_file() {
    local file_path="$1"
    local local_path="${TARGET_DIR}/${file_path}"
    local remote_url="${BASE_URL}/${file_path}"

    mkdir -p "$(dirname "${local_path}")"

    echo "Downloading: ${file_path}"
    curl -s "${remote_url}" -o "${local_path}" --create-dirs
}

# Download main map files
echo "Downloading main map files..."
download_file "CartesianMap.mjs"
download_file "Map.mjs"
download_file "MercatorMap.mjs"
download_file "Platina.mjs"

# Download loader files (we only need MercatorTiles)
echo "Downloading loader files..."
download_file "loaders/MercatorTiles.mjs"
download_file "loaders/AbstractTileLoader.mjs"
download_file "loaders/Loader.mjs"
download_file "loaders/RasterTileLoader.mjs"

# Download symbol files (we need Chain, Circle, TextLabel, and Symbol base)
echo "Downloading symbol files..."
download_file "symbols/Chain.mjs"
download_file "symbols/Circle.mjs"
download_file "symbols/TextLabel.mjs"
download_file "symbols/Symbol.mjs"
download_file "symbols/Stroke.mjs"
download_file "symbols/Fill.mjs"
download_file "symbols/Sprite.mjs"

# Download utility/dependency files
echo "Downloading utility files..."
download_file "util/Angle.mjs"
download_file "util/Bounds.mjs"
download_file "util/Cache.mjs"
download_file "util/Canvas.mjs"
download_file "util/CanvasGradient.mjs"
download_file "util/CanvasPattern.mjs"
download_file "util/CanvasPath.mjs"
download_file "util/CanvasState.mjs"
download_file "util/CanvasStyle.mjs"
download_file "util/CanvasText.mjs"
download_file "util/Cartesian.mjs"
download_file "util/Dom.mjs"
download_file "util/Event.mjs"
download_file "util/Feature.mjs"
download_file "util/FeatureCollection.mjs"
download_file "util/FeatureGroup.mjs"
download_file "util/FeatureStore.mjs"
download_file "util/Geometry.mjs"
download_file "util/Mercator.mjs"
download_file "util/Permuter.mjs"
download_file "util/Picker.mjs"
download_file "util/PixelSize.mjs"
download_file "util/Pixels.mjs"
download_file "util/Queue.mjs"
download_file "util/RectSize.mjs"
download_file "util/RenderQueueItem.mjs"
download_file "util/RequestAnimationFrameThrottler.mjs"
download_file "util/Serializer.mjs"
download_file "util/Spherical.mjs"
download_file "util/SvgPath.mjs"
download_file "util/World.mjs"

# Download geometry files
echo "Downloading geometry files..."
download_file "geometry/Geometry.mjs"
download_file "geometry/GeometryCollection.mjs"
download_file "geometry/LineString.mjs"
download_file "geometry/MultiLineString.mjs"
download_file "geometry/MultiPoint.mjs"
download_file "geometry/MultiPolygon.mjs"
download_file "geometry/Point.mjs"
download_file "geometry/Polygon.mjs"

# Download dom files
echo "Downloading dom files..."
download_file "dom/Dom.mjs"
download_file "dom/DomArrugatedRaster.mjs"
download_file "dom/DomElement.mjs"
download_file "dom/DomText.mjs"
download_file "dom/DomTextGradient.mjs"

# Download control files
echo "Downloading control files..."
download_file "control/Control.mjs"
download_file "control/Attribution.mjs"
download_file "control/Scale.mjs"
download_file "control/Zoom.mjs"

echo "✅ Gleo library download complete!"
echo "Files saved to: ${TARGET_DIR}"
