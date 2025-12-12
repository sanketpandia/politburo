-- Add route_at_id column to link events to Airtable routes
-- This allows events to reference the specific Airtable route record
-- Following the same pattern as pirep_at_synced.route_at_id

ALTER TABLE public.va_events
ADD COLUMN route_at_id VARCHAR(20);

-- Create index for efficient lookups by VA and route
CREATE INDEX IF NOT EXISTS idx_va_events_route_at_id
ON public.va_events(va_id, route_at_id);

-- Update comments
COMMENT ON COLUMN public.va_events.route_at_id IS 'Airtable ID of the route record. Allows linking to route_at_synced table and Airtable data.';
