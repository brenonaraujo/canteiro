-- F4 rollback.

DROP INDEX IF EXISTS rental_cancellations_processor_idx;
DROP INDEX IF EXISTS rental_cancellations_actor_idx;
DROP INDEX IF EXISTS rental_cancellations_rental_idx;
DROP INDEX IF EXISTS accounts_chargeback_blocked_idx;
DROP TABLE IF EXISTS rental_cancellations;

ALTER TABLE rental_receipts
    DROP CONSTRAINT IF EXISTS rental_receipts_window_ends_at_after_starts_check_v4,
    DROP COLUMN IF EXISTS cancellation_issued_at,
    DROP COLUMN IF EXISTS processor_operation_id,
    DROP COLUMN IF EXISTS deposit_partial_capture_cents,
    DROP COLUMN IF EXISTS deposit_release_cents,
    DROP COLUMN IF EXISTS deposit_capture_cents,
    DROP COLUMN IF EXISTS deposit_state,
    DROP COLUMN IF EXISTS operator_payout_cents_after_cancellation,
    DROP COLUMN IF EXISTS owner_payout_cents_after_cancellation,
    DROP COLUMN IF EXISTS tenant_refund_cents,
    DROP COLUMN IF EXISTS cancellation_fee_cents,
    DROP COLUMN IF EXISTS window_applied,
    DROP COLUMN IF EXISTS actor_kind;

ALTER TABLE accounts
    DROP COLUMN IF EXISTS conta_bloqueada_por_chargeback;

ALTER TABLE rentals
    DROP CONSTRAINT IF EXISTS rentals_state_check;
ALTER TABLE rentals
    ADD CONSTRAINT rentals_state_check
    CHECK (state IN ('pending', 'authorized', 'confirmed', 'declined', 'expired', 'cancelled', 'refunded'));
