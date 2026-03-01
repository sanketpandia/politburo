-- Migration 012: Add unique constraint on if_community_id
-- Description: Ensures each IFC ID can only be registered to one Discord user
-- Note: NULL values are allowed (multiple users can have NULL if_community_id per SQL standard)

-- Add unique constraint on if_community_id
-- This will fail if there are existing duplicate IFC IDs in the database
-- In that case, duplicates must be resolved before running this migration
-- PostgreSQL automatically creates an index for unique constraints
ALTER TABLE public.users 
ADD CONSTRAINT users_if_community_id_key UNIQUE (if_community_id);
