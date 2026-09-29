-- +goose Up

CREATE TABLE users (
    telegram_id BIGINT PRIMARY KEY,
    name        TEXT        NOT NULL DEFAULT '',
    username    TEXT,
    role        TEXT        NOT NULL DEFAULT 'customer'
                CHECK (role IN ('customer', 'staff', 'admin')),
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE restaurants (
    id               BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    name             TEXT        NOT NULL,
    address          TEXT        NOT NULL DEFAULT '',
    description      TEXT        NOT NULL DEFAULT '',
    -- IANA zone; slot dates from the API are interpreted in this zone.
    timezone         TEXT        NOT NULL DEFAULT 'UTC',
    -- How long a table is held per booking, used when generating slots.
    booking_minutes  INT         NOT NULL DEFAULT 120 CHECK (booking_minutes > 0),
    is_active        BOOLEAN     NOT NULL DEFAULT true,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Which users work at which restaurants.
CREATE TABLE restaurant_staff (
    restaurant_id BIGINT      NOT NULL REFERENCES restaurants (id) ON DELETE CASCADE,
    telegram_id   BIGINT      NOT NULL REFERENCES users (telegram_id) ON DELETE CASCADE,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (restaurant_id, telegram_id)
);
CREATE INDEX restaurant_staff_telegram_id_idx ON restaurant_staff (telegram_id);

-- Weekly template: "on weekday W, from opens_at to closes_at, start a slot
-- every slot_minutes with room for max_guests". Used to generate time_slots.
CREATE TABLE capacity_rules (
    id            BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    restaurant_id BIGINT   NOT NULL REFERENCES restaurants (id) ON DELETE CASCADE,
    weekday       SMALLINT NOT NULL CHECK (weekday BETWEEN 0 AND 6), -- 0 = Sunday
    opens_at      TIME     NOT NULL,
    closes_at     TIME     NOT NULL,
    slot_minutes  INT      NOT NULL DEFAULT 30 CHECK (slot_minutes > 0),
    max_guests    INT      NOT NULL CHECK (max_guests > 0),
    CHECK (closes_at > opens_at),
    UNIQUE (restaurant_id, weekday, opens_at)
);

-- Concrete bookable slots. booked_guests is maintained in the same
-- transaction that inserts/cancels a booking (after SELECT ... FOR UPDATE on
-- the slot row); the CHECK makes overbooking impossible at the DB level even
-- if application code has a bug.
CREATE TABLE time_slots (
    id            BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    restaurant_id BIGINT      NOT NULL REFERENCES restaurants (id) ON DELETE CASCADE,
    starts_at     TIMESTAMPTZ NOT NULL,
    capacity      INT         NOT NULL CHECK (capacity >= 0),
    booked_guests INT         NOT NULL DEFAULT 0 CHECK (booked_guests >= 0),
    CONSTRAINT time_slots_no_overbooking CHECK (booked_guests <= capacity),
    CONSTRAINT time_slots_restaurant_starts_at_key UNIQUE (restaurant_id, starts_at),
    -- Target for the composite FK from bookings.
    CONSTRAINT time_slots_id_restaurant_starts_at_key UNIQUE (id, restaurant_id, starts_at)
);

CREATE TABLE bookings (
    id              BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    restaurant_id   BIGINT      NOT NULL,
    slot_id         BIGINT      NOT NULL,
    user_id         BIGINT      NOT NULL REFERENCES users (telegram_id),
    status          TEXT        NOT NULL DEFAULT 'pending'
                    CHECK (status IN ('pending', 'confirmed', 'cancelled', 'completed', 'no_show')),
    guests          INT         NOT NULL CHECK (guests BETWEEN 1 AND 20),
    starts_at       TIMESTAMPTZ NOT NULL,
    special_request TEXT        NOT NULL DEFAULT '' CHECK (char_length(special_request) <= 500),
    reminder_sent   BOOLEAN     NOT NULL DEFAULT false,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    -- Keeps restaurant_id/starts_at consistent with the referenced slot.
    FOREIGN KEY (slot_id, restaurant_id, starts_at)
        REFERENCES time_slots (id, restaurant_id, starts_at)
);

-- A user can hold at most one active booking per slot.
CREATE UNIQUE INDEX bookings_one_active_per_user_slot
    ON bookings (user_id, slot_id)
    WHERE status IN ('pending', 'confirmed');

-- Staff "today" view and capacity sums.
CREATE INDEX bookings_restaurant_starts_at_idx ON bookings (restaurant_id, starts_at);
-- "My bookings".
CREATE INDEX bookings_user_starts_at_idx ON bookings (user_id, starts_at DESC);
-- Reminder worker scan.
CREATE INDEX bookings_reminder_due_idx
    ON bookings (starts_at)
    WHERE reminder_sent = false AND status = 'confirmed';

-- +goose Down

DROP TABLE bookings;
DROP TABLE time_slots;
DROP TABLE capacity_rules;
DROP TABLE restaurant_staff;
DROP TABLE restaurants;
DROP TABLE users;
