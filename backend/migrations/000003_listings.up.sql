-- F2 Catálogo: listings, photos, calendar blocks, owner onboarding,
-- category config (deposit minimum per category).
--
-- Listings storage follows the F2 spec §4.2 (publication fields) plus the
-- F2 §4.9 operator mode declaration and F2 §4.3 minimização de identidade
-- (a ficha pública omite nome do dono e do operador; aqui mantemos o
-- privacy do dono do lado do caller; a derivação é feita em runtime).
--
-- Data shape chosen to be neutral across v1: prices and deposits in cents,
-- operator hourly rate in cents, lead time in hours, blocks as timestamps
-- with tz. Photo URLs stored as opaque strings; v1 may resolve them via a
-- object store layer later without changing this schema.

CREATE TABLE listing_categories (
    category TEXT PRIMARY KEY,
    deposit_min_cents BIGINT NOT NULL CHECK (deposit_min_cents >= 0),
    size TEXT NOT NULL CHECK (size IN ('light', 'heavy')),
    display_order SMALLINT NOT NULL DEFAULT 0
);

INSERT INTO listing_categories (category, deposit_min_cents, size, display_order) VALUES
    ('manual',             5000,    'light', 1),
    ('electric',           8000,    'light', 2),
    ('light_construction', 15000,   'light', 3),
    ('agricultural',       20000,   'light', 4),
    ('heavy',              80000,   'heavy', 5);

CREATE TABLE listings (
    id UUID PRIMARY KEY,
    owner_account_id UUID NOT NULL REFERENCES accounts (id) ON DELETE CASCADE,
    state TEXT NOT NULL CHECK (state IN ('draft', 'published', 'paused')),
    title TEXT NOT NULL,
    description TEXT NOT NULL,
    category TEXT NOT NULL REFERENCES listing_categories (category),
    pickup_city TEXT NOT NULL,
    pickup_neighborhood TEXT NOT NULL,
    delivery_enabled BOOLEAN NOT NULL DEFAULT FALSE,
    delivery_coverage TEXT NOT NULL DEFAULT '',
    price_unit TEXT NOT NULL CHECK (price_unit IN ('hour', 'day')),
    price_amount_cents BIGINT NOT NULL CHECK (price_amount_cents >= 0),
    deposit_cents BIGINT NOT NULL CHECK (deposit_cents >= 0),
    min_lead_time_hours INTEGER NOT NULL DEFAULT 0 CHECK (min_lead_time_hours >= 0),
    operator_mode TEXT NOT NULL CHECK (operator_mode IN ('none', 'optional', 'required')),
    operator_hourly_rate_cents BIGINT NOT NULL DEFAULT 0 CHECK (operator_hourly_rate_cents >= 0),
    operator_min_hours INTEGER NOT NULL DEFAULT 0 CHECK (operator_min_hours >= 0),
    operator_name TEXT NOT NULL DEFAULT '',
    operator_phone TEXT NOT NULL DEFAULT '',
    operator_is_owner BOOLEAN NOT NULL DEFAULT FALSE,
    rule_document_required BOOLEAN NOT NULL DEFAULT FALSE,
    rule_min_age INTEGER NOT NULL DEFAULT 0 CHECK (rule_min_age >= 0 AND rule_min_age <= 130),
    rule_experience_required BOOLEAN NOT NULL DEFAULT FALSE,
    rule_travel_restricted BOOLEAN NOT NULL DEFAULT FALSE,
    heavy_legal_cession BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX listings_owner_idx ON listings (owner_account_id);
CREATE INDEX listings_state_category_idx ON listings (state, category);
CREATE INDEX listings_state_city_idx ON listings (state, pickup_city);

CREATE TABLE listing_photos (
    listing_id UUID NOT NULL REFERENCES listings (id) ON DELETE CASCADE,
    position SMALLINT NOT NULL CHECK (position >= 0),
    url TEXT NOT NULL,
    PRIMARY KEY (listing_id, position)
);

CREATE TABLE listing_blocks (
    id UUID PRIMARY KEY,
    listing_id UUID NOT NULL REFERENCES listings (id) ON DELETE CASCADE,
    starts_at TIMESTAMPTZ NOT NULL,
    ends_at TIMESTAMPTZ NOT NULL,
    reason TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CHECK (ends_at > starts_at)
);

CREATE INDEX listing_blocks_listing_idx ON listing_blocks (listing_id);
CREATE INDEX listing_blocks_window_idx ON listing_blocks (listing_id, starts_at, ends_at);

CREATE TABLE owner_onboarding (
    account_id UUID PRIMARY KEY REFERENCES accounts (id) ON DELETE CASCADE,
    payout_kind TEXT NOT NULL DEFAULT '',
    payout_last4 TEXT NOT NULL DEFAULT '',
    terms_accepted_at TIMESTAMPTZ,
    terms_version TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE audit_listing_log (
    id BIGSERIAL PRIMARY KEY,
    listing_id UUID NOT NULL REFERENCES listings (id) ON DELETE CASCADE,
    actor_account_id UUID REFERENCES accounts (id) ON DELETE SET NULL,
    action TEXT NOT NULL,
    payload JSONB NOT NULL DEFAULT '{}'::JSONB,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX audit_listing_log_listing_idx ON audit_listing_log (listing_id, created_at DESC);
