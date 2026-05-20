import { FlightMap } from './flight-map.mjs';

const dataNode = document.getElementById('live-flights-data');
const flights = dataNode ? JSON.parse(dataNode.textContent || '[]') : [];
const byID = new Map(flights.map((flight) => [flight.flight_id, flight]));
const listItems = Array.from(document.querySelectorAll('[data-flight-id]'));
const detailsPanel = document.querySelector('[data-flight-details-panel]');
const detailsEmpty = document.querySelector('[data-flight-details-empty]');
const detailsContent = document.querySelector('[data-flight-details-content]');
const sheet = document.querySelector('[data-flight-details-sheet]');
const sheetContent = document.querySelector('[data-flight-details-sheet-content]');
const workspace = document.querySelector('[data-live-workspace]');
const loading = document.querySelector('[data-map-loading]');

let flightMap = null;

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
  return `
    <p class="eyebrow">${escapeHTML(flight.phase || 'unknown')}</p>
    <h2>${escapeHTML(flight.callsign || 'Unknown callsign')}</h2>
    <p class="detail-muted">${escapeHTML(flight.username || 'Unknown pilot')}</p>
    <dl class="detail-grid">
      <div><dt>Route</dt><dd>${escapeHTML(formatRoute(flight))}</dd></div>
      <div><dt>Aircraft</dt><dd>${escapeHTML(aircraft)}</dd></div>
      <div><dt>Altitude</dt><dd>${flight.altitude ?? '—'} ft</dd></div>
      <div><dt>Speed</dt><dd>${flight.speed ?? '—'} kt</dd></div>
      <div><dt>Vertical speed</dt><dd>${flight.vertical_speed ?? '—'} fpm</dd></div>
      <div><dt>Last report</dt><dd>${escapeHTML(formatTime(flight.last_report))}</dd></div>
    </dl>`;
}

function setText(selector, value) {
  const node = detailsPanel?.querySelector(selector);
  if (node) node.textContent = value;
}

function selectFlight(flightID, focusMap = true) {
  const flight = byID.get(flightID);
  if (!flight) return;
  listItems.forEach((item) => item.classList.toggle('active', item.dataset.flightId === flightID));
  detailsPanel?.classList.add('open');
  detailsEmpty?.classList.add('hidden');
  detailsContent?.classList.remove('hidden');
  setText('[data-detail-phase]', flight.phase || 'unknown');
  setText('[data-detail-callsign]', flight.callsign || 'Unknown callsign');
  setText('[data-detail-pilot]', flight.username || 'Unknown pilot');
  setText('[data-detail-route]', formatRoute(flight));
  setText('[data-detail-aircraft]', `${flight.aircraft_name || '—'}${flight.livery_name ? ` · ${flight.livery_name}` : ''}`);
  setText('[data-detail-altitude]', `${flight.altitude ?? '—'} ft`);
  setText('[data-detail-speed]', `${flight.speed ?? '—'} kt`);
  setText('[data-detail-vs]', `${flight.vertical_speed ?? '—'} fpm`);
  setText('[data-detail-report]', formatTime(flight.last_report));
  if (sheet && sheetContent) {
    sheetContent.innerHTML = renderDetails(flight);
    sheet.classList.add('open');
    sheet.setAttribute('aria-hidden', 'false');
  }
  if (focusMap && flightMap) {
    flightMap.focusFlight(flightID, { zoom: 8, flightData: flight });
  }
}

function closeDetails() {
  detailsPanel?.classList.remove('open');
  detailsEmpty?.classList.remove('hidden');
  detailsContent?.classList.add('hidden');
  sheet?.classList.remove('open');
  sheet?.setAttribute('aria-hidden', 'true');
  listItems.forEach((item) => item.classList.remove('active'));
}

function setMobileView(view) {
  workspace?.setAttribute('data-active-view', view);
  document.querySelectorAll('[data-live-view]').forEach((button) => {
    button.classList.toggle('active', button.dataset.liveView === view);
  });
  if (view === 'map' && flightMap) {
    setTimeout(() => flightMap.getMap().invalidateSize(), 50);
  }
}

listItems.forEach((item) => item.addEventListener('click', () => selectFlight(item.dataset.flightId)));
document.querySelectorAll('[data-close-flight-details]').forEach((button) => button.addEventListener('click', closeDetails));
document.querySelectorAll('[data-live-view]').forEach((button) => button.addEventListener('click', () => setMobileView(button.dataset.liveView)));
document.addEventListener('keydown', (event) => {
  if (event.key === 'Escape') closeDetails();
});

if (document.getElementById('leaflet-map') && window.L) {
  flightMap = new FlightMap('leaflet-map', { center: [20, 0], zoom: 2 });
  flightMap.addFlights(flights);
  flightMap.onFlightClick((flight) => selectFlight(flight.flight_id, false));
  loading?.classList.add('hidden');
}

setMobileView('list');
