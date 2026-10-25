-- +goose Up

-- Needed for "table_id WITH =" / "user_id WITH =" in the exclusion constraints.
CREATE EXTENSION IF NOT EXISTS btree_gist;

-- An "owner" is not a role: a user owns venue X when venues.owner_id = telegram_id.
CREATE TABLE users (
    telegram_id BIGINT PRIMARY KEY,
    name        TEXT        NOT NULL DEFAULT '',
    username    TEXT,
    role        TEXT        NOT NULL DEFAULT 'customer'
                CHECK (role IN ('customer', 'admin')),
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE venues (
    id                BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    owner_id          BIGINT      NULL REFERENCES users (telegram_id),
    name              TEXT        NOT NULL,
    category          TEXT        NOT NULL
                      CHECK (category IN ('restaurant', 'cafe', 'bar', 'billiards', 'karaoke', 'other')),
    address           TEXT        NOT NULL DEFAULT '',
    description       TEXT        NOT NULL DEFAULT '',
    timezone          TEXT        NOT NULL DEFAULT 'Asia/Almaty',
    -- Venue-local wall-clock hours. 24:00 is allowed; overnight hours are not (MVP).
    opens_at          TIME        NOT NULL,
    closes_at         TIME        NOT NULL,
    booking_minutes   INT         NOT NULL DEFAULT 120 CHECK (booking_minutes > 0),
    slot_step_minutes INT         NOT NULL DEFAULT 30 CHECK (slot_step_minutes > 0),
    floor_plan_url    TEXT        NOT NULL DEFAULT '',
    is_active         BOOLEAN     NOT NULL DEFAULT true,
    created_at        TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT venues_hours_check CHECK (closes_at > opens_at)
);
CREATE INDEX venues_owner_id_idx ON venues (owner_id);

CREATE TABLE venue_tables (
    id        BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    venue_id  BIGINT       NOT NULL REFERENCES venues (id) ON DELETE CASCADE,
    label     TEXT         NOT NULL,
    seats     INT          NOT NULL CHECK (seats BETWEEN 1 AND 20),
    -- Table centre as a percent of the floor plan width / height.
    x         NUMERIC(5,2) NOT NULL CHECK (x BETWEEN 0 AND 100),
    y         NUMERIC(5,2) NOT NULL CHECK (y BETWEEN 0 AND 100),
    shape     TEXT         NOT NULL DEFAULT 'round' CHECK (shape IN ('round', 'rect')),
    is_active BOOLEAN      NOT NULL DEFAULT true,
    CONSTRAINT venue_tables_venue_label_key UNIQUE (venue_id, label),
    -- Target for the composite FK from bookings.
    CONSTRAINT venue_tables_id_venue_key UNIQUE (id, venue_id)
);

CREATE TABLE bookings (
    id              BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    venue_id        BIGINT      NOT NULL,
    table_id        BIGINT      NOT NULL,
    user_id         BIGINT      NOT NULL REFERENCES users (telegram_id),
    guests          INT         NOT NULL CHECK (guests BETWEEN 1 AND 20),
    during          TSTZRANGE   NOT NULL,
    status          TEXT        NOT NULL
                    CHECK (status IN ('pending', 'confirmed', 'rejected', 'cancelled', 'expired')),
    expires_at      TIMESTAMPTZ NULL,
    special_request TEXT        NOT NULL DEFAULT '' CHECK (char_length(special_request) <= 300),
    reminder_sent   BOOLEAN     NOT NULL DEFAULT false,
    decided_at      TIMESTAMPTZ NULL,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    -- The table must belong to the venue.
    CONSTRAINT bookings_table_venue_fkey FOREIGN KEY (table_id, venue_id)
        REFERENCES venue_tables (id, venue_id),
    CONSTRAINT bookings_during_check
        CHECK (NOT isempty(during) AND lower_inc(during) AND NOT upper_inc(during)),
    CONSTRAINT bookings_pending_expires_check
        CHECK (status <> 'pending' OR expires_at IS NOT NULL),
    CONSTRAINT bookings_no_table_overlap EXCLUDE USING gist (table_id WITH =, during WITH &&)
        WHERE (status IN ('pending', 'confirmed')),
    CONSTRAINT bookings_no_user_overlap EXCLUDE USING gist (user_id WITH =, during WITH &&)
        WHERE (status IN ('pending', 'confirmed'))
);

-- Owner day view.
CREATE INDEX bookings_venue_start_idx ON bookings (venue_id, lower(during));
-- "My bookings", newest first.
CREATE INDEX bookings_user_start_idx ON bookings (user_id, lower(during) DESC);
-- Expiry worker scan.
CREATE INDEX bookings_pending_expires_idx ON bookings (expires_at) WHERE status = 'pending';

-- +goose Down

DROP TABLE bookings;
DROP TABLE venue_tables;
DROP TABLE venues;
DROP TABLE users;
