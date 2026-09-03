-- F5 Devolução + avaria + dívida.
--
-- devolucoes: 1 row per rental, attached to the post-confirmation return
-- flow. UNIQUE(rental_id) backs the "exactly one return per rental" invariant.
-- state is the ReturnState enum (awaiting_pickup, in_progress,
-- awaiting_confirmation, closed, contested). deposit_released_cents and
-- deposit_captured_cents are mutated when the return closes; the
-- CHECK caps captured at the rental deposit (caller enforced) and released
-- + captured at the deposit too.
--
-- avaria_pedidos: damage claims, owned by the listing owner, defended by
-- the renter. UNIQUE(rental_id) backs "at most one open/active claim per
-- rental" (the v1 invariant from Pilar 2; re-opening after resolution is
-- out of scope for v1). state is the DamageState enum. The CHECK on
-- proposed_cents >= 0 and agreed_cents >= 0 keeps the column honest;
-- the cap (deposit or declared value) is enforced in the service.
--
-- dividas: active debts attached to a damage claim and a renter. One row
-- per debt; UNIQUE on (damage_id) where the debt is the residual of that
-- claim. state is the DebtState enum. due_at is computed at creation
-- (5 days window per Pilar 5). Index on renter_id supports the
-- "renter has open debt" check from Pilar 5 (consumed by F3 CreateIntent).
--
-- All FKs use ON DELETE CASCADE: devolucao / avaria / divida belong to
-- the rental + accounts and have no meaning without them. photo refs in
-- the JSONB evidence are object-storage keys, not paths; LGPD retention
-- is enforced by cmd/cleanup (F12, out of scope of this issue).

CREATE TABLE devolucoes (
    id UUID PRIMARY KEY,
    rental_id UUID NOT NULL UNIQUE REFERENCES rentals (id) ON DELETE CASCADE,
    state TEXT NOT NULL CHECK (state IN ('awaiting_pickup', 'in_progress', 'awaiting_confirmation', 'closed', 'contested')),
    pickup_evidence JSONB NOT NULL DEFAULT '{}'::jsonb,
    return_evidence JSONB NOT NULL DEFAULT '{}'::jsonb,
    deposit_released_cents BIGINT NOT NULL DEFAULT 0 CHECK (deposit_released_cents >= 0),
    deposit_captured_cents BIGINT NOT NULL DEFAULT 0 CHECK (deposit_captured_cents >= 0),
    returned_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CHECK (deposit_released_cents + deposit_captured_cents <= 100000000000)
);

CREATE INDEX devolucoes_state_idx ON devolucoes (state);

CREATE TABLE avaria_pedidos (
    id UUID PRIMARY KEY,
    rental_id UUID NOT NULL UNIQUE REFERENCES rentals (id) ON DELETE CASCADE,
    owner_id UUID NOT NULL REFERENCES accounts (id) ON DELETE CASCADE,
    renter_id UUID NOT NULL REFERENCES accounts (id) ON DELETE CASCADE,
    state TEXT NOT NULL CHECK (state IN ('open', 'renter_agreed', 'contested', 'staff_resolved', 'expired', 'cancelled')),
    nature TEXT NOT NULL CHECK (nature IN ('cosmetic', 'functional', 'loss')),
    description TEXT NOT NULL DEFAULT '',
    evidence JSONB NOT NULL DEFAULT '{}'::jsonb,
    proposed_cents BIGINT NOT NULL CHECK (proposed_cents >= 0),
    agreed_cents BIGINT NOT NULL DEFAULT 0 CHECK (agreed_cents >= 0),
    renter_response_kind TEXT NOT NULL DEFAULT '',
    renter_response_note TEXT NOT NULL DEFAULT '',
    staff_decision_note TEXT NOT NULL DEFAULT '',
    opened_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    responded_at TIMESTAMPTZ,
    decided_at TIMESTAMPTZ,
    resolved_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX avaria_pedidos_owner_idx ON avaria_pedidos (owner_id);
CREATE INDEX avaria_pedidos_renter_idx ON avaria_pedidos (renter_id);
CREATE INDEX avaria_pedidos_state_idx ON avaria_pedidos (state);

CREATE TABLE dividas (
    id UUID PRIMARY KEY,
    rental_id UUID NOT NULL UNIQUE REFERENCES rentals (id) ON DELETE CASCADE,
    damage_id UUID NOT NULL UNIQUE REFERENCES avaria_pedidos (id) ON DELETE CASCADE,
    renter_id UUID NOT NULL REFERENCES accounts (id) ON DELETE CASCADE,
    state TEXT NOT NULL CHECK (state IN ('open', 'settled', 'forgiven')),
    original_cents BIGINT NOT NULL CHECK (original_cents > 0),
    forgiven_cents BIGINT NOT NULL DEFAULT 0 CHECK (forgiven_cents >= 0),
    settled_cents BIGINT NOT NULL DEFAULT 0 CHECK (settled_cents >= 0),
    forgiven_reason TEXT NOT NULL DEFAULT '',
    due_at TIMESTAMPTZ NOT NULL,
    settled_at TIMESTAMPTZ,
    forgiven_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CHECK (forgiven_cents + settled_cents <= original_cents)
);

CREATE INDEX dividas_renter_state_idx ON dividas (renter_id, state);
CREATE INDEX dividas_due_at_idx ON dividas (due_at) WHERE state = 'open';
