/**
 * Map Route Utilities for Leaflet
 * 
 * Provides reusable functions for drawing altitude-colored routes on Leaflet maps.
 * Can be imported and used across multiple template files.
 * 
 * Note: For new code, prefer using the FlightMap component (flight-map.mjs) which
 * includes route functionality built-in.
 */

/**
 * Calculate color based on altitude using linear interpolation from red to blue
 * @param {number} altitude - Altitude in feet
 * @param {number} maxAltitude - Maximum altitude for cruise (default: 30000 ft)
 * @returns {string} Hex color string (e.g., "#ef4444")
 */
export function getAltitudeColor(altitude, maxAltitude = 30000) {
  // Clamp ratio between 0 and 1
  const ratio = Math.min(Math.max(altitude / maxAltitude, 0), 1);

  // Color definitions
  // Red for 0 ft: #ef4444 (rgb(239, 68, 68))
  // Blue for cruise: #3b82f6 (rgb(59, 130, 246))
  const redLow = { r: 239, g: 68, b: 68 };   // #ef4444
  const blueHigh = { r: 59, g: 130, b: 246 }; // #3b82f6

  // Linear interpolation
  const r = Math.round(redLow.r + (blueHigh.r - redLow.r) * ratio);
  const g = Math.round(redLow.g + (blueHigh.g - redLow.g) * ratio);
  const b = Math.round(redLow.b + (blueHigh.b - redLow.b) * ratio);

  // Convert to hex
  const hexColor = `#${r.toString(16).padStart(2, '0')}${g.toString(16).padStart(2, '0')}${b.toString(16).padStart(2, '0')}`;
  return hexColor;
}

function isValidLatLng(lat, lng) {
  return (
    !Number.isNaN(lat) &&
    !Number.isNaN(lng) &&
    lat >= -90 &&
    lat <= 90 &&
    lng >= -180 &&
    lng <= 180
  );
}

function normalizeRoutePoints(routePoints) {
  const result = [];

  for (const point of routePoints) {
    const lat = Number(point.latitude);
    const lng = Number(point.longitude);

    if (!isValidLatLng(lat, lng)) continue;

    const normalized = {
      ...point,
      latitude: lat,
      longitude: lng,
    };
    const previous = result[result.length - 1];

    if (!previous) {
      result.push(normalized);
      continue;
    }

    const epsilon = 0.0001;
    if (Math.abs(lat - previous.latitude) > epsilon || Math.abs(lng - previous.longitude) > epsilon) {
      result.push(normalized);
    }
  }

  return result;
}

function crossesAntimeridian(a, b) {
  return Math.abs(Number(b.longitude) - Number(a.longitude)) > 180;
}

function interpolateAntimeridianPoint(a, b, longitude) {
  const latA = Number(a.latitude);
  const lngA = Number(a.longitude);
  const latB = Number(b.latitude);
  let lngB = Number(b.longitude);

  if (lngA > 0 && lngB < 0) {
    lngB += 360;
  } else if (lngA < 0 && lngB > 0) {
    lngB -= 360;
  }

  const targetLng = longitude === 180 && lngA < 0 ? -180 : longitude;
  const unwrappedTargetLng = lngA > 0 && targetLng < 0 ? targetLng + 360 : targetLng;
  const ratio = (unwrappedTargetLng - lngA) / (lngB - lngA);
  const lat = latA + ((latB - latA) * ratio);

  return {
    ...a,
    latitude: lat,
    longitude,
  };
}

function splitRouteAtAntimeridian(routePoints) {
  const normalized = normalizeRoutePoints(routePoints);

  if (normalized.length < 2) return [];

  const segments = [];
  let current = [normalized[0]];

  for (let i = 1; i < normalized.length; i++) {
    const previous = normalized[i - 1];
    const point = normalized[i];

    if (crossesAntimeridian(previous, point)) {
      current.push(interpolateAntimeridianPoint(previous, point, Number(previous.longitude) < 0 ? -180 : 180));

      if (current.length >= 2) {
        segments.push(current);
      }

      current = [interpolateAntimeridianPoint(previous, point, Number(point.longitude) < 0 ? -180 : 180), point];
    } else {
      current.push(point);
    }
  }

  if (current.length >= 2) {
    segments.push(current);
  }

  return segments;
}

function toLatLngTuple(point) {
  return [Number(point.latitude), Number(point.longitude)];
}

function getLongestRouteSegment(segments) {
  return segments.reduce((longest, current) => current.length > longest.length ? current : longest, []);
}

/**
 * Create a route manager for a Leaflet map instance
 * @param {L.Map} map - Leaflet map instance
 * @param {Object} options - Configuration options
 * @param {number} options.maxAltitude - Maximum altitude for cruise (default: 30000)
 * @param {number} options.lineWidth - Width of route lines (default: 3)
 * @returns {Object} Route manager with mapRoute and clearRoute methods
 */
export function createRouteManager(map, options = {}) {
  const {
    maxAltitude = 30000,
    lineWidth = 3
  } = options;

  // Store route polylines for cleanup
  let routePolylines = [];
  let routeLayerGroup = null;

  // Create a layer group for routes if it doesn't exist
  if (!routeLayerGroup) {
    routeLayerGroup = L.layerGroup().addTo(map);
  }

  /**
   * Draw a route on the map with altitude-based color grading
   * @param {Array} routePoints - Array of route point objects with {latitude, longitude, altitude, ...}
   */
  function mapRoute(routePoints) {
    if (!map) {
      console.error('mapRoute: Map instance is not available');
      return;
    }

    if (!Array.isArray(routePoints) || routePoints.length < 2) {
      console.warn('mapRoute: routePoints must be an array with at least 2 points');
      return;
    }

    // Clear any existing route first
    clearRoute();

    const normalizedRoutePoints = normalizeRoutePoints(routePoints);

    // Draw segments between consecutive points
    for (let i = 0; i < normalizedRoutePoints.length - 1; i++) {
      const point1 = normalizedRoutePoints[i];
      const point2 = normalizedRoutePoints[i + 1];

      // Validate coordinates
      if (point1.latitude == null || point1.longitude == null ||
          point2.latitude == null || point2.longitude == null) {
        console.warn(`mapRoute: Skipping segment ${i} due to missing coordinates`);
        continue;
      }

      if (crossesAntimeridian(point1, point2)) {
        console.debug('mapRoute: Skipping antimeridian split segment', {
          from: [point1.latitude, point1.longitude],
          to: [point2.latitude, point2.longitude],
        });
        continue;
      }

      const segment = [toLatLngTuple(point1), toLatLngTuple(point2)];

      // Calculate average altitude for color interpolation
      const altitude1 = point1.altitude != null ? point1.altitude : 0;
      const altitude2 = point2.altitude != null ? point2.altitude : 0;
      const avgAltitude = (altitude1 + altitude2) / 2;

      // Get color based on altitude
      const color = getAltitudeColor(avgAltitude, maxAltitude);

      // Create polyline segment
      const polyline = L.polyline(segment, {
        color: color,
        weight: lineWidth,
        opacity: 0.8
      });

      polyline.addTo(routeLayerGroup);
      routePolylines.push(polyline);
    }

    // Fit map bounds to show the route
    const routeSegments = splitRouteAtAntimeridian(normalizedRoutePoints);
    const routeBounds = getLongestRouteSegment(routeSegments).map(toLatLngTuple);

    if (routeBounds.length > 0) {
      map.fitBounds(routeBounds, {
        padding: [50, 50],
        maxZoom: 10
      });
    }

    console.log(`mapRoute: Rendered ${routePolylines.length} route segments`);
  }

  /**
   * Clear all route segments from the map
   */
  function clearRoute() {
    routePolylines.forEach(polyline => {
      if (routeLayerGroup && routeLayerGroup.hasLayer(polyline)) {
        routeLayerGroup.removeLayer(polyline);
      }
    });
    routePolylines = [];
    console.log('clearRoute: Cleared all route segments');
  }

  // Return public API
  return {
    mapRoute,
    clearRoute
  };
}
