-- F4 Cancelamento + liquidação:
--   * `rental_cancellations` — one immutable record per rental (UNIQUE rental_id).
--     Captures the F4 declarative window (AC-1..AC-8), the split per
--     part (rent/operator/deposit), and the PSP-side operation id when
--     the trigger is a webhook (refund/chargeback). Tenant/owner get
--     the same row; corrections append a new row, never UPDATE.
--   * `rental_receipts` extended with F4 fields (AC-11): actor_kind,
--     window_applied, cancellation_fee, refund/payouts after
--     cancellation, deposit state + capture/release amounts, PSP op
--     id, cancellation_issued_at.
--   * `accounts.conta_bloqueada_por_chargeback` — F4 EC-5: tenant
--     account is blocked from new reservations until the chargeback
--     is regularised. Independent from the account.deactivated flag.
--
-- The state machine on rentals adds `cancellation_in_progress`
-- (terminal-only after the cancellation row is committed) but the
-- CHECK constraint on rentals.state is widened in this same migration
-- so the new state is valid for both the optimistic-lock and the
-- EC-1/EC-6 anti-double-penalty check.

ALTER TABLE rentals
    DROP CONSTRAINT rentals_state_check;
ALTER TABLE rentals
    ADD CONSTRAINT rentals_state_check
    CHECK (state IN (
        'pending',
        'authorized',
        'confirmed',
        'declined',
        'expired',
        'cancelled',
        'cancellation_in_progress',
        'refunded'
    ));

ALTER TABLE accounts
    ADD COLUMN conta_bloqueada_por_chargeback BOOLEAN NOT NULL DEFAULT FALSE;

ALTER TABLE rental_receipts
    ADD COLUMN actor_kind TEXT NOT NULL DEFAULT 'tenant',
    ADD COLUMN window_applied TEXT NOT NULL DEFAULT '',
    ADD COLUMN cancellation_fee_cents BIGINT NOT NULL DEFAULT 0,
    ADD COLUMN tenant_refund_cents BIGINT NOT NULL DEFAULT 0,
    ADD COLUMN owner_payout_cents_after_cancellation BIGINT NOT NULL DEFAULT 0,
    ADD COLUMN operator_payout_cents_after_cancellation BIGINT NOT NULL DEFAULT 0,
    ADD COLUMN deposit_state TEXT NOT NULL DEFAULT 'released'
        CHECK (deposit_state IN ('released', 'captured', 'partial', 'held')),
    ADD COLUMN deposit_capture_cents BIGINT NOT NULL DEFAULT 0,
    ADD COLUMN deposit_release_cents BIGINT NOT NULL DEFAULT 0,
    ADD COLUMN deposit_partial_capture_cents BIGINT NOT NULL DEFAULT 0,
    ADD COLUMN processor_operation_id TEXT NOT NULL DEFAULT '',
    ADD COLUMN cancellation_issued_at TIMESTAMPTZ,
    ADD CONSTRAINT rental_receipts_window_ends_at_after_starts_check_v4
        CHECK (window_ends_at > window_starts_at);

CREATE TABLE rental_cancellations (
    id UUID PRIMARY KEY,
    rental_id UUID NOT NULL UNIQUE REFERENCES rentals (id) ON DELETE CASCADE,
    actor_account_id UUID NOT NULL REFERENCES accounts (id) ON DELETE CASCADE,
    actor_kind TEXT NOT NULL CHECK (actor_kind IN ('tenant', 'owner', 'platform', 'operator')),
    window_applied TEXT NOT NULL,
    state_deposit TEXT NOT NULL CHECK (state_deposit IN ('released', 'captured', 'partial', 'held')),
    cancellation_fee_cents BIGINT NOT NULL DEFAULT 0 CHECK (cancellation_fee_cents >= 0),
    tenant_refund_cents BIGINT NOT NULL,
    owner_payout_cents_after_cancellation BIGINT NOT NULL,
    operator_payout_cents_after_cancellation BIGINT NOT NULL,
    deposit_capture_cents BIGINT NOT NULL DEFAULT 0 CHECK (deposit_capture_cents >= 0),
    deposit_release_cents BIGINT NOT NULL DEFAULT 0 CHECK (deposit_release_cents >= 0),
    deposit_partial_capture_cents BIGINT NOT NULL DEFAULT 0 CHECK (deposit_partial_capture_cents >= 0),
    processor_operation_id TEXT NOT NULL DEFAULT '',
    reversal_reason TEXT NOT NULL DEFAULT '',
    issued_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX rental_cancellations_rental_idx ON rental_cancellations (rental_id);
CREATE INDEX rental_cancellations_actor_idx ON rental_cancellations (actor_account_id);
CREATE INDEX rental_cancellations_processor_idx
    ON rental_cancellations (processor_operation_id)
    WHERE processor_operation_id <> '';

CREATE INDEX accounts_chargeback_blocked_idx
    ON accounts (conta_bloqueada_por_chargeback)
    WHERE conta_bloqueada_por_chargeback = TRUE;
