-- Migration 009: Create Events tables
-- Description: Add Events functionality with multi-leg events for Virtual Airlines

-- Create events table for managing event events
CREATE TABLE IF NOT EXISTS public.events (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    va_id UUID NOT NULL REFERENCES virtual_airlines(id) ON DELETE CASCADE,
    name VARCHAR(255) NOT NULL,
    description TEXT,
    status VARCHAR(20) DEFAULT 'draft' CHECK (status IN ('draft', 'active', 'completed', 'cancelled')),
    start_date TIMESTAMPTZ,
    end_date TIMESTAMPTZ,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW(),
    created_by_id UUID REFERENCES users(id) ON DELETE SET NULL,
    updated_by_id UUID REFERENCES users(id) ON DELETE SET NULL,
    
    -- Constraints
    CONSTRAINT event_name_not_empty CHECK (LENGTH(TRIM(name)) > 0),
    CONSTRAINT event_dates_valid CHECK (
        start_date IS NULL OR
        end_date IS NULL OR
        start_date < end_date
    )
);

-- Create event_legs table for individual event legs
CREATE TABLE IF NOT EXISTS public.event_legs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    event_id UUID NOT NULL REFERENCES events(id) ON DELETE CASCADE,
    leg_number INTEGER NOT NULL,
    origin VARCHAR(10) NOT NULL,
    destination VARCHAR(10) NOT NULL,
    route_at_id VARCHAR(20),
    description TEXT,
    thumbnail_url TEXT,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW(),
    created_by_id UUID REFERENCES users(id) ON DELETE SET NULL,
    updated_by_id UUID REFERENCES users(id) ON DELETE SET NULL,
    
    -- Constraints
    CONSTRAINT leg_number_positive CHECK (leg_number > 0),
    CONSTRAINT origin_not_empty CHECK (LENGTH(TRIM(origin)) > 0),
    CONSTRAINT destination_not_empty CHECK (LENGTH(TRIM(destination)) > 0),
    
    -- Unique leg number per event
    CONSTRAINT unique_leg_per_event UNIQUE (event_id, leg_number),
    
    -- Unique origin-destination combination per event (prevents duplicate routes)
    CONSTRAINT unique_route_per_event UNIQUE (event_id, origin, destination)
);

-- Create indexes for efficient queries

-- Events indexes
CREATE INDEX IF NOT EXISTS idx_events_va_id ON public.events(va_id);
CREATE INDEX IF NOT EXISTS idx_events_status ON public.events(va_id, status);
CREATE INDEX IF NOT EXISTS idx_events_date_range ON public.events(start_date, end_date);
CREATE INDEX IF NOT EXISTS idx_events_created_by ON public.events(created_by_id);
CREATE INDEX IF NOT EXISTS idx_events_updated_by ON public.events(updated_by_id);

-- Event Legs indexes  
CREATE INDEX IF NOT EXISTS idx_event_legs_event_id ON public.event_legs(event_id);
CREATE INDEX IF NOT EXISTS idx_event_legs_leg_number ON public.event_legs(event_id, leg_number);
CREATE INDEX IF NOT EXISTS idx_event_legs_route_at_id ON public.event_legs(route_at_id);
CREATE INDEX IF NOT EXISTS idx_event_legs_created_by ON public.event_legs(created_by_id);
CREATE INDEX IF NOT EXISTS idx_event_legs_updated_by ON public.event_legs(updated_by_id);

-- Add updated_at trigger for events
CREATE OR REPLACE FUNCTION update_events_timestamp()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS trigger_events_updated_at ON public.events;
CREATE TRIGGER trigger_events_updated_at
BEFORE UPDATE ON public.events
FOR EACH ROW
EXECUTE FUNCTION update_events_timestamp();

-- Add updated_at trigger for event_legs
CREATE OR REPLACE FUNCTION update_event_legs_timestamp()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS trigger_event_legs_updated_at ON public.event_legs;
CREATE TRIGGER trigger_event_legs_updated_at
BEFORE UPDATE ON public.event_legs
FOR EACH ROW
EXECUTE FUNCTION update_event_legs_timestamp();

-- Add table comments for documentation
COMMENT ON TABLE public.events IS 'Multi-leg events for Virtual Airlines - used for group flights, special events, etc.';
COMMENT ON TABLE public.event_legs IS 'Individual legs of events with route information and Airtable integration';

-- Add column comments for key fields
COMMENT ON COLUMN public.events.status IS 'Event status: draft, active, completed, cancelled';
COMMENT ON COLUMN public.events.start_date IS 'Event start date/time - NULL means event starts immediately. Event is active when current time >= start_date (or start_date is NULL).';
COMMENT ON COLUMN public.events.end_date IS 'Event end date/time - NULL means event never expires. Event is active when current time <= end_date (or end_date is NULL).';
COMMENT ON COLUMN public.event_legs.origin IS 'ICAO code of origin airport (2-4 characters)';
COMMENT ON COLUMN public.event_legs.destination IS 'ICAO code of destination airport (2-4 characters)';
COMMENT ON COLUMN public.event_legs.route_at_id IS 'Resolved Airtable route ID from route_at_sync table';
COMMENT ON COLUMN public.event_legs.leg_number IS 'Sequential leg number for ordered progression';
