# TableBot

Telegram Mini App for booking a specific table at a venue: restaurants, cafes,
bars, billiards, karaoke. The guest picks a time and a table on the venue's floor
plan, the venue owner confirms or rejects, and the database makes double booking
impossible.

> **Status: MVP feature-complete on the backend.** Every endpoint in
> `docs/api.yaml` is implemented, plus Telegram notifications (with confirm/reject
> buttons for owners) and a worker that expires unanswered requests and sends reminders.

## Stack

| Layer    | Tech |
|----------|------|
| Backend  | Go, chi, pgx/v5 (pgxpool), goose, go-telegram/bot, slog |
| Frontend | Vue 3 + Vite (JS), vue-router (history mode), Pinia |
| Database | PostgreSQL 16 |
| Deploy   | Docker Compose + Caddy (serves `frontend/dist`, proxies `/api`) |

## Run locally

Prerequisites: Go 1.26+, Node 20.19+ (22 recommended), Docker.

```bash
cp .env.example .env      # APP_ENV=dev by default
make dev                  # starts Postgres in docker, migrates, runs API + Vite
make seed                 # 3 venues in Almaty owned by dev user 1; user 2 is a customer
make seed OWNER=123456789 # same, but your own Telegram id owns the venues
```

If you ran the old scaffold before (slot/capacity schema), run `make db-reset`
once: it deletes the local Postgres volume and applies the new migration.

- App: http://localhost:5173 (Vite proxies `/api` to `:8080`)
- API: http://localhost:8080/api/health

In a normal browser there is no Telegram, so the frontend sends
`Authorization: dev <VITE_DEV_TELEGRAM_ID>`. The backend accepts this **only**
when `APP_ENV=dev`. With `make seed`, id `1` owns all seeded venues and any other
id is a plain customer. Users are created automatically on their first request.

### Inside Telegram

1. Create a bot with [@BotFather](https://t.me/BotFather) and put the token in `BOT_TOKEN`.
2. Expose Vite over HTTPS, e.g. `cloudflared tunnel --url http://localhost:5173`
   or `ngrok http 5173`. `*.trycloudflare.com` and `*.ngrok-free.app` are
   already allowed in `vite.config.js`.
3. Set `WEBAPP_URL` to the tunnel URL and restart `make dev`. On startup the
   API sets the bot's menu button to **Open TableBot**. Send `/start` and tap
   **Open TableBot**.
4. To receive owner notifications, run `make seed OWNER=<your telegram id>` and
   press **Start** in the bot once (Telegram only lets bots message users who did).

### Full stack in Docker

```bash
make up      # builds frontend/dist, then postgres + migrate + api + caddy on :80
make logs
make down
```

Set `SITE_ADDRESS=your.domain` for automatic HTTPS via Caddy.

## Make targets

| Target | Description |
|--------|-------------|
| `make dev` | Postgres (docker) + migrations + API and Vite on the host |
| `make migrate-up` / `migrate-down` / `migrate-status` | goose migrations (embedded, via `cmd/migrate`) |
| `make seed [OWNER=<id>]` | Load `backend/seed/dev.sql` (re-runnable) |
| `make db-reset` | **Delete** the local Postgres volume, then migrate from scratch |
| `make build` | `backend/bin/{api,migrate}` + `frontend/dist` |
| `make test` | `go test -race ./...` (DB tests are skipped) |
| `make test-db` | All tests including DB integration tests, against `tablebot_test` in the compose Postgres |
| `make lint` | gofmt check, `go vet`, golangci-lint if installed |
| `make up` / `down` / `logs` | Full docker stack |

## Environment variables

All variables are read from `.env`. See `.env.example`.

| Variable | Default | Notes |
|----------|---------|-------|
| `APP_ENV` | `prod` | `dev`, `staging` or `prod`. Only `dev` enables the dev auth bypass; any other value fails startup. |
| `HTTP_ADDR` | `:8080` | API listen address |
| `LOG_LEVEL` | `info` | `debug`, `info`, `warn`, `error`. Text logs in dev, JSON otherwise. |
| `DATABASE_URL` | (required) | Postgres URL for the host. Compose builds its own URL for containers. |
| `BOT_TOKEN` | (required outside dev) | Bot token. It is also the key used to validate `initData`. When empty in dev, the bot is disabled. |
| `WEBAPP_URL` | (empty) | HTTPS URL for the `/start` button |
| `HOLD_MINUTES` | `15` | How long a pending booking waits for the owner before it expires |
| `REMINDER_INTERVAL` | `1m` | Worker tick (expire overdue holds, send reminders) |
| `REMINDER_BEFORE` | `2h` | How long before the start a confirmed guest gets a reminder |
| `SHUTDOWN_TIMEOUT` | `10s` | Graceful HTTP shutdown timeout |
| `POSTGRES_USER` / `POSTGRES_PASSWORD` / `POSTGRES_DB` / `POSTGRES_PORT` | `tablebot` / `tablebot` / `tablebot` / `5432` | Compose Postgres |
| `SITE_ADDRESS` | `:80` | Caddy site address |
| `VITE_DEV_TELEGRAM_ID` | `1` | Frontend dev-auth user id (outside Telegram only) |

## API

See [`docs/api.yaml`](docs/api.yaml) (OpenAPI 3.1). All routes live under
`/api`. Errors always look like `{"error": "code", "message": "human text"}`.

| Method | Path | Auth | Status |
|--------|------|------|--------|
| GET  | `/api/health` | none | done |
| GET  | `/api/me` | user | done |
| GET  | `/api/static/plans/{file}` | none | done |
| GET  | `/api/venues?category=` | user | done |
| GET  | `/api/venues/{id}` | user | done |
| GET  | `/api/venues/{id}/times?date=YYYY-MM-DD&guests=N` | user | done |
| GET  | `/api/venues/{id}/tables?starts_at=<RFC3339>&guests=N` | user | done |
| POST | `/api/bookings` | user | done |
| GET  | `/api/bookings/my` | user | done |
| POST | `/api/bookings/{id}/cancel` | user | done |
| GET  | `/api/owner/bookings?venue_id=&date=YYYY-MM-DD` | venue owner | done |
| POST | `/api/owner/bookings/{id}/confirm` | venue owner | done |
| POST | `/api/owner/bookings/{id}/reject` | venue owner | done |

## For the frontend developer

- **Run the backend:** `cp .env.example .env`, then `make dev` (or `make db-up migrate-up dev-api`
  for the API alone on `:8080`), then `make seed`. Vite already proxies `/api` to `:8080`.
- **Auth outside Telegram:** send `Authorization: dev <telegram_id>`. `dev 1` is the owner of
  all seeded venues, `dev 2` is a customer; any other positive id works as a new customer.
  Inside Telegram send `Authorization: tma <Telegram.WebApp.initData>`.
- **Contract:** [`docs/api.yaml`](docs/api.yaml). It lists every endpoint, schema, example and
  error code. Lists are plain arrays; errors are `{"error": "<code>", "message": "..."}`.
- **Floor plans:** `floor_plan_url` is an SVG with `viewBox="0 0 1000 700"` (10:7) and no tables
  drawn. Draw each table centred at `x`% of the width and `y`% of the height.
- **All endpoints are implemented.** The request/response shapes in the contract will
  not change without a note in the PR.
- **Without a bot token** (`BOT_TOKEN` empty), the backend logs every Telegram message it
  would send (`telegram disabled, would send ...`), so you can follow the flow in the API logs.

```bash
curl -H 'Authorization: dev 1' localhost:8080/api/me
curl -o plan.svg localhost:8080/api/static/plans/nomad-bistro.svg
```

## Project structure

```
backend/
  cmd/api/            main.go (HTTP + bot + reminder worker, graceful shutdown), router.go
  cmd/migrate/        goose runner using the embedded migrations (up|down|status)
  internal/
    auth/             initData validation, middleware (dev bypass, user upsert), /me
    bookingstatus/    shared "blocking" / effective-status SQL
    venue/            venues, schedule grid, free times, table map
    booking/          create / my / cancel, Notifier hook
    owner/            owner list, confirm/reject (shared with the bot), per-venue checks
    bot/              Telegram: /start, menu button, notifications, confirm/reject buttons; texts.go
    notify/           worker: expire unanswered requests, send reminders
    platform/         config, database (pgxpool), dbtest, logging (slog), httpx (JSON, errors, middleware)
  migrations/         0001_init.sql + embed.go + schema_test.go
  seed/dev.sql        dev data (3 Almaty venues)
  static/plans/       floor plan SVGs, embedded into the binary
frontend/
  src/api/            client.js (fetch wrapper, typed errors) + one module per domain
  src/composables/    useTelegram.js
  src/views/customer/ RestaurantList, SlotPicker, BookingDone, MyBookings
  src/views/staff/    TodayBookings, Capacity
  src/components/     AppNav, AsyncState
  src/stores/         session (me/role), booking (last created booking)
  src/router/         routes + staff guard
deploy/
  docker-compose.yml  postgres, migrate (one-shot), api, caddy
  Caddyfile
docs/api.yaml
```

### Schema notes

- `users` is keyed by `telegram_id`; `role` is `customer` or `admin`. Being an
  owner is not a role: a user owns venue X when `venues.owner_id` is their id.
- `venues` holds opening hours (`opens_at` < `closes_at`, local time in
  `timezone`, 24:00 allowed), `booking_minutes`, `slot_step_minutes` and
  `floor_plan_url`. `venue_tables` holds each table's seats and plan position.
- `bookings.during` is a `tstzrange` `[start, end)`. Two exclusion constraints
  make double booking impossible for `pending`/`confirmed` rows: one per table
  (`bookings_no_table_overlap`) and one per user across all venues
  (`bookings_no_user_overlap`). Both raise SQLSTATE `23P01`.
- A composite FK `(table_id, venue_id)` guarantees the table belongs to the venue.
- Pending bookings must have `expires_at`; see `CLAUDE.md` for the full booking rules.
