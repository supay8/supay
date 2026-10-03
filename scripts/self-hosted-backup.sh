#!/usr/bin/env sh
set -eu

: "${POSTGRES_USER:=postgres}"
: "${POSTGRES_DB:=supay}"
: "${POSTGRES_HOST:=localhost}"
: "${POSTGRES_PORT:=5432}"
: "${POSTGRES_PASSWORD:?POSTGRES_PASSWORD is required}"
: "${BACKUP_DIR:=./backups/postgres}"

timestamp=$(date -u +%Y%m%dT%H%M%SZ)
mkdir -p "$BACKUP_DIR"
backup_file="$BACKUP_DIR/${POSTGRES_DB}_${timestamp}.dump"

PGPASSWORD="$POSTGRES_PASSWORD" pg_dump \
  --format=custom \
  --no-owner \
  --no-acl \
  --file="$backup_file" \
  --host="$POSTGRES_HOST" \
  --port="$POSTGRES_PORT" \
  --username="$POSTGRES_USER" \
  "$POSTGRES_DB"

printf 'PostgreSQL backup written to %s\n' "$backup_file"