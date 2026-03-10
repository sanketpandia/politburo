-- Migration 015: Add career_mode_pilot_id column to va_user_roles table and pilot_type enum to pilot_at_synced
-- Description: Allow users to have separate career mode pilot records linked via Airtable ID
--              Add pilot_type enum to distinguish regular vs career mode pilots in pilot_at_synced table

-- Create pilot_type enum
CREATE TYPE public.pilot_type AS ENUM (
    'regular',
    'career_mode'
);

-- Add career_mode_pilot_id column to va_user_roles
ALTER TABLE va_user_roles 
ADD COLUMN career_mode_pilot_id VARCHAR(20);

COMMENT ON COLUMN va_user_roles.career_mode_pilot_id IS 'Airtable record ID for career mode pilot record (separate from regular pilot record)';

-- Add pilot_type column to pilot_at_synced table with default 'regular' for existing records
ALTER TABLE pilot_at_synced
ADD COLUMN pilot_type public.pilot_type DEFAULT 'regular' NOT NULL;

COMMENT ON COLUMN pilot_at_synced.pilot_type IS 'Type of pilot record: regular (normal flights/events) or career_mode (career mode progression)';
