# TableBot

Telegram Mini App for booking a **specific table** at a venue (restaurant, cafe,
bar, billiards, karaoke, ...). The guest picks a time and a table on the venue's
floor plan; the venue owner confirms or rejects in the app. The database makes
double booking impossible.

Stack: Go (chi, pgx/v5, goose, go-telegram/bot, slog), PostgreSQL 16,
Vue 3 + Vite (frontend, owned by another developer), Docker Compose + Caddy.

## Layout

```
backend/
  cmd/api/              main.go (HTTP + bot + worker), router.go (all routes)
  cmd/migrate/          goose runner (up | down | status)
  internal/
    auth/               initData validation, middleware (upserts user), /me, UserRepo
    bookingstatus/      THE blocking / effective-status SQL; use it, never re-type it
    venue/              venues, schedule (start-time grid), free times, table map
    booking/            guest bookings
    owner/              owner list + Decide (confirm/reject, shared by HTTP and bot)
    bot/                Telegram: Sender interface, Notifier, /start, menu button, callbacks;
                        every user-facing text is in bot/texts.go
    notify/             background worker: expire overdue holds, send reminders
    platform/           config, database, dbtest, httpx (JSON, errors), logging
  migrations/           0001_init.sql (+ schema_test.go)
  seed/dev.sql          dev data (psql variable owner_id)
  static/plans/         floor plan SVGs, embedded and served at /api/static/plans/{file}
docs/api.yaml           the API contract (OpenAPI 3.1)
frontend/               Vue app — NOT ours, never touch it in backend tasks
deploy/                 docker-compose.yml, Caddyfile
```

Each domain package uses **handler -> service -> repo**:
- `handler.go`: parse/validate HTTP input, call the service, map errors to JSON.
- `service.go`: business rules, domain errors (`ErrNotFound`, ...), no HTTP.
- `repo.go`: SQL via pgx; returns domain types and domain errors.

## Commands (run from the repo root)

- `make dev` — Postgres in Docker, migrations, API on :8080 and Vite on :5173.
- `make seed` / `make seed OWNER=<telegram_id>` — 3 Almaty venues owned by that user, plus customer user 2.
- `make db-reset` — deletes the local Postgres volume and re-migrates (needed once after schema rewrites).
- `make test` — unit tests; DB tests are skipped (it unsets `TEST_DATABASE_URL`).
- `make test-db` — all tests incl. DB integration tests against the `tablebot_test`
  database in the compose Postgres. Each DB test wipes and re-migrates the `public`
  schema (`dbtest.New`); it refuses to run on a database whose name lacks "test".
  Run by hand: `TEST_DATABASE_URL=postgres://tablebot:tablebot@localhost:5432/tablebot_test?sslmode=disable go test -p 1 ./...` in `backend/`.
  `-p 1` is required whenever more than one DB package runs: they share one database.
- `make lint` — gofmt + go vet.
- Lint the contract: `npx @redocly/cli lint docs/api.yaml`. HTTP tests in
  `cmd/api/api_test.go` validate every response against it (libopenapi-validator); an
  undocumented status code or field fails the test.

## Conventions

- JSON errors are always `{"error": "<code>", "message": "<text>"}` (use `httpx.Error`;
  codes live in `httpx/json.go` and must match `docs/api.yaml`).
- Times in the API are RFC 3339 with offset. Venue-local times (dates, opening hours,
  slot grid, response timestamps) use `venues.timezone`.
- `docs/api.yaml` is the contract the frontend builds against. Any change to it must be
  called out in the PR description.
- Never touch `frontend/` in backend tasks.
- Code comments: short, one line, only where the code is not obvious. No long comment blocks.
- Code must be gofmt + go vet clean.
- Owner access is per venue (`venues.owner_id = current user`), never a global role.
  Owner code must go through `owner.Service` (`CheckVenue` / `CheckBooking` / `List` / `Decide`).
- Telegram: talk to it only through `bot.Sender` (fake it in tests). With an empty
  `BOT_TOKEN`, `bot.LogSender` logs every message instead, so dev runs the same code.
- Floor plans: SVG, `viewBox="0 0 1000 700"`, no tables drawn. Table `x`/`y` are the
  table centre in percent of plan width/height. Rename the file when a plan changes
  (served with a 30-day cache).

## Workflow

- One task per branch, branched from `main`.
- Run `make lint`, `make test` and `make test-db` before pushing.
- Open a PR with `gh pr create`; description has a summary, files/areas changed and
  "How I tested" with the real commands and results. Never merge.
- Never commit `.env`, bot tokens or other secrets.

## Booking rules

- A booking lasts `venues.booking_minutes`. Start times are every `slot_step_minutes`
  from `opens_at`; the booking must end by `closes_at` (24:00 allowed, no overnight
  hours); the start must be in the future.
- New bookings are `pending` with `expires_at = now() + HOLD_MINUTES` (env, default 15).
  Owner confirms -> `confirmed`, rejects -> `rejected` (set `decided_at`).
  The worker flips overdue pending -> `expired`.
  The guest cancels `pending`/`confirmed` -> `cancelled`.
- `guests <= table seats`. One user cannot hold overlapping active bookings anywhere.
- `during` is `[start, end)`, so back-to-back bookings are fine. The DB exclusion
  constraints (`bookings_no_table_overlap`, `bookings_no_user_overlap`) cover rows with
  status `pending` or `confirmed`.
- A booking **blocks** a table/user if `status = 'confirmed' OR (status = 'pending' AND
  expires_at > now())` -> `bookingstatus.BlockingSQL`. Every read reports the **effective
  status** (overdue pending reads as `expired`) -> `bookingstatus.EffectiveSQL`.
  `expires_at` is returned only while the effective status is `pending`.
- **Create-booking transaction** (`booking.Repo.Insert`), in this order:
  1. `SELECT ... FROM venue_tables WHERE id = $table FOR NO KEY UPDATE`, then the same on
     `users` for `$user`. Always table first, then user. Without these locks, racing
     inserts deadlock inside the exclusion constraints (40P01 -> 500).
  2. Expire before insert: `bookingstatus.ExpireOverdueSQL` (overdue pending on this
     table or user -> `expired`).
  3. INSERT. Map SQLSTATE `23P01` on `bookings_no_table_overlap` -> 409 `table_taken`,
     on `bookings_no_user_overlap` -> 409 `user_busy`.
- **Confirm / reject**: `owner.Service.Decide`, used by both the HTTP handler and the bot
  callback (`bk:c:<id>` / `bk:r:<id>`). Ownership check first (403/404), then the single
  atomic `bookingstatus.DecideSQL` (`... WHERE status = 'pending' AND expires_at > now()`);
  0 rows -> 409 `not_pending`.
- **Worker** (`notify.Worker`, every `REMINDER_INTERVAL`): `ExpireAllOverdueSQL` -> tell each
  guest; `ClaimRemindersSQL` (confirmed, not yet reminded, starting within `REMINDER_BEFORE`)
  -> remind each guest. The UPDATE marks rows first, so each guest is told at most once.
- Notifications (`bot.Notifier`) run after commit. Send failures (e.g. the user never
  started the bot) are logged at warn with `booking_id` and never fail a request.
- Venue times: build with `time.Date` in `venue.Location(timezone)`; TIME columns are
  read as minutes since midnight (0..1440, so 24:00 works). `/times`, `/tables` and
  `POST /bookings` accept dates up to `venue.MaxDaysAhead` (30) days ahead.
