# TableBot

Telegram Mini App for restaurant table reservations.

> **Status: skeleton.** Routing, auth, config, DB schema and deploy wiring are
> in place. Handlers return mock data. Business logic is still TODO. The one
> exception is Telegram `initData` validation, which is fully implemented and
> tested.

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
make seed                 # optional: makes dev user 1 a staff member
```

- App: http://localhost:5173 (Vite proxies `/api` to `:8080`)
- API: http://localhost:8080/api/health

In a normal browser there is no Telegram, so the frontend sends
`Authorization: dev <VITE_DEV_TELEGRAM_ID>`. The backend accepts this **only**
when `APP_ENV=dev`. With `make seed`, id `1` is staff and any other id is a
customer.

### Inside Telegram

1. Create a bot with [@BotFather](https://t.me/BotFather) and put the token in `BOT_TOKEN`.
2. Expose Vite over HTTPS, e.g. `cloudflared tunnel --url http://localhost:5173`
   or `ngrok http 5173`. `*.trycloudflare.com` and `*.ngrok-free.app` are
   already allowed in `vite.config.js`.
3. Set `WEBAPP_URL` to the tunnel URL and restart `make dev`. Send `/start`
   to the bot and tap **Book a table**. You can also set the URL as the bot's
   menu button in BotFather.

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
| `make seed` | Load `backend/seed/dev.sql` |
| `make build` | `backend/bin/{api,migrate}` + `frontend/dist` |
| `make test` | `go test -race ./...` |
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
| `REMINDER_INTERVAL` | `1m` | Reminder worker tick |
| `SHUTDOWN_TIMEOUT` | `10s` | Graceful HTTP shutdown timeout |
| `POSTGRES_USER` / `POSTGRES_PASSWORD` / `POSTGRES_DB` / `POSTGRES_PORT` | `tablebot` / `tablebot` / `tablebot` / `5432` | Compose Postgres |
| `SITE_ADDRESS` | `:80` | Caddy site address |
| `VITE_DEV_TELEGRAM_ID` | `1` | Frontend dev-auth user id (outside Telegram only) |

## API

See [`docs/api.yaml`](docs/api.yaml) (OpenAPI 3.1). All routes live under
`/api`. Errors always look like `{"error": "code", "message": "human text"}`.

| Method | Path | Auth |
|--------|------|------|
| GET  | `/api/health` | none |
| GET  | `/api/me` | user |
| GET  | `/api/restaurants` | user |
| GET  | `/api/restaurants/{id}` | user |
| GET  | `/api/restaurants/{id}/slots?date=YYYY-MM-DD&guests=N` | user |
| POST | `/api/bookings` | user |
| GET  | `/api/bookings/my` | user |
| POST | `/api/bookings/{id}/cancel` | user |
| GET  | `/api/staff/bookings?date=YYYY-MM-DD` | staff |
| POST | `/api/staff/bookings/{id}/confirm` | staff |

## Project structure

```
backend/
  cmd/api/            main.go (HTTP + bot + reminder worker, graceful shutdown), router.go
  cmd/migrate/        goose runner using the embedded migrations (up|down|status)
  internal/
    auth/             initData validation, auth middleware, dev bypass, /me
    restaurant/       handler / service / repo (repo returns mock data)
    booking/          handler / service / repo (mock)
    staff/            handler / service / repo (role lookup is real SQL; the rest is mock)
    bot/              go-telegram/bot setup, /start, Notifier interface
    notify/           reminder worker (ticker only)
    platform/         config, database (pgxpool), logging (slog), httpx (JSON, errors, middleware)
  migrations/         0001_init.sql + embed.go
  seed/dev.sql        dev data
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

- `capacity_rules` is a weekly template (weekday, hours, slot length, seats).
  `time_slots` holds concrete slots generated from it.
- Overbooking is prevented in the database. `time_slots.booked_guests` has
  `CHECK (booked_guests <= capacity)` and is updated in the same transaction
  that inserts or cancels a booking, after `SELECT ... FOR UPDATE` on the slot.
- `bookings` references `time_slots (id, restaurant_id, starts_at)` with a
  composite FK, so the denormalised `restaurant_id` and `starts_at` can't drift
  from the slot.
- A partial unique index allows one active (`pending` or `confirmed`) booking
  per user per slot.
- A partial index on `starts_at WHERE status='confirmed' AND NOT reminder_sent`
  serves the reminder worker.
