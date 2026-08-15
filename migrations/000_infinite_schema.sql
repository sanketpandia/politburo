--
-- PostgreSQL database dump
--


-- Dumped from database version 15.15 (Debian 15.15-1.pgdg13+1)
-- Dumped by pg_dump version 15.15 (Debian 15.15-1.pgdg13+1)

SET statement_timeout = 0;
SET lock_timeout = 0;
SET idle_in_transaction_session_timeout = 0;
SET client_encoding = 'UTF8';
SET standard_conforming_strings = on;
SELECT pg_catalog.set_config('search_path', '', false);
SET check_function_bodies = false;
SET xmloption = content;
SET client_min_messages = warning;
SET row_security = off;

--
-- Name: pilot_type; Type: TYPE; Schema: public; Owner: ieuser
--

CREATE TYPE public.pilot_type AS ENUM (
    'regular',
    'career_mode'
);



--
-- Name: va_role; Type: TYPE; Schema: public; Owner: ieuser
--

CREATE TYPE public.va_role AS ENUM (
    'pilot',
    'staff',
    'admin'
);



--
-- Name: validation_status; Type: TYPE; Schema: public; Owner: ieuser
--

CREATE TYPE public.validation_status AS ENUM (
    'pending',
    'validating',
    'valid',
    'invalid'
);



--
-- Name: update_event_legs_timestamp(); Type: FUNCTION; Schema: public; Owner: ieuser
--

CREATE FUNCTION public.update_event_legs_timestamp() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$;



--
-- Name: update_events_timestamp(); Type: FUNCTION; Schema: public; Owner: ieuser
--

CREATE FUNCTION public.update_events_timestamp() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$;



SET default_tablespace = '';

SET default_table_access_method = heap;

--
-- Name: aircraft_liveries; Type: TABLE; Schema: public; Owner: ieuser
--

CREATE TABLE public.aircraft_liveries (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    livery_id character varying(100) NOT NULL,
    aircraft_id character varying(100) NOT NULL,
    aircraft_name text NOT NULL,
    livery_name text NOT NULL,
    is_active boolean DEFAULT true,
    created_at timestamp without time zone DEFAULT now(),
    updated_at timestamp without time zone DEFAULT now(),
    last_synced_at timestamp without time zone DEFAULT now()
);



--
-- Name: airports; Type: TABLE; Schema: public; Owner: ieuser
--

CREATE TABLE public.airports (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    icao character varying(4) NOT NULL,
    iata character varying(3),
    name text NOT NULL,
    city character varying(100),
    country character varying(100),
    elevation integer,
    latitude numeric(10,6) NOT NULL,
    longitude numeric(10,6) NOT NULL,
    timezone character varying(50),
    created_at timestamp without time zone DEFAULT now(),
    updated_at timestamp without time zone DEFAULT now()
);



--
-- Name: api_keys; Type: TABLE; Schema: public; Owner: ieuser
--

CREATE TABLE public.api_keys (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    status boolean DEFAULT false
);



--
-- Name: event_legs; Type: TABLE; Schema: public; Owner: ieuser
--

CREATE TABLE public.event_legs (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    event_id uuid NOT NULL,
    leg_number integer NOT NULL,
    origin character varying(10) NOT NULL,
    destination character varying(10) NOT NULL,
    route_at_id character varying(20),
    description text,
    thumbnail_url text,
    created_at timestamp with time zone DEFAULT now(),
    updated_at timestamp with time zone DEFAULT now(),
    created_by_id uuid,
    updated_by_id uuid,
    additional_data jsonb DEFAULT '{}'::jsonb,
    CONSTRAINT destination_not_empty CHECK ((length(TRIM(BOTH FROM destination)) > 0)),
    CONSTRAINT leg_number_positive CHECK ((leg_number > 0)),
    CONSTRAINT origin_not_empty CHECK ((length(TRIM(BOTH FROM origin)) > 0))
);



--
-- Name: events; Type: TABLE; Schema: public; Owner: ieuser
--

CREATE TABLE public.events (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    va_id uuid NOT NULL,
    name character varying(255) NOT NULL,
    description text,
    status character varying(20) DEFAULT 'draft'::character varying,
    start_date timestamp with time zone,
    end_date timestamp with time zone,
    created_at timestamp with time zone DEFAULT now(),
    updated_at timestamp with time zone DEFAULT now(),
    created_by_id uuid,
    updated_by_id uuid,
    flight_mode character varying(100),
    CONSTRAINT event_dates_valid CHECK (((start_date IS NULL) OR (end_date IS NULL) OR (start_date < end_date))),
    CONSTRAINT event_name_not_empty CHECK ((length(TRIM(BOTH FROM name)) > 0)),
    CONSTRAINT events_status_check CHECK (((status)::text = ANY (ARRAY[('draft'::character varying)::text, ('active'::character varying)::text, ('completed'::character varying)::text, ('cancelled'::character varying)::text])))
);



--
-- Name: livery_airtable_mappings; Type: TABLE; Schema: public; Owner: ieuser
--

CREATE TABLE public.livery_airtable_mappings (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    va_id uuid NOT NULL,
    livery_id character varying(255) NOT NULL,
    field_type character varying(50) NOT NULL,
    source_value character varying(255) NOT NULL,
    target_value character varying(255) NOT NULL,
    is_active boolean DEFAULT true,
    created_at timestamp without time zone DEFAULT CURRENT_TIMESTAMP,
    updated_at timestamp without time zone DEFAULT CURRENT_TIMESTAMP
);



--
-- Name: pilot_at_synced; Type: TABLE; Schema: public; Owner: ieuser
--

CREATE TABLE public.pilot_at_synced (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    at_id character varying(20) NOT NULL,
    callsign character varying(20),
    registered boolean DEFAULT false,
    server_id uuid,
    pilot_type public.pilot_type DEFAULT 'regular'::public.pilot_type NOT NULL
);



--
-- Name: pirep_at_synced; Type: TABLE; Schema: public; Owner: ieuser
--

CREATE TABLE public.pirep_at_synced (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    at_id character varying(20) NOT NULL,
    server_id uuid NOT NULL,
    route text,
    flight_mode character varying(50),
    flight_time numeric(10,2),
    pilot_callsign character varying(50),
    aircraft character varying(100),
    livery character varying(100),
    route_at_id character varying(20),
    pilot_at_id character varying(20),
    at_created_time timestamp without time zone,
    created_at timestamp without time zone DEFAULT now(),
    updated_at timestamp without time zone DEFAULT now(),
    backfill_status integer DEFAULT 0 NOT NULL
);



--
-- Name: route_at_synced; Type: TABLE; Schema: public; Owner: ieuser
--

CREATE TABLE public.route_at_synced (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    at_id character varying(20) NOT NULL,
    server_id uuid NOT NULL,
    origin character varying(10),
    destination character varying(10),
    created_at timestamp without time zone DEFAULT now(),
    updated_at timestamp without time zone DEFAULT now(),
    route character varying(100),
    origin_lat numeric(10,6),
    origin_lon numeric(10,6),
    destination_lat numeric(10,6),
    destination_lon numeric(10,6)
);



--
-- Name: users; Type: TABLE; Schema: public; Owner: ieuser
--

CREATE TABLE public.users (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    discord_id character varying(32) NOT NULL,
    if_community_id character varying(30),
    if_api_id uuid,
    is_active boolean DEFAULT false,
    username text,
    created_at timestamp without time zone DEFAULT now(),
    updated_at timestamp without time zone DEFAULT now(),
    otp character varying(6)
);



--
-- Name: va_configs; Type: TABLE; Schema: public; Owner: ieuser
--

CREATE TABLE public.va_configs (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    va_id uuid,
    config_key character varying(50) NOT NULL,
    config_value text NOT NULL,
    created_at timestamp without time zone DEFAULT now(),
    updated_at timestamp without time zone DEFAULT now()
);



--
-- Name: va_data_provider_configs; Type: TABLE; Schema: public; Owner: ieuser
--

CREATE TABLE public.va_data_provider_configs (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    va_id uuid NOT NULL,
    provider_type character varying(50) NOT NULL,
    config_type character varying(50) NOT NULL,
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
    updated_by uuid
);



--
-- Name: va_provider_validation_history; Type: TABLE; Schema: public; Owner: ieuser
--

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



--
-- Name: va_sync_history; Type: TABLE; Schema: public; Owner: ieuser
--

CREATE TABLE public.va_sync_history (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    va_id uuid NOT NULL,
    event character varying(50) NOT NULL,
    created_at timestamp without time zone DEFAULT now(),
    last_sync_at timestamp without time zone
);



--
-- Name: va_user_roles; Type: TABLE; Schema: public; Owner: ieuser
--

CREATE TABLE public.va_user_roles (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    user_id uuid,
    va_id uuid,
    role public.va_role NOT NULL,
    is_active boolean DEFAULT true,
    joined_at timestamp without time zone DEFAULT now(),
    airtable_pilot_id character varying(20),
    callsign character varying(20),
    updated_at timestamp with time zone DEFAULT (now() AT TIME ZONE 'UTC'::text) NOT NULL,
    career_mode_pilot_id character varying(20)
);



--
-- Name: va_webhooks; Type: TABLE; Schema: public; Owner: ieuser
--

CREATE TABLE public.va_webhooks (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    va_id uuid NOT NULL,
    webhook_type character varying(50) NOT NULL,
    webhook_url text NOT NULL,
    frequency_minutes integer DEFAULT 30 NOT NULL,
    is_active boolean DEFAULT true NOT NULL,
    label character varying(255),
    created_at timestamp without time zone DEFAULT now() NOT NULL,
    updated_at timestamp without time zone DEFAULT now() NOT NULL
);



--
-- Name: virtual_airlines; Type: TABLE; Schema: public; Owner: ieuser
--

CREATE TABLE public.virtual_airlines (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    name text NOT NULL,
    code character varying(30) NOT NULL,
    is_active boolean DEFAULT true,
    created_at timestamp without time zone DEFAULT now(),
    updated_at timestamp without time zone DEFAULT now(),
    discord_server_id character varying(32),
    is_airtable_enabled boolean DEFAULT false,
    flight_modes_config jsonb DEFAULT '{}'::jsonb
);



--
-- Name: aircraft_liveries aircraft_liveries_livery_id_key; Type: CONSTRAINT; Schema: public; Owner: ieuser
--

ALTER TABLE ONLY public.aircraft_liveries
    ADD CONSTRAINT aircraft_liveries_livery_id_key UNIQUE (livery_id);


--
-- Name: aircraft_liveries aircraft_liveries_pkey; Type: CONSTRAINT; Schema: public; Owner: ieuser
--

ALTER TABLE ONLY public.aircraft_liveries
    ADD CONSTRAINT aircraft_liveries_pkey PRIMARY KEY (id);


--
-- Name: airports airports_icao_key; Type: CONSTRAINT; Schema: public; Owner: ieuser
--

ALTER TABLE ONLY public.airports
    ADD CONSTRAINT airports_icao_key UNIQUE (icao);


--
-- Name: airports airports_pkey; Type: CONSTRAINT; Schema: public; Owner: ieuser
--

ALTER TABLE ONLY public.airports
    ADD CONSTRAINT airports_pkey PRIMARY KEY (id);


--
-- Name: api_keys api_keys_pkey; Type: CONSTRAINT; Schema: public; Owner: ieuser
--

ALTER TABLE ONLY public.api_keys
    ADD CONSTRAINT api_keys_pkey PRIMARY KEY (id);


--
-- Name: event_legs event_legs_pkey; Type: CONSTRAINT; Schema: public; Owner: ieuser
--

ALTER TABLE ONLY public.event_legs
    ADD CONSTRAINT event_legs_pkey PRIMARY KEY (id);


--
-- Name: events events_pkey; Type: CONSTRAINT; Schema: public; Owner: ieuser
--

ALTER TABLE ONLY public.events
    ADD CONSTRAINT events_pkey PRIMARY KEY (id);


--
-- Name: livery_airtable_mappings livery_airtable_mappings_pkey; Type: CONSTRAINT; Schema: public; Owner: ieuser
--

ALTER TABLE ONLY public.livery_airtable_mappings
    ADD CONSTRAINT livery_airtable_mappings_pkey PRIMARY KEY (id);


--
-- Name: livery_airtable_mappings livery_airtable_mappings_va_id_livery_id_field_type_key; Type: CONSTRAINT; Schema: public; Owner: ieuser
--

ALTER TABLE ONLY public.livery_airtable_mappings
    ADD CONSTRAINT livery_airtable_mappings_va_id_livery_id_field_type_key UNIQUE (va_id, livery_id, field_type);


--
-- Name: pilot_at_synced pilot_at_synced_pkey; Type: CONSTRAINT; Schema: public; Owner: ieuser
--

ALTER TABLE ONLY public.pilot_at_synced
    ADD CONSTRAINT pilot_at_synced_pkey PRIMARY KEY (id);


--
-- Name: pilot_at_synced pilot_at_synced_server_at_id_key; Type: CONSTRAINT; Schema: public; Owner: ieuser
--

ALTER TABLE ONLY public.pilot_at_synced
    ADD CONSTRAINT pilot_at_synced_server_at_id_key UNIQUE (server_id, at_id);


--
-- Name: pirep_at_synced pirep_at_synced_pkey; Type: CONSTRAINT; Schema: public; Owner: ieuser
--

ALTER TABLE ONLY public.pirep_at_synced
    ADD CONSTRAINT pirep_at_synced_pkey PRIMARY KEY (id);


--
-- Name: pirep_at_synced pirep_at_synced_unique; Type: CONSTRAINT; Schema: public; Owner: ieuser
--

ALTER TABLE ONLY public.pirep_at_synced
    ADD CONSTRAINT pirep_at_synced_unique UNIQUE (server_id, at_id);


--
-- Name: route_at_synced route_at_synced_pkey; Type: CONSTRAINT; Schema: public; Owner: ieuser
--

ALTER TABLE ONLY public.route_at_synced
    ADD CONSTRAINT route_at_synced_pkey PRIMARY KEY (id);


--
-- Name: route_at_synced route_at_synced_unique; Type: CONSTRAINT; Schema: public; Owner: ieuser
--

ALTER TABLE ONLY public.route_at_synced
    ADD CONSTRAINT route_at_synced_unique UNIQUE (server_id, at_id);


--
-- Name: event_legs unique_leg_per_event; Type: CONSTRAINT; Schema: public; Owner: ieuser
--

ALTER TABLE ONLY public.event_legs
    ADD CONSTRAINT unique_leg_per_event UNIQUE (event_id, leg_number);


--
-- Name: event_legs unique_route_per_event; Type: CONSTRAINT; Schema: public; Owner: ieuser
--

ALTER TABLE ONLY public.event_legs
    ADD CONSTRAINT unique_route_per_event UNIQUE (event_id, origin, destination);


--
-- Name: users users_discord_id_key; Type: CONSTRAINT; Schema: public; Owner: ieuser
--

ALTER TABLE ONLY public.users
    ADD CONSTRAINT users_discord_id_key UNIQUE (discord_id);


--
-- Name: users users_if_community_id_key; Type: CONSTRAINT; Schema: public; Owner: ieuser
--

ALTER TABLE ONLY public.users
    ADD CONSTRAINT users_if_community_id_key UNIQUE (if_community_id);


--
-- Name: users users_pkey; Type: CONSTRAINT; Schema: public; Owner: ieuser
--

ALTER TABLE ONLY public.users
    ADD CONSTRAINT users_pkey PRIMARY KEY (id);


--
-- Name: va_configs va_configs_pkey; Type: CONSTRAINT; Schema: public; Owner: ieuser
--

ALTER TABLE ONLY public.va_configs
    ADD CONSTRAINT va_configs_pkey PRIMARY KEY (id);


--
-- Name: va_configs va_configs_va_id_config_key_key; Type: CONSTRAINT; Schema: public; Owner: ieuser
--

ALTER TABLE ONLY public.va_configs
    ADD CONSTRAINT va_configs_va_id_config_key_key UNIQUE (va_id, config_key);


--
-- Name: va_data_provider_configs va_data_provider_configs_pkey; Type: CONSTRAINT; Schema: public; Owner: ieuser
--

ALTER TABLE ONLY public.va_data_provider_configs
    ADD CONSTRAINT va_data_provider_configs_pkey PRIMARY KEY (id);


--
-- Name: va_sync_history va_sync_history_pkey; Type: CONSTRAINT; Schema: public; Owner: ieuser
--

ALTER TABLE ONLY public.va_sync_history
    ADD CONSTRAINT va_sync_history_pkey PRIMARY KEY (id);


--
-- Name: va_user_roles va_user_roles_pkey; Type: CONSTRAINT; Schema: public; Owner: ieuser
--

ALTER TABLE ONLY public.va_user_roles
    ADD CONSTRAINT va_user_roles_pkey PRIMARY KEY (id);


--
-- Name: va_webhooks va_webhooks_pkey; Type: CONSTRAINT; Schema: public; Owner: ieuser
--

ALTER TABLE ONLY public.va_webhooks
    ADD CONSTRAINT va_webhooks_pkey PRIMARY KEY (id);


--
-- Name: virtual_airlines virtual_airlines_code_key; Type: CONSTRAINT; Schema: public; Owner: ieuser
--

ALTER TABLE ONLY public.virtual_airlines
    ADD CONSTRAINT virtual_airlines_code_key UNIQUE (code);


--
-- Name: virtual_airlines virtual_airlines_discord_server_id_key; Type: CONSTRAINT; Schema: public; Owner: ieuser
--

ALTER TABLE ONLY public.virtual_airlines
    ADD CONSTRAINT virtual_airlines_discord_server_id_key UNIQUE (discord_server_id);


--
-- Name: virtual_airlines virtual_airlines_pkey; Type: CONSTRAINT; Schema: public; Owner: ieuser
--

ALTER TABLE ONLY public.virtual_airlines
    ADD CONSTRAINT virtual_airlines_pkey PRIMARY KEY (id);


--
-- Name: idx_aircraft_liveries_active; Type: INDEX; Schema: public; Owner: ieuser
--

CREATE INDEX idx_aircraft_liveries_active ON public.aircraft_liveries USING btree (is_active);


--
-- Name: idx_aircraft_liveries_active_livery; Type: INDEX; Schema: public; Owner: ieuser
--

CREATE INDEX idx_aircraft_liveries_active_livery ON public.aircraft_liveries USING btree (is_active, livery_id);


--
-- Name: idx_aircraft_liveries_aircraft_id; Type: INDEX; Schema: public; Owner: ieuser
--

CREATE INDEX idx_aircraft_liveries_aircraft_id ON public.aircraft_liveries USING btree (aircraft_id);


--
-- Name: idx_aircraft_liveries_livery_id; Type: INDEX; Schema: public; Owner: ieuser
--

CREATE INDEX idx_aircraft_liveries_livery_id ON public.aircraft_liveries USING btree (livery_id);


--
-- Name: idx_aircraft_liveries_sync; Type: INDEX; Schema: public; Owner: ieuser
--

CREATE INDEX idx_aircraft_liveries_sync ON public.aircraft_liveries USING btree (last_synced_at);


--
-- Name: idx_airports_coordinates; Type: INDEX; Schema: public; Owner: ieuser
--

CREATE INDEX idx_airports_coordinates ON public.airports USING btree (latitude, longitude);


--
-- Name: idx_airports_iata; Type: INDEX; Schema: public; Owner: ieuser
--

CREATE INDEX idx_airports_iata ON public.airports USING btree (iata);


--
-- Name: idx_airports_icao; Type: INDEX; Schema: public; Owner: ieuser
--

CREATE INDEX idx_airports_icao ON public.airports USING btree (icao);


--
-- Name: idx_event_legs_additional_data; Type: INDEX; Schema: public; Owner: ieuser
--

CREATE INDEX idx_event_legs_additional_data ON public.event_legs USING gin (additional_data);


--
-- Name: idx_event_legs_created_by; Type: INDEX; Schema: public; Owner: ieuser
--

CREATE INDEX idx_event_legs_created_by ON public.event_legs USING btree (created_by_id);


--
-- Name: idx_event_legs_event_id; Type: INDEX; Schema: public; Owner: ieuser
--

CREATE INDEX idx_event_legs_event_id ON public.event_legs USING btree (event_id);


--
-- Name: idx_event_legs_leg_number; Type: INDEX; Schema: public; Owner: ieuser
--

CREATE INDEX idx_event_legs_leg_number ON public.event_legs USING btree (event_id, leg_number);


--
-- Name: idx_event_legs_route_at_id; Type: INDEX; Schema: public; Owner: ieuser
--

CREATE INDEX idx_event_legs_route_at_id ON public.event_legs USING btree (route_at_id);


--
-- Name: idx_event_legs_updated_by; Type: INDEX; Schema: public; Owner: ieuser
--

CREATE INDEX idx_event_legs_updated_by ON public.event_legs USING btree (updated_by_id);


--
-- Name: idx_events_created_by; Type: INDEX; Schema: public; Owner: ieuser
--

CREATE INDEX idx_events_created_by ON public.events USING btree (created_by_id);


--
-- Name: idx_events_date_range; Type: INDEX; Schema: public; Owner: ieuser
--

CREATE INDEX idx_events_date_range ON public.events USING btree (start_date, end_date);


--
-- Name: idx_events_status; Type: INDEX; Schema: public; Owner: ieuser
--

CREATE INDEX idx_events_status ON public.events USING btree (va_id, status);


--
-- Name: idx_events_updated_by; Type: INDEX; Schema: public; Owner: ieuser
--

CREATE INDEX idx_events_updated_by ON public.events USING btree (updated_by_id);


--
-- Name: idx_events_va_id; Type: INDEX; Schema: public; Owner: ieuser
--

CREATE INDEX idx_events_va_id ON public.events USING btree (va_id);


--
-- Name: idx_livery_mappings_lookup; Type: INDEX; Schema: public; Owner: ieuser
--

CREATE INDEX idx_livery_mappings_lookup ON public.livery_airtable_mappings USING btree (va_id, field_type, source_value);


--
-- Name: idx_livery_mappings_va_id; Type: INDEX; Schema: public; Owner: ieuser
--

CREATE INDEX idx_livery_mappings_va_id ON public.livery_airtable_mappings USING btree (va_id);


--
-- Name: idx_livery_mappings_va_livery; Type: INDEX; Schema: public; Owner: ieuser
--

CREATE INDEX idx_livery_mappings_va_livery ON public.livery_airtable_mappings USING btree (va_id, livery_id);


--
-- Name: idx_pirep_at_synced_at_created_time; Type: INDEX; Schema: public; Owner: ieuser
--

CREATE INDEX idx_pirep_at_synced_at_created_time ON public.pirep_at_synced USING btree (server_id, at_created_time DESC);


--
-- Name: idx_pirep_at_synced_flight_mode; Type: INDEX; Schema: public; Owner: ieuser
--

CREATE INDEX idx_pirep_at_synced_flight_mode ON public.pirep_at_synced USING btree (server_id, flight_mode);


--
-- Name: idx_pirep_at_synced_pilot_at_id; Type: INDEX; Schema: public; Owner: ieuser
--

CREATE INDEX idx_pirep_at_synced_pilot_at_id ON public.pirep_at_synced USING btree (server_id, pilot_at_id);


--
-- Name: idx_pirep_at_synced_pilot_callsign; Type: INDEX; Schema: public; Owner: ieuser
--

CREATE INDEX idx_pirep_at_synced_pilot_callsign ON public.pirep_at_synced USING btree (server_id, pilot_callsign);


--
-- Name: idx_pirep_at_synced_route_at_id; Type: INDEX; Schema: public; Owner: ieuser
--

CREATE INDEX idx_pirep_at_synced_route_at_id ON public.pirep_at_synced USING btree (server_id, route_at_id);


--
-- Name: idx_pirep_at_synced_server_id; Type: INDEX; Schema: public; Owner: ieuser
--

CREATE INDEX idx_pirep_at_synced_server_id ON public.pirep_at_synced USING btree (server_id);


--
-- Name: idx_route_at_synced_coordinates; Type: INDEX; Schema: public; Owner: ieuser
--

CREATE INDEX idx_route_at_synced_coordinates ON public.route_at_synced USING btree (origin_lat, origin_lon, destination_lat, destination_lon);


--
-- Name: idx_users_discord_id; Type: INDEX; Schema: public; Owner: ieuser
--

CREATE UNIQUE INDEX idx_users_discord_id ON public.users USING btree (discord_id);


--
-- Name: idx_users_username; Type: INDEX; Schema: public; Owner: ieuser
--

CREATE INDEX idx_users_username ON public.users USING btree (username);


--
-- Name: idx_va_configs_va_id; Type: INDEX; Schema: public; Owner: ieuser
--

CREATE INDEX idx_va_configs_va_id ON public.va_configs USING btree (va_id);


--
-- Name: idx_va_configs_va_key; Type: INDEX; Schema: public; Owner: ieuser
--

CREATE INDEX idx_va_configs_va_key ON public.va_configs USING btree (va_id, config_key);


--
-- Name: idx_va_discord_server_id; Type: INDEX; Schema: public; Owner: ieuser
--

CREATE UNIQUE INDEX idx_va_discord_server_id ON public.virtual_airlines USING btree (discord_server_id);


--
-- Name: idx_va_provider_configs_active; Type: INDEX; Schema: public; Owner: ieuser
--

CREATE INDEX idx_va_provider_configs_active ON public.va_data_provider_configs USING btree (va_id, is_active);


--
-- Name: idx_va_provider_configs_config_type; Type: INDEX; Schema: public; Owner: ieuser
--

CREATE INDEX idx_va_provider_configs_config_type ON public.va_data_provider_configs USING btree (va_id, provider_type, config_type);


--
-- Name: idx_va_provider_configs_data; Type: INDEX; Schema: public; Owner: ieuser
--

CREATE INDEX idx_va_provider_configs_data ON public.va_data_provider_configs USING gin (config_data);


--
-- Name: idx_va_provider_configs_errors; Type: INDEX; Schema: public; Owner: ieuser
--

CREATE INDEX idx_va_provider_configs_errors ON public.va_data_provider_configs USING gin (validation_errors);


--
-- Name: idx_va_provider_configs_features; Type: INDEX; Schema: public; Owner: ieuser
--

CREATE INDEX idx_va_provider_configs_features ON public.va_data_provider_configs USING gin (features_enabled);


--
-- Name: idx_va_provider_configs_provider; Type: INDEX; Schema: public; Owner: ieuser
--

CREATE INDEX idx_va_provider_configs_provider ON public.va_data_provider_configs USING btree (provider_type);


--
-- Name: idx_va_provider_configs_va_provider_type; Type: INDEX; Schema: public; Owner: ieuser
--

CREATE INDEX idx_va_provider_configs_va_provider_type ON public.va_data_provider_configs USING btree (va_id, provider_type);


--
-- Name: idx_va_provider_validation_history_config_id; Type: INDEX; Schema: public; Owner: ieuser
--

CREATE INDEX idx_va_provider_validation_history_config_id ON public.va_provider_validation_history USING btree (config_id);


--
-- Name: idx_va_provider_validation_history_status; Type: INDEX; Schema: public; Owner: ieuser
--

CREATE INDEX idx_va_provider_validation_history_status ON public.va_provider_validation_history USING btree (validation_status);


--
-- Name: idx_va_user_roles_user_id; Type: INDEX; Schema: public; Owner: ieuser
--

CREATE INDEX idx_va_user_roles_user_id ON public.va_user_roles USING btree (user_id);


--
-- Name: idx_va_user_roles_user_va; Type: INDEX; Schema: public; Owner: ieuser
--

CREATE INDEX idx_va_user_roles_user_va ON public.va_user_roles USING btree (user_id, va_id);


--
-- Name: idx_va_user_roles_va_id; Type: INDEX; Schema: public; Owner: ieuser
--

CREATE INDEX idx_va_user_roles_va_id ON public.va_user_roles USING btree (va_id);


--
-- Name: idx_va_webhooks_type_active; Type: INDEX; Schema: public; Owner: ieuser
--

CREATE INDEX idx_va_webhooks_type_active ON public.va_webhooks USING btree (webhook_type, is_active);


--
-- Name: idx_va_webhooks_va_id; Type: INDEX; Schema: public; Owner: ieuser
--

CREATE INDEX idx_va_webhooks_va_id ON public.va_webhooks USING btree (va_id);


--
-- Name: event_legs trigger_event_legs_updated_at; Type: TRIGGER; Schema: public; Owner: ieuser
--

CREATE TRIGGER trigger_event_legs_updated_at BEFORE UPDATE ON public.event_legs FOR EACH ROW EXECUTE FUNCTION public.update_event_legs_timestamp();


--
-- Name: events trigger_events_updated_at; Type: TRIGGER; Schema: public; Owner: ieuser
--

CREATE TRIGGER trigger_events_updated_at BEFORE UPDATE ON public.events FOR EACH ROW EXECUTE FUNCTION public.update_events_timestamp();


--
-- Name: event_legs event_legs_created_by_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: ieuser
--

ALTER TABLE ONLY public.event_legs
    ADD CONSTRAINT event_legs_created_by_id_fkey FOREIGN KEY (created_by_id) REFERENCES public.users(id) ON DELETE SET NULL;


--
-- Name: event_legs event_legs_event_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: ieuser
--

ALTER TABLE ONLY public.event_legs
    ADD CONSTRAINT event_legs_event_id_fkey FOREIGN KEY (event_id) REFERENCES public.events(id) ON DELETE CASCADE;


--
-- Name: event_legs event_legs_updated_by_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: ieuser
--

ALTER TABLE ONLY public.event_legs
    ADD CONSTRAINT event_legs_updated_by_id_fkey FOREIGN KEY (updated_by_id) REFERENCES public.users(id) ON DELETE SET NULL;


--
-- Name: events events_created_by_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: ieuser
--

ALTER TABLE ONLY public.events
    ADD CONSTRAINT events_created_by_id_fkey FOREIGN KEY (created_by_id) REFERENCES public.users(id) ON DELETE SET NULL;


--
-- Name: events events_updated_by_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: ieuser
--

ALTER TABLE ONLY public.events
    ADD CONSTRAINT events_updated_by_id_fkey FOREIGN KEY (updated_by_id) REFERENCES public.users(id) ON DELETE SET NULL;


--
-- Name: events events_va_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: ieuser
--

ALTER TABLE ONLY public.events
    ADD CONSTRAINT events_va_id_fkey FOREIGN KEY (va_id) REFERENCES public.virtual_airlines(id) ON DELETE CASCADE;


--
-- Name: livery_airtable_mappings livery_airtable_mappings_va_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: ieuser
--

ALTER TABLE ONLY public.livery_airtable_mappings
    ADD CONSTRAINT livery_airtable_mappings_va_id_fkey FOREIGN KEY (va_id) REFERENCES public.virtual_airlines(id) ON DELETE CASCADE;


--
-- Name: va_configs va_configs_va_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: ieuser
--

ALTER TABLE ONLY public.va_configs
    ADD CONSTRAINT va_configs_va_id_fkey FOREIGN KEY (va_id) REFERENCES public.virtual_airlines(id);


--
-- Name: va_user_roles va_user_roles_user_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: ieuser
--

ALTER TABLE ONLY public.va_user_roles
    ADD CONSTRAINT va_user_roles_user_id_fkey FOREIGN KEY (user_id) REFERENCES public.users(id);


--
-- Name: va_user_roles va_user_roles_va_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: ieuser
--

ALTER TABLE ONLY public.va_user_roles
    ADD CONSTRAINT va_user_roles_va_id_fkey FOREIGN KEY (va_id) REFERENCES public.virtual_airlines(id);


--
-- Name: va_webhooks va_webhooks_va_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: ieuser
--

ALTER TABLE ONLY public.va_webhooks
    ADD CONSTRAINT va_webhooks_va_id_fkey FOREIGN KEY (va_id) REFERENCES public.virtual_airlines(id) ON DELETE CASCADE;


--
-- PostgreSQL database dump complete
--


