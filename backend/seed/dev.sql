-- Development seed data. Safe to run repeatedly.
-- Makes the default dev user (telegram_id 1) a staff member of restaurant 1.

INSERT INTO users (telegram_id, name, username, role)
VALUES (1, 'Dev User', 'dev_1', 'staff')
ON CONFLICT (telegram_id) DO UPDATE SET role = EXCLUDED.role;

INSERT INTO restaurants (id, name, address, description, timezone)
OVERRIDING SYSTEM VALUE
VALUES
    (1, 'Bistro Nomad', '12 Abay Ave', 'Modern Central Asian cuisine.', 'Asia/Almaty'),
    (2, 'Trattoria Sole', '5 Dostyk St', 'Wood-fired pizza and pasta.', 'Asia/Almaty')
ON CONFLICT (id) DO NOTHING;
SELECT setval(pg_get_serial_sequence('restaurants', 'id'), GREATEST((SELECT max(id) FROM restaurants), 1));

INSERT INTO restaurant_staff (restaurant_id, telegram_id)
VALUES (1, 1)
ON CONFLICT DO NOTHING;
