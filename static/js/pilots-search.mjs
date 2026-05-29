/**
 * Pilots search module - filters the pilots table by callsign, username, and role
 */

let pilotsTable = null;
let pilotRows = [];
let searchInput = null;

function getPilotRows() {
  pilotsTable = document.querySelector('.pilots-table');
  if (!pilotsTable) return [];
  return Array.from(pilotsTable.querySelectorAll('[data-pilot-row]'));
}

function filterPilots(query) {
  const normalized = query.trim().toLowerCase();
  let visibleCount = 0;

  pilotRows.forEach((row) => {
    const username = row.dataset.pilotUsername?.toLowerCase() || '';
    const callsign = row.dataset.pilotCallsign?.toLowerCase() || '';
    const role = row.dataset.pilotRole?.toLowerCase() || '';

    // Search in all three fields
    const haystack = `${username} ${callsign} ${role}`;
    const visible = !normalized || haystack.includes(normalized);

    row.classList.toggle('hidden', !visible);
    if (visible) visibleCount++;
  });

  // Show/hide empty state
  updateEmptyState(visibleCount, normalized);
}

function updateEmptyState(visibleCount, hasQuery) {
  const tableContainer = document.getElementById('pilots-container');
  let emptyState = tableContainer?.querySelector('[data-pilots-search-empty]');

  // Remove old empty state if it exists
  if (emptyState) emptyState.remove();

  // If no results and there's a search query, show empty state
  if (visibleCount === 0 && hasQuery) {
    const pilotsTable = document.querySelector('.pilots-table');
    if (pilotsTable) {
      emptyState = document.createElement('div');
      emptyState.className = 'empty-state compact';
      emptyState.setAttribute('data-pilots-search-empty', '');
      emptyState.innerHTML = '<h3>No matching pilots</h3><p>Try another callsign, username, or role.</p>';
      pilotsTable.parentNode.appendChild(emptyState);
    }
  }
}

function attachSearchListener() {
  searchInput = document.querySelector('[data-pilots-search]');
  if (searchInput) {
    searchInput.addEventListener('input', () => filterPilots(searchInput.value));
  }
}

/**
 * Initialize pilots search functionality
 * Should be called when the pilots table is loaded (on page load)
 * Also needs to be called after HTMX swaps the table content
 */
export function initPilotsSearch() {
  // Initial setup on page load
  pilotRows = getPilotRows();
  attachSearchListener();

  // Re-initialize whenever HTMX swaps the content
  document.addEventListener('htmx:afterSwap', (event) => {
    // Only reinitialize if this is a pilots-related swap
    if (event.detail.target?.id === 'pilots-container') {
      pilotRows = getPilotRows();
      attachSearchListener();
      // Reset search input value to trigger re-filter if needed
      const searchValue = document.querySelector('[data-pilots-search]')?.value || '';
      if (searchValue) {
        filterPilots(searchValue);
      }
    }
  });
}
