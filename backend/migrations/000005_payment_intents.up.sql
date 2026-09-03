-- F3 Pagamento: payment_intents + payment_webhook_events.
--
-- payment_intents tracks the PSP-side intent (Stripe/PI/etc.). UNIQUE on
-- idempotency_key backs EC-2 (duplicate tenant call resolves to same row).
-- amount_cents / deposit_cents / expected_total_cents are the server's
-- authoritative numbers — EC-4 rejects the webhook if the PSP-reported
-- amount diverges from expected_total_cents.
--
-- payment_webhook_events is the audit + idempotency log of every webhook
-- received. UNIQUE(provider, provider_event_id) backs EC-2 against PSP
-- retries: a redelivery is a no-op.

CREATE TABLE payment_intents (
    id UUID PRIMARY KEY,
    rental_id UUID NOT NULL REFERENCES rentals (id) ON DELETE CASCADE,
    provider TEXT NOT NULL,
    provider_payment_id TEXT NOT NULL DEFAULT '',
    idempotency_key TEXT NOT NULL UNIQUE,
    status TEXT NOT NULL DEFAULT '',
    failure_code TEXT NOT NULL DEFAULT '',
    failure_message TEXT NOT NULL DEFAULT '',
    attempt INTEGER NOT NULL DEFAULT 1 CHECK (attempt >= 1),
    amount_cents BIGINT NOT NULL CHECK (amount_cents >= 0),
    deposit_cents BIGINT NOT NULL CHECK (deposit_cents >= 0),
    expected_total_cents BIGINT NOT NULL CHECK (expected_total_cents >= 0),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX payment_intents_rental_idx ON payment_intents (rental_id);
CREATE INDEX payment_intents_provider_idx ON payment_intents (provider, provider_payment_id);

CREATE TABLE payment_webhook_events (
    id UUID PRIMARY KEY,
    provider TEXT NOT NULL,
    provider_event_id TEXT NOT NULL,
    event_type TEXT NOT NULL DEFAULT '',
    rental_id UUID REFERENCES rentals (id) ON DELETE SET NULL,
    payment_intent_id UUID REFERENCES payment_intents (id) ON DELETE SET NULL,
    payload BYTEA NOT NULL,
    signature_valid BOOLEAN NOT NULL DEFAULT FALSE,
    received_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    processed_at TIMESTAMPTZ,
    UNIQUE (provider, provider_event_id)
);

CREATE INDEX payment_webhook_events_rental_idx ON payment_webhook_events (rental_id);
