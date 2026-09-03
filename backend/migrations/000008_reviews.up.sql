-- F6 Avaliação.
--
-- reviews: one row per (rental, rater_user_id, scope). The scope is
-- part of the unique key so the renter can rate listing + owner +
-- operator (when distinct) on the same rental without collision.
-- score is integer 1..5 with a CHECK; the service also enforces
-- server-side. comment is free text up to 4 KiB (the service
-- enforces; the column is TEXT to be lenient on UTF-8). ratee_user_id
-- is NULL when scope='listing' (the rating is of the asset, not of a
-- user); for the other three scopes it points at the receiving
-- account.
--
-- Indexes:
--   - UNIQUE(rental_id, rater_user_id, scope): one rating per
--     (rental, rater, scope). Drives ErrAlreadyReviewed.
--   - ratee_user_id: drives the listing API for a user's profile.
--   - rental_id: drives the listing API for a listing's reviews.
--
-- review_aggregates: materialized count+sum+avg per (ratee_user_id,
-- scope). One row per (ratee, scope) that has at least one review;
-- the API returns a zero aggregate for misses. The trigger keeps the
-- aggregate in sync with reviews INSERTs (the only writer in v1;
-- EDIT/MODERATION/DELETE arrive in F12 — those will recompute via
-- cmd/recompute-aggregates per DoD Decisão 1).
--
-- All FKs use ON DELETE CASCADE: reviews belong to the rental and
-- belong to the rater account; aggregates belong to the ratee
-- account. Removing the rental cleans up reviews (and aggregates get
-- recomputed by the F12 cleanup job).

CREATE TABLE reviews (
    id UUID PRIMARY KEY,
    rental_id UUID NOT NULL REFERENCES rentals (id) ON DELETE CASCADE,
    rater_user_id UUID NOT NULL REFERENCES accounts (id) ON DELETE CASCADE,
    ratee_user_id UUID REFERENCES accounts (id) ON DELETE CASCADE,
    scope TEXT NOT NULL CHECK (scope IN ('listing', 'owner', 'operator', 'renter')),
    score INTEGER NOT NULL CHECK (score BETWEEN 1 AND 5),
    comment TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT reviews_unique_per_rater_scope UNIQUE (rental_id, rater_user_id, scope),
    CONSTRAINT reviews_ratee_required_for_user_scopes CHECK (
        (scope = 'listing' AND ratee_user_id IS NULL)
        OR (scope IN ('owner', 'operator', 'renter') AND ratee_user_id IS NOT NULL)
    )
);

CREATE INDEX reviews_ratee_scope_idx ON reviews (ratee_user_id, scope, created_at DESC);
CREATE INDEX reviews_rental_idx ON reviews (rental_id);

CREATE TABLE review_aggregates (
    ratee_user_id UUID NOT NULL,
    scope TEXT NOT NULL CHECK (scope IN ('listing', 'owner', 'operator', 'renter')),
    count BIGINT NOT NULL DEFAULT 0 CHECK (count >= 0),
    sum BIGINT NOT NULL DEFAULT 0 CHECK (sum >= 0),
    avg NUMERIC(4,2) NOT NULL DEFAULT 0.00,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (ratee_user_id, scope)
);

-- Keep review_aggregates consistent with reviews inserts. Application
-- code does the upsert inside the same transaction, so this trigger
-- is a belt-and-braces backstop: if a future writer forgets the
-- upsert, the aggregate still moves. Inserts only — F12 edits /
-- moderation / cleanup run cmd/recompute-aggregates (DoD Decisão 1)
-- to rebuild the row from scratch when needed.
CREATE OR REPLACE FUNCTION review_aggregates_sync() RETURNS TRIGGER AS $$
DECLARE
    ratee UUID;
BEGIN
    ratee := NEW.ratee_user_id;
    IF ratee IS NULL THEN
        RETURN NEW;
    END IF;
    INSERT INTO review_aggregates (ratee_user_id, scope, count, sum, avg, updated_at)
    VALUES (ratee, NEW.scope, 1, NEW.score, NEW.score, now())
    ON CONFLICT (ratee_user_id, scope) DO UPDATE
    SET count = review_aggregates.count + 1,
        sum = review_aggregates.sum + NEW.score,
        avg = ROUND((review_aggregates.sum + NEW.score)::NUMERIC
                    / (review_aggregates.count + 1), 2),
        updated_at = now();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER reviews_after_insert_aggregate_sync
    AFTER INSERT ON reviews
    FOR EACH ROW
    EXECUTE FUNCTION review_aggregates_sync();
