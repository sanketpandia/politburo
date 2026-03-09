-- Migration 014: Create va_webhooks table for per-VA Discord webhooks
-- Description: Store webhook URLs per VA and type (e.g. live_flights); job posts payload every 30 mins

CREATE TABLE public.va_webhooks (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    va_id uuid NOT NULL,
    webhook_type character varying(50) NOT NULL,
    webhook_url text NOT NULL,
    frequency_minutes integer DEFAULT 30 NOT NULL,
    is_active boolean DEFAULT true NOT NULL,
    label character varying(255),
    created_at timestamp without time zone DEFAULT now() NOT NULL,
    updated_at timestamp without time zone DEFAULT now() NOT NULL,
    CONSTRAINT va_webhooks_pkey PRIMARY KEY (id),
    CONSTRAINT va_webhooks_va_id_fkey FOREIGN KEY (va_id) REFERENCES public.virtual_airlines(id) ON DELETE CASCADE
);

CREATE INDEX idx_va_webhooks_va_id ON public.va_webhooks USING btree (va_id);
CREATE INDEX idx_va_webhooks_type_active ON public.va_webhooks USING btree (webhook_type, is_active);

COMMENT ON TABLE public.va_webhooks IS 'Per-VA webhooks (e.g. Discord) for notifications; first type: live_flights (30 min snapshot).';
