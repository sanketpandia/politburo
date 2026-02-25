--
-- Name: airports; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.airports (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    icao character varying(4) NOT NULL,
    iata character varying(3),
    name text NOT NULL,
    city character varying(100),
    country character varying(100),
    elevation integer,
    latitude numeric(10, 6) NOT NULL,
    longitude numeric(10, 6) NOT NULL,
    timezone character varying(50),
    created_at timestamp without time zone DEFAULT now(),
    updated_at timestamp without time zone DEFAULT now()
);

--
-- Name: airports airports_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.airports
    ADD CONSTRAINT airports_pkey PRIMARY KEY (id);

--
-- Name: airports airports_icao_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.airports
    ADD CONSTRAINT airports_icao_key UNIQUE (icao);

--
-- Name: idx_airports_icao; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_airports_icao ON public.airports USING btree (icao);

--
-- Name: idx_airports_iata; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_airports_iata ON public.airports USING btree (iata);

--
-- Name: idx_airports_coordinates; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_airports_coordinates ON public.airports USING btree (latitude, longitude);
