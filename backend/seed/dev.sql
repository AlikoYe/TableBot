-- Development seed data. Safe to run repeatedly.
--   make seed                 -- owner is telegram_id 1
--   make seed OWNER=123456789 -- your own Telegram id owns every seeded venue
-- Dev user 2 is a plain customer.

\if :{?owner_id}
\else
\set owner_id 1
\endif

BEGIN;

INSERT INTO users (telegram_id, name, username)
VALUES (:'owner_id'::bigint, 'Venue Owner', NULL),
       (2, 'Dev Customer', 'dev_2')
ON CONFLICT (telegram_id) DO NOTHING;

INSERT INTO venues (id, owner_id, name, category, address, description, timezone,
                    opens_at, closes_at, booking_minutes, slot_step_minutes, floor_plan_url)
OVERRIDING SYSTEM VALUE
VALUES
    (1, :'owner_id'::bigint, 'Nomad Bistro', 'restaurant', 'Abay Ave 52, Almaty',
     'Modern Kazakh and Central Asian cuisine: beshbarmak, lagman and steppe lamb, with big windows onto Abay Avenue.',
     'Asia/Almaty', '12:00', '24:00', 120, 30, '/api/static/plans/nomad-bistro.svg'),
    (2, :'owner_id'::bigint, 'Pyramid Billiards Club', 'billiards', 'Rozybakiev St 247, Almaty',
     'Six Russian pyramid and pool tables, a cue rack for guests and a quiet bar. Tables are booked by the hour.',
     'Asia/Almaty', '14:00', '24:00', 60, 30, '/api/static/plans/pyramid-billiards.svg'),
    (3, :'owner_id'::bigint, 'Medeu Coffee', 'cafe', 'Kabanbay Batyr St 112, Almaty',
     'Specialty coffee, fresh pastries and a big window bar. Good for breakfast meetings and laptops.',
     'Asia/Almaty', '08:00', '22:00', 90, 30, '/api/static/plans/medeu-coffee.svg')
ON CONFLICT (id) DO UPDATE SET
    owner_id = EXCLUDED.owner_id, name = EXCLUDED.name, category = EXCLUDED.category,
    address = EXCLUDED.address, description = EXCLUDED.description, timezone = EXCLUDED.timezone,
    opens_at = EXCLUDED.opens_at, closes_at = EXCLUDED.closes_at,
    booking_minutes = EXCLUDED.booking_minutes, slot_step_minutes = EXCLUDED.slot_step_minutes,
    floor_plan_url = EXCLUDED.floor_plan_url, is_active = true;

SELECT setval(pg_get_serial_sequence('venues', 'id'), GREATEST((SELECT max(id) FROM venues), 1));

-- x / y are the table centre in percent of the plan width / height.
INSERT INTO venue_tables (venue_id, label, seats, x, y, shape)
VALUES
    -- Nomad Bistro: window row, middle, family tables near the bar.
    (1, 'T1', 2, 13, 18, 'round'),
    (1, 'T2', 2, 13, 42, 'round'),
    (1, 'T3', 4, 32, 20, 'round'),
    (1, 'T4', 4, 50, 20, 'round'),
    (1, 'T5', 4, 67, 20, 'round'),
    (1, 'T6', 6, 32, 52, 'rect'),
    (1, 'T7', 6, 54, 52, 'rect'),
    (1, 'T8', 8, 68, 76, 'rect'),
    -- Pyramid Billiards Club: two rows of three tables.
    (2, 'T1', 2, 32, 30, 'rect'),
    (2, 'T2', 2, 55, 30, 'rect'),
    (2, 'T3', 4, 78, 30, 'rect'),
    (2, 'T4', 4, 32, 64, 'rect'),
    (2, 'T5', 4, 55, 64, 'rect'),
    (2, 'T6', 6, 78, 64, 'rect'),
    -- Medeu Coffee: window tables, middle, big table by the sofa.
    (3, 'T1', 2, 12, 32, 'round'),
    (3, 'T2', 2, 12, 58, 'round'),
    (3, 'T3', 2, 30, 80, 'round'),
    (3, 'T4', 4, 36, 42, 'round'),
    (3, 'T5', 4, 60, 48, 'rect'),
    (3, 'T6', 6, 82, 62, 'rect')
ON CONFLICT (venue_id, label) DO UPDATE SET
    seats = EXCLUDED.seats, x = EXCLUDED.x, y = EXCLUDED.y,
    shape = EXCLUDED.shape, is_active = true;

COMMIT;
