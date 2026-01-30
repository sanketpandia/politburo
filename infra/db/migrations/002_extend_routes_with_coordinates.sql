--
-- Add geographic coordinates to route_at_synced table
--

ALTER TABLE public.route_at_synced
    ADD COLUMN origin_lat numeric(10, 6),
    ADD COLUMN origin_lon numeric(10, 6),
    ADD COLUMN destination_lat numeric(10, 6),
    ADD COLUMN destination_lon numeric(10, 6);

--
-- Name: idx_route_at_synced_coordinates; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_route_at_synced_coordinates ON public.route_at_synced USING btree (origin_lat, origin_lon, destination_lat, destination_lon);
