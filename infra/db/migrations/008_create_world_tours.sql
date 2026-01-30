-- Migration 008: Create World Tours tables
-- Description: Add World Tours functionality with multi-leg tour events for Virtual Airlines

-- Create world_tours table for managing tour events
CREATE TABLE IF NOT EXISTS public.world_tours (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    va_id UUID NOT NULL REFERENCES virtual_airlines(id) ON DELETE CASCADE,
    name VARCHAR(255) NOT NULL,
    description TEXT,
    documentation_url TEXT,
    status VARCHAR(20) DEFAULT 'draft' CHECK (status IN ('draft', 'active', 'completed', 'cancelled')),
    flight_mode_key VARCHAR(100) NOT NULL, -- Used for filtering PIREPs (e.g., "world_tour_9")
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW(),
    created_by_id UUID REFERENCES users(id) ON DELETE SET NULL,
    
    -- Constraints
    CONSTRAINT tour_name_not_empty CHECK (LENGTH(TRIM(name)) > 0),
    CONSTRAINT flight_mode_key_not_empty CHECK (LENGTH(TRIM(flight_mode_key)) > 0),
    
    -- One active tour per VA at a time
    CONSTRAINT unique_active_tour_per_va 
        EXCLUDE (va_id WITH =) WHERE (status = 'active'),
    
    -- Unique flight mode key per VA (prevents reuse)
    CONSTRAINT unique_flight_mode_per_va UNIQUE (va_id, flight_mode_key)
);

-- Create world_tour_legs table for individual tour legs
CREATE TABLE IF NOT EXISTS public.world_tour_legs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    world_tour_id UUID NOT NULL REFERENCES world_tours(id) ON DELETE CASCADE,
    leg_number INTEGER NOT NULL,
    name VARCHAR(255) NOT NULL, -- Display name: "Amsterdam to Krakow"
    route_name VARCHAR(255) NOT NULL, -- Free text route: "EHAM-EPKK" (for matching)
    route_at_id VARCHAR(20), -- Resolved Airtable route ID from route_at_sync table
    description TEXT,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW(),
    
    -- Constraints
    CONSTRAINT leg_number_positive CHECK (leg_number > 0),
    CONSTRAINT leg_name_not_empty CHECK (LENGTH(TRIM(name)) > 0),
    CONSTRAINT route_name_not_empty CHECK (LENGTH(TRIM(route_name)) > 0),
    
    -- Unique leg number per tour
    CONSTRAINT unique_leg_per_tour UNIQUE (world_tour_id, leg_number),
    
    -- Unique route per tour (prevents duplicate routes)
    CONSTRAINT unique_route_per_tour UNIQUE (world_tour_id, route_name)
);

-- Create indexes for efficient queries

-- World Tours indexes
CREATE INDEX IF NOT EXISTS idx_world_tours_va_id ON public.world_tours(va_id);
CREATE INDEX IF NOT EXISTS idx_world_tours_status ON public.world_tours(va_id, status);
CREATE INDEX IF NOT EXISTS idx_world_tours_flight_mode ON public.world_tours(va_id, flight_mode_key);
CREATE INDEX IF NOT EXISTS idx_world_tours_created_at ON public.world_tours(created_at DESC);

-- World Tour Legs indexes  
CREATE INDEX IF NOT EXISTS idx_world_tour_legs_tour_id ON public.world_tour_legs(world_tour_id);
CREATE INDEX IF NOT EXISTS idx_world_tour_legs_leg_number ON public.world_tour_legs(world_tour_id, leg_number);
CREATE INDEX IF NOT EXISTS idx_world_tour_legs_route_at_id ON public.world_tour_legs(route_at_id);
CREATE INDEX IF NOT EXISTS idx_world_tour_legs_route_name ON public.world_tour_legs(world_tour_id, route_name);

-- Add table comments for documentation
COMMENT ON TABLE public.world_tours IS 'Multi-leg tour events for Virtual Airlines with flight mode integration for PIREP tracking';
COMMENT ON TABLE public.world_tour_legs IS 'Individual legs of world tours with route information and Airtable integration';

-- Add column comments for key fields
COMMENT ON COLUMN public.world_tours.flight_mode_key IS 'Used to filter PIREPs for progress tracking (e.g., "world_tour_9")';
COMMENT ON COLUMN public.world_tours.status IS 'Tour status: draft, active, completed, cancelled';
COMMENT ON COLUMN public.world_tour_legs.route_name IS 'Free text route identifier for matching against route_at_sync.route';
COMMENT ON COLUMN public.world_tour_legs.route_at_id IS 'Resolved Airtable route ID from route_at_sync table';
COMMENT ON COLUMN public.world_tour_legs.leg_number IS 'Sequential leg number for ordered progression';