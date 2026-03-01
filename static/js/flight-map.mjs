/**
 * FlightMap - Reusable Leaflet map component for flight radar
 * 
 * Provides a clean API for displaying flights, routes, and other geographic data
 * on a Leaflet map with altitude-based route coloring and interactive markers.
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

/**
 * Get color based on flight phase
 * @param {string} phase - Flight phase (cruise, climb, descent, takeoff, etc.)
 * @returns {string} Hex color string
 */
export function getFlightPhaseColor(phase) {
  const colors = {
    'cruise': '#3b82f6',     // blue-500
    'climb': '#10b981',      // emerald-500
    'descent': '#a855f7',    // purple-500
    'takeoff': '#f59e0b',    // amber-500
    'on_ground': '#64748b',  // slate-500
    'landed': '#64748b',     // slate-500
    'unknown': '#94a3b8',    // slate-400
  };
  return colors[phase] || '#3b82f6'; // Default to blue
}

/**
 * Create a custom HTML icon for a flight marker
 * @param {Object} flight - Flight data object
 * @param {string} color - Color for the marker
 * @returns {L.DivIcon} Leaflet DivIcon instance
 */
function createFlightIcon(flight, color) {
  const track = flight.track != null ? flight.track : 0;
  const rotation = track; // Track is already in degrees from north (0-360)
  
  // Create SVG for rotatable triangle (heading indicator)
  const svg = `
    <svg width="32" height="28" viewBox="0 0 32 28" xmlns="http://www.w3.org/2000/svg">
      <g transform="rotate(${rotation} 16 14)">
        <path d="M 16 2 L 28 26 L 16 22 Z" fill="${color}" stroke="#0f111a" stroke-width="2"/>
        <path d="M 16 2 L 4 26 L 16 22 Z" fill="${color}" stroke="#0f111a" stroke-width="2"/>
      </g>
    </svg>
  `;

  return L.divIcon({
    html: svg,
    className: 'flight-marker',
    iconSize: [32, 28],
    iconAnchor: [16, 14], // Center of the icon
    popupAnchor: [0, -14]
  });
}

/**
 * Create a simple circle icon for flights without heading
 * @param {string} color - Color for the marker
 * @returns {L.DivIcon} Leaflet DivIcon instance
 */
function createCircleIcon(color) {
  const svg = `
    <svg width="24" height="24" viewBox="0 0 24 24" xmlns="http://www.w3.org/2000/svg">
      <circle cx="12" cy="12" r="10" fill="${color}" stroke="#0f111a" stroke-width="2"/>
    </svg>
  `;

  return L.divIcon({
    html: svg,
    className: 'flight-marker',
    iconSize: [24, 24],
    iconAnchor: [12, 12],
    popupAnchor: [0, -12]
  });
}

/**
 * FlightMap class - Reusable map component for flights and routes
 */
export class FlightMap {
  /**
   * @param {string|HTMLElement} container - Map container ID or element
   * @param {Object} options - Map options
   * @param {Array<number>} options.center - Initial center [lat, lng] (default: [0, 0])
   * @param {number} options.zoom - Initial zoom level (default: 2)
   * @param {number} options.maxAltitude - Max altitude for route coloring (default: 30000)
   * @param {number} options.routeLineWidth - Route line width (default: 3)
   */
  constructor(container, options = {}) {
    const {
      center = [0, 0],
      zoom = 2,
      maxAltitude = 30000,
      routeLineWidth = 3
    } = options;

    this.maxAltitude = maxAltitude;
    this.routeLineWidth = routeLineWidth;

    // Initialize Leaflet map
    this.map = L.map(container, {
      center: center,
      zoom: zoom,
      zoomControl: true,
      attributionControl: true,
      maxZoom: 18,
      minZoom: 2,
    });

    // Add OpenStreetMap tiles
    L.tileLayer('https://tile.openstreetmap.org/{z}/{x}/{y}.png', {
      attribution: '© OpenStreetMap contributors',
      maxZoom: 19
    }).addTo(this.map);

    // Layer groups for managing flights and routes
    this.flightLayerGroup = L.layerGroup().addTo(this.map);
    this.routeLayerGroup = L.layerGroup().addTo(this.map);

    // Store flight markers and route polylines for management
    this.flightMarkers = new Map(); // flight_id -> marker
    this.routePolylines = [];

    // Click handler callback
    this.onFlightClickCallback = null;
  }

  /**
   * Add flights to the map
   * @param {Array} flights - Array of flight objects with {flight_id, latitude, longitude, track, phase, callsign, ...}
   */
  addFlights(flights) {
    if (!Array.isArray(flights) || flights.length === 0) {
      return;
    }

    // Clear existing flights
    this.clearFlights();

    const validFlights = flights.filter(f => 
      f.latitude != null && f.longitude != null
    );

    if (validFlights.length === 0) {
      return;
    }

    // Create markers for each flight
    validFlights.forEach(flight => {
      const color = getFlightPhaseColor(flight.phase);
      const icon = flight.track != null 
        ? createFlightIcon(flight, color)
        : createCircleIcon(color);

      const marker = L.marker([flight.latitude, flight.longitude], {
        icon: icon
      });

      // Add callsign as popup/tooltip
      if (flight.callsign) {
        marker.bindTooltip(flight.callsign, {
          permanent: true,
          direction: 'right',
          offset: [10, 0],
          className: 'flight-tooltip'
        });
      }

      // Store flight data on marker for click handler
      marker._flightData = flight;

      // Add click handler
      marker.on('click', (e) => {
        if (this.onFlightClickCallback) {
          this.onFlightClickCallback(flight);
        }
      });

      // Add to layer group
      marker.addTo(this.flightLayerGroup);
      this.flightMarkers.set(flight.flight_id, marker);
    });

    // Fit map bounds to show all flights
    if (validFlights.length > 0) {
      const bounds = validFlights.map(f => [f.latitude, f.longitude]);
      this.map.fitBounds(bounds, {
        padding: [50, 50], // Padding in pixels
        maxZoom: 18 // Allow deeper zoom
      });
    }

    console.log(`FlightMap: Added ${validFlights.length} flights to map`);
  }

  /**
   * Clear all flights from the map
   */
  clearFlights() {
    this.flightLayerGroup.clearLayers();
    this.flightMarkers.clear();
  }

  /**
   * Add a route to the map with altitude-based coloring
   * @param {Array} waypoints - Array of waypoint objects with {latitude, longitude, altitude, ...}
   * @param {Object} options - Route options
   * @param {boolean} options.colorBy - Color by 'altitude' or 'phase' (default: 'altitude')
   * @param {string} options.color - Fixed color if not using altitude/phase coloring
   */
  addRoute(waypoints, options = {}) {
    console.log('FlightMap.addRoute called', {
      waypointsCount: waypoints?.length,
      mapExists: !!this.map,
      routeLayerGroupExists: !!this.routeLayerGroup,
      mapLoaded: this.map?.loaded
    });

    if (!Array.isArray(waypoints) || waypoints.length < 2) {
      console.warn('FlightMap.addRoute: waypoints must be an array with at least 2 points', waypoints);
      return;
    }

    if (!this.map) {
      console.error('FlightMap.addRoute: Map not initialized');
      return;
    }

    if (!this.routeLayerGroup) {
      console.error('FlightMap.addRoute: routeLayerGroup not initialized, creating it');
      this.routeLayerGroup = L.layerGroup().addTo(this.map);
    }

    // Clear existing route
    this.clearRoute();

    const { colorBy = 'altitude', color = null } = options;

    console.log(`FlightMap.addRoute: Processing ${waypoints.length} waypoints`);

    // Create polylines for each segment with color based on altitude
    let validSegments = 0;
    for (let i = 0; i < waypoints.length - 1; i++) {
      const point1 = waypoints[i];
      const point2 = waypoints[i + 1];

      // Validate coordinates
      if (point1.latitude == null || point1.longitude == null ||
          point2.latitude == null || point2.longitude == null) {
        console.warn(`FlightMap.addRoute: Skipping segment ${i} - invalid coordinates`, {
          point1: { lat: point1.latitude, lng: point1.longitude },
          point2: { lat: point2.latitude, lng: point2.longitude }
        });
        continue;
      }

      // Additional validation: check if coordinates are valid numbers
      const lat1 = Number(point1.latitude);
      const lng1 = Number(point1.longitude);
      const lat2 = Number(point2.latitude);
      const lng2 = Number(point2.longitude);

      if (isNaN(lat1) || isNaN(lng1) || isNaN(lat2) || isNaN(lng2)) {
        console.warn(`FlightMap.addRoute: Skipping segment ${i} - NaN coordinates`);
        continue;
      }

      if (lat1 < -90 || lat1 > 90 || lat2 < -90 || lat2 > 90 ||
          lng1 < -180 || lng1 > 180 || lng2 < -180 || lng2 > 180) {
        console.warn(`FlightMap.addRoute: Skipping segment ${i} - coordinates out of range`);
        continue;
      }

      const segment = [
        [lat1, lng1],
        [lat2, lng2]
      ];

      // Determine color
      let segmentColor;
      if (color) {
        segmentColor = color;
      } else if (colorBy === 'altitude') {
        const altitude1 = point1.altitude != null ? Number(point1.altitude) : 0;
        const altitude2 = point2.altitude != null ? Number(point2.altitude) : 0;
        const avgAltitude = (altitude1 + altitude2) / 2;
        segmentColor = getAltitudeColor(avgAltitude, this.maxAltitude);
      } else {
        segmentColor = '#3b82f6'; // Default blue
      }

      // Create polyline segment with more visible styling
      const polyline = L.polyline(segment, {
        color: segmentColor,
        weight: Math.max(this.routeLineWidth, 3), // Ensure minimum width of 3px
        opacity: 0.9,
        lineCap: 'round',
        lineJoin: 'round',
        interactive: false // Don't capture mouse events
      });

      // Add to route layer group
      this.routeLayerGroup.addLayer(polyline);
      this.routePolylines.push(polyline);
      validSegments++;

      // Debug first and last segments
      if (i === 0) {
        console.log('FlightMap: First segment', {
          from: segment[0],
          to: segment[1],
          color: segmentColor,
          polyline: polyline
        });
      }
      if (i === waypoints.length - 2) {
        console.log('FlightMap: Last segment', {
          from: segment[0],
          to: segment[1],
          color: segmentColor
        });
      }
    }

    console.log(`FlightMap: Added route with ${validSegments} valid segments out of ${waypoints.length - 1} possible`);

    // Ensure route layer is visible and on top
    if (!this.map.hasLayer(this.routeLayerGroup)) {
      console.warn('FlightMap: routeLayerGroup not on map, adding it');
      this.routeLayerGroup.addTo(this.map);
    }

    // Bring each polyline segment to front to ensure visibility
    // (bringToFront() is not available on layer groups, only on individual layers)
    this.routePolylines.forEach(polyline => {
      try {
        polyline.bringToFront();
      } catch (e) {
        // Ignore errors if polyline is not fully initialized
        console.debug('Could not bring polyline to front:', e);
      }
    });

    // Invalidate map size to ensure polylines are rendered
    if (validSegments > 0) {
      console.log('FlightMap: Route rendering complete', {
        segmentsCreated: validSegments,
        polylinesInArray: this.routePolylines.length,
        layersInGroup: this.routeLayerGroup.getLayers().length,
        mapHasLayer: this.map.hasLayer(this.routeLayerGroup)
      });

      // Force a redraw
      this.map.invalidateSize();
      
      // Also try after a short delay in case map needs to update
      setTimeout(() => {
        this.map.invalidateSize();
        console.log('FlightMap: Map invalidated, route should be visible');
        console.log('FlightMap: Route layer group has', this.routeLayerGroup.getLayers().length, 'layers');
        
        // Verify polylines are actually on the map
        this.routePolylines.forEach((polyline, idx) => {
          const bounds = polyline.getBounds();
          console.log(`FlightMap: Polyline ${idx} bounds:`, bounds.toBBoxString());
        });
      }, 100);
    } else {
      console.warn('FlightMap: No valid segments created!', {
        waypointsLength: waypoints.length,
        waypointsSample: waypoints.slice(0, 3)
      });
    }

    // Fit map bounds to show the route
    const routeBounds = waypoints
      .filter(wp => {
        const lat = Number(wp.latitude);
        const lng = Number(wp.longitude);
        return !isNaN(lat) && !isNaN(lng) && 
               lat >= -90 && lat <= 90 && 
               lng >= -180 && lng <= 180;
      })
      .map(wp => [Number(wp.latitude), Number(wp.longitude)]);

    if (routeBounds.length > 0) {
      try {
        this.map.fitBounds(routeBounds, {
          padding: [50, 50],
          maxZoom: 18,
          minZoom: 2
        });
        console.log(`FlightMap: Fitted bounds to route with ${routeBounds.length} valid points`);
      } catch (error) {
        console.error('FlightMap: Error fitting bounds', error);
      }
    } else {
      console.warn('FlightMap: No valid route bounds to fit');
    }
  }

  /**
   * Clear the route from the map
   */
  clearRoute() {
    this.routePolylines.forEach(polyline => {
      this.routeLayerGroup.removeLayer(polyline);
    });
    this.routePolylines = [];
  }

  /**
   * Set callback for flight click events
   * @param {Function} callback - Callback function that receives flight object
   */
  onFlightClick(callback) {
    this.onFlightClickCallback = callback;
  }

  /**
   * Get the underlying Leaflet map instance
   * @returns {L.Map} Leaflet map instance
   */
  getMap() {
    return this.map;
  }

  /**
   * Destroy the map instance and clean up
   */
  destroy() {
    this.clearFlights();
    this.clearRoute();
    this.map.remove();
  }
}
