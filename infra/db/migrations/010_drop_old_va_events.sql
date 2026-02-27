-- Migration 010: Drop old va_events table
-- Description: Remove deprecated va_events table and related objects

-- Drop triggers
DROP TRIGGER IF EXISTS trigger_va_events_updated_at ON public.va_events;

-- Drop function (if not used elsewhere)
DROP FUNCTION IF EXISTS update_va_events_timestamp();

-- Drop indexes
DROP INDEX IF EXISTS idx_va_events_va_id;
DROP INDEX IF EXISTS idx_va_events_date_range;
DROP INDEX IF EXISTS idx_va_events_created_by;
DROP INDEX IF EXISTS idx_va_events_route_at_id;

-- Drop table
DROP TABLE IF EXISTS public.va_events;
