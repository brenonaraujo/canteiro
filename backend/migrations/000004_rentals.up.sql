-- F3 Reserva: rentals + rental_receipts.
--
-- rentals is the lifecycle row of a reservation. The listing_snapshot
-- JSONB stores the immutable commercial snapshot taken at intent creation
-- (AC-10) so later listing edits never propagate to in-flight rentals.
-- The intent_key UNIQUE is the idempotency surface for duplicate tenant
-- calls (CreateIntent uses ON CONFLICT DO NOTHING).
--
-- rental_receipts is the tenant-visible write-once projection persisted
-- on payment.authorized (AC-7). UNIQUE(rental_id) enforces EC-2
-- idempotency against double-authorize replays.
--
-- All FKs to listings/accounts use ON DELETE CASCADE: rentals belong to
-- the listing+tenant pair and have no meaning without either.

CREATE TABLE rentals (
    id UUID PRIMARY KEY,
    listing_id UUID NOT NULL REFERENCES listings (id) ON DELETE CASCADE,
    tenant_account_id UUID NOT NULL REFERENCES accounts (id) ON DELETE CASCADE,
    state TEXT NOT NULL CHECK (state IN ('pending', 'authorized', 'confirmed', 'declined', 'expired', 'cancelled', 'refunded')),
    decline_reason TEXT NOT NULL DEFAULT '',
    intent_key TEXT NOT NULL,
    tenant_claim_debt TEXT NOT NULL DEFAULT 'none',
    listing_snapshot JSONB NOT NULL,
    rent_cents BIGINT NOT NULL CHECK (rent_cents >= 0),
    operator_cents BIGINT NOT NULL CHECK (operator_cents >= 0),
    deposit_cents BIGINT NOT NULL CHECK (deposit_cents >= 0),
    commission_cents BIGINT NOT NULL CHECK (commission_cents >= 0),
    owner_payout_cents BIGINT NOT NULL CHECK (owner_payout_cents >= 0),
    operator_payout_cents BIGINT NOT NULL CHECK (operator_payout_cents >= 0),
    starts_at TIMESTAMPTZ NOT NULL,
    ends_at TIMESTAMPTZ NOT NULL,
    acceptance_deadline_at TIMESTAMPTZ,
    confirmed_at TIMESTAMPTZ,
    declined_at TIMESTAMPTZ,
    with_operator BOOLEAN NOT NULL DEFAULT FALSE,
    operator_terms_accepted BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CHECK (ends_at > starts_at),
    UNIQUE (tenant_account_id, listing_id, intent_key)
);

CREATE INDEX rentals_tenant_idx ON rentals (tenant_account_id);
CREATE INDEX rentals_listing_idx ON rentals (listing_id);
CREATE INDEX rentals_state_idx ON rentals (state);
CREATE INDEX rentals_listing_state_window_idx ON rentals (listing_id, starts_at, ends_at) WHERE state IN ('authorized', 'confirmed');
CREATE INDEX rentals_owner_snapshot_idx ON rentals ((listing_snapshot->>'owner_id'));

CREATE TABLE rental_receipts (
    id UUID PRIMARY KEY,
    rental_id UUID NOT NULL UNIQUE REFERENCES rentals (id) ON DELETE CASCADE,
    tenant_account_id UUID NOT NULL REFERENCES accounts (id) ON DELETE CASCADE,
    listing_snapshot JSONB NOT NULL,
    rent_cents BIGINT NOT NULL CHECK (rent_cents >= 0),
    operator_cents BIGINT NOT NULL CHECK (operator_cents >= 0),
    deposit_cents BIGINT NOT NULL CHECK (deposit_cents >= 0),
    total_cents BIGINT NOT NULL CHECK (total_cents >= 0),
    commission_base_cents BIGINT NOT NULL CHECK (commission_base_cents >= 0),
    commission_cents BIGINT NOT NULL CHECK (commission_cents >= 0),
    owner_payout_cents BIGINT NOT NULL CHECK (owner_payout_cents >= 0),
    operator_payout_cents BIGINT NOT NULL CHECK (operator_payout_cents >= 0),
    window_starts_at TIMESTAMPTZ NOT NULL,
    window_ends_at TIMESTAMPTZ NOT NULL,
    issued_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CHECK (window_ends_at > window_starts_at)
);

CREATE INDEX rental_receipts_tenant_idx ON rental_receipts (tenant_account_id);
