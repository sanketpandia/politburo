-- Migration 011: Refactor data provider configs with config_type
-- Description: Drop old tables and create new structure with separate config types

-- Drop existing tables (CASCADE to remove dependent objects)
DROP TABLE IF EXISTS public.va_provider_validation_history CASCADE;
DROP TABLE IF EXISTS public.va_data_provider_configs CASCADE;

-- Create new va_data_provider_configs table with config_type column
CREATE TABLE public.va_data_provider_configs (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    va_id uuid NOT NULL,
    provider_type character varying(50) NOT NULL,
    config_type character varying(50) NOT NULL,  -- 'credentials', 'route', 'pilot', 'pirep', etc.
    config_data jsonb NOT NULL,
    config_version integer DEFAULT 1 NOT NULL,
    is_active boolean DEFAULT false NOT NULL,
    validation_status public.validation_status DEFAULT 'pending'::public.validation_status NOT NULL,
    features_enabled text[] DEFAULT '{}'::text[],
    last_validated_at timestamp without time zone,
    validation_errors jsonb,
    created_at timestamp without time zone DEFAULT now() NOT NULL,
    updated_at timestamp without time zone DEFAULT now() NOT NULL,
    created_by uuid,
    updated_by uuid,
    CONSTRAINT va_data_provider_configs_pkey PRIMARY KEY (id)
);

-- Create indexes
CREATE INDEX idx_va_provider_configs_va_provider_type ON public.va_data_provider_configs USING btree (va_id, provider_type);
CREATE INDEX idx_va_provider_configs_config_type ON public.va_data_provider_configs USING btree (va_id, provider_type, config_type);
CREATE INDEX idx_va_provider_configs_active ON public.va_data_provider_configs USING btree (va_id, is_active);
CREATE INDEX idx_va_provider_configs_data ON public.va_data_provider_configs USING gin (config_data);
CREATE INDEX idx_va_provider_configs_errors ON public.va_data_provider_configs USING gin (validation_errors);
CREATE INDEX idx_va_provider_configs_features ON public.va_data_provider_configs USING gin (features_enabled);
CREATE INDEX idx_va_provider_configs_provider ON public.va_data_provider_configs USING btree (provider_type);

-- Create va_provider_validation_history table
CREATE TABLE public.va_provider_validation_history (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    config_id uuid NOT NULL,
    validation_status public.validation_status NOT NULL,
    validation_errors jsonb,
    phases_completed text[],
    phases_failed text[],
    duration_ms integer,
    validated_at timestamp without time zone DEFAULT now() NOT NULL,
    triggered_by character varying(50)
);

-- Create index on validation history
CREATE INDEX idx_va_provider_validation_history_config_id ON public.va_provider_validation_history USING btree (config_id);
CREATE INDEX idx_va_provider_validation_history_status ON public.va_provider_validation_history USING btree (validation_status);
