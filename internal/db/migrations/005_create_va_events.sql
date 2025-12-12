-- Create va_events table for managing time-bound events with predefined routes
CREATE TABLE IF NOT EXISTS public.va_events (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    va_id UUID NOT NULL REFERENCES virtual_airlines(id) ON DELETE CASCADE,
    event_name VARCHAR(100) NOT NULL,
    description TEXT,
    predefined_route VARCHAR(20) NOT NULL,
    start_date TIMESTAMPTZ NOT NULL,
    end_date TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW(),
    created_by UUID REFERENCES users(id) ON DELETE SET NULL,

    -- Constraints
    CONSTRAINT event_dates_valid CHECK (start_date < end_date),
    CONSTRAINT event_name_not_empty CHECK (LENGTH(TRIM(event_name)) > 0),
    CONSTRAINT predefined_route_not_empty CHECK (LENGTH(TRIM(predefined_route)) > 0)
);

-- Indexes for efficient queries
CREATE INDEX IF NOT EXISTS idx_va_events_va_id ON public.va_events(va_id);
CREATE INDEX IF NOT EXISTS idx_va_events_date_range ON public.va_events(start_date, end_date);
CREATE INDEX IF NOT EXISTS idx_va_events_created_by ON public.va_events(created_by);

-- Add updated_at trigger to automatically update timestamp on modification
CREATE OR REPLACE FUNCTION update_va_events_timestamp()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS trigger_va_events_updated_at ON public.va_events;
CREATE TRIGGER trigger_va_events_updated_at
BEFORE UPDATE ON public.va_events
FOR EACH ROW
EXECUTE FUNCTION update_va_events_timestamp();

-- Add comments
COMMENT ON TABLE public.va_events IS 'Events with predefined routes for VAs - used for group flights, special events, etc.';
COMMENT ON COLUMN public.va_events.predefined_route IS 'Format: ICAO-ICAO (e.g., EGLL-EHAM) - the route that qualifies for this event';
COMMENT ON COLUMN public.va_events.start_date IS 'Event start date/time - event is active when current time >= start_date';
COMMENT ON COLUMN public.va_events.end_date IS 'Event end date/time - event is active when current time <= end_date';
