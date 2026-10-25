#!/usr/bin/env bash
# Dumps the production database to $APP_DIR/backups and keeps the newest 7 dumps.
# Run by cron every night (installed by server-setup.sh) and by `make prod-backup`.
# Env overrides (for testing): APP_DIR (default /opt/tablebot), COMPOSE_PROJECT (default tablebot).
set -Eeuo pipefail

APP_DIR="${APP_DIR:-/opt/tablebot}"
COMPOSE_PROJECT="${COMPOSE_PROJECT:-tablebot}"
KEEP=7

cd "$APP_DIR"
mkdir -p backups

file="backups/tablebot-$(date -u +%Y%m%d-%H%M%S).sql.gz"
docker compose -p "$COMPOSE_PROJECT" --env-file .env \
  -f deploy/docker-compose.yml -f deploy/docker-compose.prod.yml \
  exec -T postgres sh -c 'pg_dump -U "$POSTGRES_USER" -d "$POSTGRES_DB" --clean --if-exists' |
  gzip >"$file.tmp"
mv "$file.tmp" "$file"
echo "$(date -u +%FT%TZ) backup written: $APP_DIR/$file ($(du -h "$file" | cut -f1))"

# File names sort by time; delete everything after the newest $KEEP.
n=0
while IFS= read -r old; do
  n=$((n + 1))
  if ((n > KEEP)); then
    rm -f -- "$old"
    echo "deleted old backup: $old"
  fi
done < <(find backups -maxdepth 1 -name 'tablebot-*.sql.gz' | sort -r)
