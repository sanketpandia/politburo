-- Migration 011: Add additional_data JSONB column to event_legs table
-- This allows flexible storage of leg metadata (aircrafts, livery, fpl, hero image, SID/STAR, cruise speed, terminals, duration, url, etc.)

-- Add JSONB column with default empty object
ALTER TABLE public.event_legs 
ADD COLUMN additional_data JSONB DEFAULT '{}'::jsonb;

-- Create GIN index for efficient JSON queries
CREATE INDEX IF NOT EXISTS idx_event_legs_additional_data 
ON public.event_legs USING GIN (additional_data);

-- Add column comment explaining usage
COMMENT ON COLUMN public.event_legs.additional_data IS 
'JSONB field for flexible leg metadata (aircrafts, livery, fpl, hero image, SID/STAR, cruise speed, terminals, duration, url, etc.)';
