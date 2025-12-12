-- Make event dates nullable to support indefinite events
-- NULL start_date = event starts immediately when created
-- NULL end_date = event never expires

-- First, drop the existing constraint that requires dates
ALTER TABLE public.va_events DROP CONSTRAINT event_dates_valid;

-- Alter columns to be nullable
ALTER TABLE public.va_events
    ALTER COLUMN start_date DROP NOT NULL,
    ALTER COLUMN end_date DROP NOT NULL;

-- Add new constraint that allows nulls
-- If both dates are present, start_date must be before end_date
-- If one or both are NULL, the constraint is satisfied
ALTER TABLE public.va_events
    ADD CONSTRAINT event_dates_valid CHECK (
        start_date IS NULL OR
        end_date IS NULL OR
        start_date < end_date
    );

-- Update index comments to reflect nullable columns
COMMENT ON COLUMN public.va_events.start_date IS 'Event start date/time - NULL means event starts immediately. Event is active when current time >= start_date (or start_date is NULL).';
COMMENT ON COLUMN public.va_events.end_date IS 'Event end date/time - NULL means event never expires. Event is active when current time <= end_date (or end_date is NULL).';
