--
-- Backfill existing routes with airport coordinates
-- Parses the route field (format: KJFK-EGLL) to extract origin and destination ICAOs
-- Then joins with airports table to populate lat/lon coordinates
--

WITH parsed_routes AS (
  SELECT
    id,
    SPLIT_PART(route, '-', 1) AS origin_icao,
    SPLIT_PART(route, '-', 2) AS dest_icao
  FROM route_at_synced
  WHERE route IS NOT NULL AND route != '' AND route LIKE '%-%'
)
UPDATE route_at_synced r
SET
  origin_lat = ao.latitude,
  origin_lon = ao.longitude,
  destination_lat = ad.latitude,
  destination_lon = ad.longitude,
  updated_at = now()
FROM parsed_routes pr
LEFT JOIN airports ao ON UPPER(ao.icao) = UPPER(pr.origin_icao)
LEFT JOIN airports ad ON UPPER(ad.icao) = UPPER(pr.dest_icao)
WHERE r.id = pr.id
  AND r.origin_lat IS NULL;  -- Only update if not already populated
