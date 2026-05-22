# Live Cache Job And Live Flights UI Handoff

Date: 2026-05-22
Branch: `feat/live-flights-redesign`
Relevant commits:
- `f9b99b0 Redesign live flights dashboard`
- `cfb9d80 Commit remaining code changes`

## Purpose

This note captures implementation context for the live flight cache job and the `/dashboard/live` Vizburo UI so future work can continue without rediscovering the same details.

## Live Cache Job Notes

- Live flight cache work is centered in `internal/flights/`.
- The cache/session processing flow now has code changes across:
  - `internal/flights/cache_session_processor.go`
  - `internal/flights/complete_flight_builder.go`
  - `internal/flights/dto.go`
  - `internal/flights/flight_phase.go`
  - `internal/flights/flight_plan_worker.go`
  - `internal/flights/handler.go`
  - `internal/flights/model.go`
  - `internal/flights/service.go`
  - `internal/routes/router.go`
- The Live Flights UI depends on cached live-flight DTO data being available as JSON in `templates/pages/live.html` via the `live-flights-data` script tag.
- Route and flown-path display depends on the dashboard flight path endpoint used by `static/js/live-flights.mjs`:
  - `GET /dashboard/flights/{flightID}/paths`
  - Expected response shape can be a plain payload or envelope with `data`/`result`.
  - Expected payload fields are `flight_plan` and `flown_route`, each an array of points with `latitude` and `longitude`.
- The map supports named paths through `FlightMap.addPath(name, waypoints, options)` and `FlightMap.clearPath(name)`, so planned and flown routes can be rendered independently.
- Current UI rendering assumes live flight objects may include:
  - `flight_id`
  - `callsign`
  - `username`
  - `session_name`
  - `latitude`
  - `longitude`
  - `altitude`
  - `speed`
  - `track`
  - `vertical_speed`
  - `aircraft_name`
  - `livery_name`
  - `origin`
  - `destination`
  - `phase`
  - `phase_history`
  - `takeoff_time`
  - `last_report`
  - `max_altitude`
  - `max_speed`

## Phase Handling

- UI phase classes normalize phase strings by lowercasing and replacing whitespace with underscores.
- Known timeline phases rendered by the UI:
  - `on_ground`
  - `takeoff`
  - `climb`
  - `cruise`
  - `descent`
  - `landing`
- Phase history should be an array if possible. The UI accepts entries with either:
  - `ph` or `phase`
  - `at` or `changed_at`
- The UI no longer renders phase history as a raw string. It renders a timeline and highlights the current phase.

## Live Flights UI Notes

- Main page: `templates/pages/live.html`.
- Reusable partials:
  - `templates/partials/flight-list.html`
  - `templates/partials/live-map-shell.html`
  - `templates/partials/map-legend.html`
  - `templates/partials/flight-details-panel.html`
  - `templates/partials/flight-details-sheet.html`
- Browser behavior lives in `static/js/live-flights.mjs`.
- Reusable Leaflet map wrapper lives in `static/js/flight-map.mjs`.
- Styling lives in `static/css/design-system.css` using existing Vizburo tokens.

## Desktop Layout

- Desktop is a three-region dashboard:
  - left flight list
  - center map
  - right inspector
- Selecting a flight:
  - highlights the list row
  - updates the inspector
  - focuses the map aircraft
  - loads planned/flown paths
  - emphasizes the selected aircraft marker
- The inspector is collapsible. Collapsing does not clear selection or map paths.
- Map actions are present for center/follow/view route. They all refocus or reload selected-flight path context.

## Mobile Layout

- Mobile uses segmented tabs: `List`, `Map`, `Details`.
- Default markup has `List` active.
- Tapping a flight selects it and moves to the map tab.
- A selected-flight mini dock appears on mobile with callsign, route, altitude/speed, phase, and an `Open map` action.
- Details use a dismissible bottom sheet, not a blocking modal.
- The bottom sheet can be collapsed back to map with the sheet control.

## CSS Caution

- `static/css/design-system.css` had unrelated pre-existing changes in the worktree when the Live Flights UI was implemented.
- The live-flight commit staged only live-flight-related CSS hunks.
- Be careful when reviewing or cherry-picking this branch because later commit `cfb9d80` intentionally includes the remaining code files requested by the user, while docs/plans/artifacts were left unstaged.

## Validation Already Run

- `go test ./infra/templates`
- `go build -buildvcs=false -o .air_tmp/main ./cmd/server`

## Follow-Up Ideas

- Add focused handler/service tests around the `/dashboard/flights/{flightID}/paths` response shape.
- Add an explicit UI empty state when a selected flight has no flight plan or flown route.
- Consider converting filter chips into real HTMX-backed filters if the active-flight list grows large.
- Consider a small component test or browser smoke check for mobile tab and bottom-sheet behavior.
