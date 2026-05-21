import { FlightMap } from './flight-map.mjs';

const dataNode = document.getElementById('live-flights-data');
const flights = dataNode ? JSON.parse(dataNode.textContent || '[]') : [];
const byID = new Map(flights.map((flight) => [flight.flight_id, flight]));
const listItems = Array.from(document.querySelectorAll('[data-flight-id]'));
const searchInput = document.querySelector('[data-flight-search]');
const searchEmpty = document.querySelector('[data-flight-search-empty]');
const detailsPanel = document.querySelector('[data-flight-details-panel]');
const detailsEmpty = document.querySelector('[data-flight-details-empty]');
const detailsContent = document.querySelector('[data-flight-details-content]');
const sheet = document.querySelector('[data-flight-details-sheet]');
const sheetContent = document.querySelector('[data-flight-details-sheet-content]');
const loading = document.querySelector('[data-map-loading]');
const workspace = document.querySelector('[data-live-workspace]');
const tabButtons = Array.from(document.querySelectorAll('[data-live-tab]'));
const dock = document.querySelector('[data-selected-flight-dock]');

let flightMap = null;
let selectedFlightID = null;
let inspectorCollapsed = false;

function formatRoute(flight) {
  const origin = flight.origin || '—';
  const destination = flight.destination || '—';
  return `${origin} → ${destination}`;
}

function formatTime(value) {
  if (!value) return '—';
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return '—';
  return date.toLocaleString(undefined, { dateStyle: 'short', timeStyle: 'short' });
}

function formatNumber(value, unit = '', digits = 0) {
  if (value === null || value === undefined || value === '') return '—';
  const number = Number(value);
  if (Number.isNaN(number)) return '—';
  return `${number.toLocaleString(undefined, { maximumFractionDigits: digits })}${unit ? ` ${unit}` : ''}`;
}

function formatPosition(flight) {
  if (flight.latitude === null || flight.latitude === undefined || flight.longitude === null || flight.longitude === undefined) return '—';
  return `${Number(flight.latitude).toFixed(4)}, ${Number(flight.longitude).toFixed(4)}`;
}

function normalizePhase(phase) {
  return String(phase || 'unknown').toLowerCase().replace(/\s+/g, '_');
}

function formatPhaseLabel(phase) {
  const value = normalizePhase(phase).replace(/_/g, ' ');
  return value.replace(/\b\w/g, (char) => char.toUpperCase());
}

function getPhaseHistory(flight) {
  const history = Array.isArray(flight.phase_history) ? flight.phase_history : [];
  const currentPhase = normalizePhase(flight?.phase);
  const phases = ['on_ground', 'takeoff', 'climb', 'cruise', 'descent', 'landing'];
  const seen = new Map(history.map((entry) => [normalizePhase(entry.ph || entry.phase), entry]));
  return phases.map((phase) => ({
    phase,
    label: formatPhaseLabel(phase),
    current: phase === currentPhase,
    complete: seen.has(phase) || phases.indexOf(phase) <= phases.indexOf(currentPhase),
    timestamp: seen.get(phase)?.at || seen.get(phase)?.changed_at || null,
  }));
}

function escapeHTML(value) {
  return String(value ?? '').replace(/[&<>'"]/g, (char) => ({
    '&': '&amp;',
    '<': '&lt;',
    '>': '&gt;',
    "'": '&#39;',
    '"': '&quot;',
  }[char]));
}

function renderDetails(flight) {
  if (!flight) return '';
  const aircraft = `${flight.aircraft_name || '—'}${flight.livery_name ? ` · ${flight.livery_name}` : ''}`;
  const timeline = renderPhaseTimeline(flight);
  return `
    <div class="aircraft-summary-card">
      <div class="sheet-title-row">
        <div>
          <h2>${escapeHTML(flight.callsign || 'Unknown callsign')}</h2>
          <p class="detail-muted">${escapeHTML(flight.username || 'Unknown pilot')}</p>
        </div>
        <span class="status-pill phase-${escapeHTML(normalizePhase(flight.phase))}">${escapeHTML(formatPhaseLabel(flight.phase))}</span>
      </div>
      <p class="aircraft-line">${escapeHTML(aircraft)}</p>
      <div class="route-strip"><strong>${escapeHTML(flight.origin || '—')}</strong><span>→</span><strong>${escapeHTML(flight.destination || '—')}</strong></div>
      <p class="detail-muted">${escapeHTML(flight.session_name || '—')}</p>
    </div>
    <dl class="detail-grid metric-grid">
      <div><dt>Altitude</dt><dd>${escapeHTML(formatNumber(flight.altitude, 'ft'))}</dd></div>
      <div><dt>Speed</dt><dd>${escapeHTML(formatNumber(flight.speed, 'kt'))}</dd></div>
      <div><dt>Vertical speed</dt><dd>${escapeHTML(formatNumber(flight.vertical_speed, 'fpm', 1))}</dd></div>
      <div><dt>Track</dt><dd>${escapeHTML(formatNumber(flight.track, 'deg', 1))}</dd></div>
      <div><dt>Position</dt><dd>${escapeHTML(formatPosition(flight))}</dd></div>
      <div><dt>Max altitude</dt><dd>${escapeHTML(formatNumber(flight.max_altitude, 'ft'))}</dd></div>
      <div><dt>Max speed</dt><dd>${escapeHTML(formatNumber(flight.max_speed, 'kt'))}</dd></div>
      <div><dt>Takeoff</dt><dd>${escapeHTML(formatTime(flight.takeoff_time))}</dd></div>
      <div><dt>Last report</dt><dd>${escapeHTML(formatTime(flight.last_report))}</dd></div>
    </dl>
    <div class="mobile-sheet-actions">
      <button class="btn btn-secondary" type="button" data-center-selected-flight>Center</button>
      <button class="btn btn-secondary" type="button" data-view-selected-route>View route</button>
    </div>
    <section class="phase-history-card"><h3>Phase History</h3><div class="phase-timeline">${timeline}</div></section>`;
}

function renderPhaseTimeline(flight) {
  return getPhaseHistory(flight).map((item) => `
    <div class="phase-step ${item.complete ? 'complete' : ''} ${item.current ? 'current' : ''}">
      <span class="phase-node" aria-hidden="true"></span>
      <strong>${escapeHTML(item.label)}</strong>
      <small>${escapeHTML(formatTime(item.timestamp))}</small>
    </div>
  `).join('');
}

function setText(selector, value) {
  const node = detailsPanel?.querySelector(selector);
  if (node) node.textContent = value;
}

function selectFlight(flightID, focusMap = true) {
  const flight = byID.get(flightID);
  if (!flight) return;
  selectedFlightID = flightID;
  listItems.forEach((item) => item.classList.toggle('active', item.dataset.flightId === flightID));
  inspectorCollapsed = false;
  detailsPanel?.classList.add('open');
  detailsPanel?.classList.remove('collapsed');
  workspace?.classList.remove('inspector-collapsed');
  detailsEmpty?.classList.add('hidden');
  detailsContent?.classList.remove('hidden');
  const phaseNode = detailsPanel?.querySelector('[data-detail-phase]');
  if (phaseNode) {
    phaseNode.textContent = formatPhaseLabel(flight.phase);
    phaseNode.className = `status-pill phase-${normalizePhase(flight.phase)}`;
  }
  setText('[data-detail-callsign]', flight.callsign || 'Unknown callsign');
  setText('[data-detail-pilot]', flight.username || 'Unknown pilot');
  setText('[data-detail-origin]', flight.origin || '—');
  setText('[data-detail-destination]', flight.destination || '—');
  setText('[data-detail-aircraft]', `${flight.aircraft_name || '—'}${flight.livery_name ? ` · ${flight.livery_name}` : ''}`);
  setText('[data-detail-session]', flight.session_name || '—');
  setText('[data-detail-altitude]', formatNumber(flight.altitude, 'ft'));
  setText('[data-detail-speed]', formatNumber(flight.speed, 'kt'));
  setText('[data-detail-vs]', formatNumber(flight.vertical_speed, 'fpm', 1));
  setText('[data-detail-track]', formatNumber(flight.track, 'deg', 1));
  setText('[data-detail-position]', formatPosition(flight));
  setText('[data-detail-max-altitude]', formatNumber(flight.max_altitude, 'ft'));
  setText('[data-detail-max-speed]', formatNumber(flight.max_speed, 'kt'));
  const timeline = detailsPanel?.querySelector('[data-detail-phase-history]');
  if (timeline) timeline.innerHTML = renderPhaseTimeline(flight);
  setText('[data-detail-takeoff]', formatTime(flight.takeoff_time));
  setText('[data-detail-report]', formatTime(flight.last_report));
  updateDock(flight);
  if (sheet && sheetContent) {
    sheetContent.innerHTML = renderDetails(flight);
  }
  if (focusMap && flightMap) {
    flightMap.focusFlight(flightID, { zoom: 8, flightData: flight });
    showMobileSection('map');
  }
  loadFlightPaths(flightID);
}

function updateDock(flight) {
  if (!dock) return;
  dock.classList.remove('hidden');
  dock.querySelector('[data-dock-callsign]').textContent = flight.callsign || 'Unknown callsign';
  dock.querySelector('[data-dock-meta]').textContent = `${formatRoute(flight)} · ${formatNumber(flight.altitude, 'ft')} · ${formatNumber(flight.speed, 'kt')}`;
  dock.querySelector('[data-dock-phase]').textContent = formatPhaseLabel(flight.phase);
}

function showMobileSection(name) {
  document.querySelector('.live-page')?.setAttribute('data-mobile-view', name);
  tabButtons.forEach((button) => {
    const active = button.dataset.liveTab === name;
    button.classList.toggle('active', active);
    button.setAttribute('aria-pressed', String(active));
  });
  if (name === 'details' && selectedFlightID) {
    sheet?.classList.add('open');
    sheet?.setAttribute('aria-hidden', 'false');
  } else if (name !== 'details') {
    sheet?.classList.remove('open');
    sheet?.setAttribute('aria-hidden', 'true');
  }
  setTimeout(() => flightMap?.getMap()?.invalidateSize(), 50);
}

function collapseInspector() {
  inspectorCollapsed = true;
  detailsPanel?.classList.add('collapsed');
  workspace?.classList.add('inspector-collapsed');
  sheet?.classList.remove('open');
  sheet?.setAttribute('aria-hidden', 'true');
  setTimeout(() => flightMap?.getMap()?.invalidateSize(), 150);
}

function reopenInspector() {
  inspectorCollapsed = false;
  detailsPanel?.classList.remove('collapsed');
  workspace?.classList.remove('inspector-collapsed');
  showMobileSection('details');
  setTimeout(() => flightMap?.getMap()?.invalidateSize(), 150);
}

async function loadFlightPaths(flightID) {
  if (!flightMap) return;
  flightMap.clearPath('flight-plan');
  flightMap.clearPath('flown-route');

  try {
    const response = await fetch(`/dashboard/flights/${encodeURIComponent(flightID)}/paths`, { headers: { Accept: 'application/json' } });
    if (!response.ok) return;
    const envelope = await response.json();
    const paths = envelope.data || envelope.result || envelope;
    if (selectedFlightID !== flightID) return;

    const flightPlan = Array.isArray(paths.flight_plan) ? paths.flight_plan : [];
    const flownRoute = Array.isArray(paths.flown_route) ? paths.flown_route : [];
    if (flightPlan.length > 1) {
      flightMap.addPath('flight-plan', flightPlan, { color: '#8b5cf6', weight: 3, opacity: 0.85, dashArray: '8 8' });
    }
    if (flownRoute.length > 1) {
      flightMap.addPath('flown-route', flownRoute, { color: '#22c55e', weight: 4, opacity: 0.95, fit: true });
    }
  } catch (error) {
    console.warn('Failed to load cached flight paths', error);
  }
}

function closeDetails() {
  collapseInspector();
}

function filterFlights(query) {
  const normalized = query.trim().toLowerCase();
  let visibleCount = 0;
  listItems.forEach((item) => {
    const flight = byID.get(item.dataset.flightId);
    const haystack = [flight?.callsign, flight?.username, flight?.origin, flight?.destination, flight?.phase, flight?.session_name]
      .filter(Boolean)
      .join(' ')
      .toLowerCase();
    const visible = !normalized || haystack.includes(normalized);
    item.classList.toggle('hidden', !visible);
    if (visible) visibleCount++;
  });
  searchEmpty?.classList.toggle('hidden', visibleCount !== 0 || !normalized);
}

listItems.forEach((item) => item.addEventListener('click', () => selectFlight(item.dataset.flightId)));
searchInput?.addEventListener('input', () => filterFlights(searchInput.value));
document.querySelectorAll('[data-close-flight-details]').forEach((button) => button.addEventListener('click', closeDetails));
document.querySelectorAll('[data-reopen-flight-details]').forEach((button) => button.addEventListener('click', reopenInspector));
document.querySelectorAll('[data-collapse-mobile-sheet]').forEach((button) => button.addEventListener('click', () => showMobileSection('map')));
document.querySelectorAll('[data-open-mobile-map]').forEach((button) => button.addEventListener('click', () => showMobileSection('map')));
document.querySelectorAll('[data-collapse-mobile-dock]').forEach((button) => button.addEventListener('click', () => dock?.classList.add('hidden')));
tabButtons.forEach((button) => button.addEventListener('click', () => showMobileSection(button.dataset.liveTab)));
document.addEventListener('click', (event) => {
  const action = event.target.closest('[data-center-selected-flight], [data-follow-selected-flight], [data-view-selected-route]');
  if (!action || !selectedFlightID) return;
  const flight = byID.get(selectedFlightID);
  if (flightMap && flight) flightMap.focusFlight(selectedFlightID, { zoom: 8, flightData: flight });
  if (action.matches('[data-view-selected-route]')) loadFlightPaths(selectedFlightID);
});
document.querySelectorAll('[data-return-to-flight-list]').forEach((button) => button.addEventListener('click', () => {
  showMobileSection('list');
  document.getElementById('live-flight-list')?.scrollIntoView({ behavior: 'smooth', block: 'start' });
}));
document.addEventListener('keydown', (event) => {
  if (event.key === 'Escape') closeDetails();
});

if (document.getElementById('leaflet-map') && window.L) {
  flightMap = new FlightMap('leaflet-map', { center: [20, 0], zoom: 2 });
  flightMap.addFlights(flights);
  flightMap.onFlightClick((flight) => selectFlight(flight.flight_id, false));
  loading?.classList.add('hidden');
}
