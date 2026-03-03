-- Migration 013: Add flight_mode column to events table
-- Description: Add flight_mode field to parent event for tour PIREP submission
-- This allows setting a single flight mode for all legs in an event

-- Add flight_mode column (nullable for backward compatibility)
ALTER TABLE public.events 
ADD COLUMN flight_mode VARCHAR(100);

-- Add column comment
COMMENT ON COLUMN public.events.flight_mode IS 
'Flight mode identifier for the event (e.g., "World Tour 10"). Used when submitting tour PIREPs to Airtable. Applies to all legs in the event.';
