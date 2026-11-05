-- Demo data: 10 Almaty venues with tables. Add-only and safe to re-run: rows that already
-- exist are left alone, so tables edited in the app and changed owners are never overwritten.
--   make seed                  -- new venues belong to telegram_id 1
--   make seed OWNER=123456789  -- new venues belong to that user
-- Change the owner of one venue: make set-owner VENUE=<id> OWNER=<telegram_id>
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
     'Asia/Almaty', '08:00', '22:00', 90, 30, '/api/static/plans/medeu-coffee.svg'),
    (4, :'owner_id'::bigint, 'Steppe Grill', 'restaurant', 'Dostyk Ave 136, Almaty',
     'Open-fire grill with shashlik, kazy and steppe lamb; big windows onto Dostyk Avenue.',
     'Asia/Almaty', '12:00', '24:00', 120, 30, '/api/static/plans/steppe-grill.svg'),
    (5, :'owner_id'::bigint, 'Skyline Bar', 'bar', 'Furmanov St 240, Almaty',
     'Cocktail bar with a city view, DJ sets on weekends and a lounge sofa corner.',
     'Asia/Almaty', '18:00', '24:00', 120, 30, '/api/static/plans/skyline-bar.svg'),
    (6, :'owner_id'::bigint, 'Voice Box Karaoke', 'karaoke', 'Satpayev St 30, Almaty',
     'Four private karaoke rooms and a small hall with a stage. Rooms are booked for two hours.',
     'Asia/Almaty', '16:00', '24:00', 120, 60, '/api/static/plans/voice-karaoke.svg'),
    (7, :'owner_id'::bigint, 'Dastarkhan Hall', 'restaurant', 'Raiymbek Ave 160, Almaty',
     'Banquet hall for family celebrations: large round tables, a stage and a dance floor.',
     'Asia/Almaty', '11:00', '23:00', 180, 60, '/api/static/plans/dastarkhan-hall.svg'),
    (8, :'owner_id'::bigint, 'Shanyrak Tea House', 'cafe', 'Zheltoksan St 59, Almaty',
     'Traditional tea house with a samovar, baursaks and low topchan platforms by the garden windows.',
     'Asia/Almaty', '10:00', '22:00', 90, 30, '/api/static/plans/shanyrak-teahouse.svg'),
    (9, :'owner_id'::bigint, 'Arbat Books & Coffee', 'cafe', 'Zhibek Zholy St 50, Almaty',
     'Quiet cafe with a book corner, filter coffee and desserts. Good for reading and work.',
     'Asia/Almaty', '08:00', '21:00', 90, 30, '/api/static/plans/medeu-coffee.svg'),
    (10, :'owner_id'::bigint, 'Cue Club Almaty', 'billiards', 'Tole Bi St 101, Almaty',
     'Pool and Russian pyramid tables with a bar and a lounge. Tables are booked by the hour.',
     'Asia/Almaty', '12:00', '24:00', 60, 30, '/api/static/plans/pyramid-billiards.svg')
ON CONFLICT (id) DO NOTHING;

SELECT setval(pg_get_serial_sequence('venues', 'id'), GREATEST((SELECT max(id) FROM venues), 1));

-- x / y are the table centre in percent of the plan width / height.
INSERT INTO venue_tables (venue_id, label, seats, x, y, shape)
VALUES
    -- Nomad Bistro
    (1, 'T1', 2, 13, 18, 'round'),
    (1, 'T2', 2, 13, 42, 'round'),
    (1, 'T3', 4, 32, 20, 'round'),
    (1, 'T4', 4, 50, 20, 'round'),
    (1, 'T5', 4, 67, 20, 'round'),
    (1, 'T6', 6, 32, 52, 'rect'),
    (1, 'T7', 6, 54, 52, 'rect'),
    (1, 'T8', 8, 68, 76, 'rect'),
    -- Pyramid Billiards Club
    (2, 'T1', 2, 32, 30, 'rect'),
    (2, 'T2', 2, 55, 30, 'rect'),
    (2, 'T3', 4, 78, 30, 'rect'),
    (2, 'T4', 4, 32, 64, 'rect'),
    (2, 'T5', 4, 55, 64, 'rect'),
    (2, 'T6', 6, 78, 64, 'rect'),
    -- Medeu Coffee
    (3, 'T1', 2, 12, 32, 'round'),
    (3, 'T2', 2, 12, 58, 'round'),
    (3, 'T3', 2, 30, 80, 'round'),
    (3, 'T4', 4, 36, 42, 'round'),
    (3, 'T5', 4, 60, 48, 'rect'),
    (3, 'T6', 6, 82, 62, 'rect'),
    -- Steppe Grill
    (4, 'T1', 2, 22, 15, 'round'),
    (4, 'T2', 2, 35, 15, 'round'),
    (4, 'T3', 4, 50, 15, 'round'),
    (4, 'T4', 4, 65, 15, 'round'),
    (4, 'T5', 4, 24, 45, 'round'),
    (4, 'T6', 6, 42, 45, 'rect'),
    (4, 'T7', 6, 60, 45, 'rect'),
    (4, 'T8', 8, 85, 58, 'rect'),
    (4, 'T9', 4, 32, 74, 'round'),
    (4, 'T10', 2, 48, 76, 'round'),
    -- Skyline Bar
    (5, 'T1', 2, 28, 28, 'round'),
    (5, 'T2', 2, 43, 28, 'round'),
    (5, 'T3', 2, 58, 28, 'round'),
    (5, 'T4', 2, 73, 28, 'round'),
    (5, 'T5', 4, 84, 45, 'rect'),
    (5, 'T6', 4, 84, 66, 'rect'),
    (5, 'T7', 6, 35, 56, 'rect'),
    (5, 'T8', 6, 58, 56, 'rect'),
    (5, 'T9', 4, 46, 80, 'round'),
    -- Voice Box Karaoke
    (6, 'R1', 6, 14, 25, 'rect'),
    (6, 'R2', 8, 38, 25, 'rect'),
    (6, 'R3', 8, 62, 25, 'rect'),
    (6, 'R4', 12, 86, 25, 'rect'),
    (6, 'H1', 4, 22, 62, 'round'),
    (6, 'H2', 4, 42, 62, 'round'),
    (6, 'H3', 4, 62, 62, 'round'),
    (6, 'H4', 2, 66, 80, 'round'),
    -- Dastarkhan Hall
    (7, 'T1', 10, 30, 32, 'round'),
    (7, 'T2', 10, 52, 32, 'round'),
    (7, 'T3', 10, 74, 32, 'round'),
    (7, 'T4', 10, 28, 64, 'round'),
    (7, 'T5', 10, 76, 64, 'round'),
    (7, 'T6', 8, 90, 40, 'round'),
    (7, 'T7', 6, 12, 50, 'round'),
    (7, 'T8', 6, 12, 76, 'round'),
    -- Shanyrak Tea House
    (8, 'T1', 4, 20, 30, 'rect'),
    (8, 'T2', 4, 20, 62, 'rect'),
    (8, 'T3', 6, 40, 40, 'rect'),
    (8, 'T4', 6, 62, 40, 'rect'),
    (8, 'T5', 2, 80, 32, 'round'),
    (8, 'T6', 2, 88, 55, 'round'),
    (8, 'T7', 8, 70, 70, 'rect'),
    (8, 'T8', 4, 35, 74, 'round'),
    -- Arbat Books & Coffee
    (9, 'T1', 2, 25, 28, 'round'),
    (9, 'T2', 2, 40, 28, 'round'),
    (9, 'T3', 4, 28, 58, 'round'),
    (9, 'T4', 4, 48, 52, 'rect'),
    (9, 'T5', 6, 78, 45, 'rect'),
    (9, 'T6', 2, 78, 72, 'round'),
    (9, 'T7', 2, 12, 66, 'round'),
    -- Cue Club Almaty
    (10, 'T1', 2, 30, 26, 'rect'),
    (10, 'T2', 2, 52, 26, 'rect'),
    (10, 'T3', 4, 74, 26, 'rect'),
    (10, 'T4', 4, 30, 56, 'rect'),
    (10, 'T5', 4, 52, 56, 'rect'),
    (10, 'T6', 6, 74, 56, 'rect')
ON CONFLICT (venue_id, label) DO NOTHING;

COMMIT;

