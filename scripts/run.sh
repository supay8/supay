#!/usr/bin/env bash
set -euo pipefail

# Wrapper legacy para SELF_HOSTED=true.
# Fuente de verdad en Go es internal/config/config.go (DEPLOYMENT_MODE/STORAGE_DRIVER).
# Este script solo traduce el alias y elige el profile de compose.

ROOT_DIR="$(cd "$(dirname "$0")/.." && pwd)"
ENV_FILE="$ROOT_DIR/backend/.env"

if [[ -f "$ENV_FILE" ]]; then
  set -a
  # shellcheck disable=SC1090
  source "$ENV_FILE"
  set +a
fi

# Legacy alias
if [[ "${SELF_HOSTED:-}" == "true" || "${SELF_HOSTED:-}" == "1" ]]; then
  export DEPLOYMENT_MODE=selfhosted
  echo "[run.sh] SELF_HOSTED=true detectado → DEPLOYMENT_MODE=selfhosted (alias legacy)"
fi

DEPLOYMENT_MODE="${DEPLOYMENT_MODE:-selfhosted}"
# Default condicional: selfhosted -> local (disco), cloud -> r2
if [[ -z "${STORAGE_DRIVER:-}" ]]; then
  if [[ "$DEPLOYMENT_MODE" == "cloud" ]]; then
    STORAGE_DRIVER="r2"
  else
    STORAGE_DRIVER="local"
  fi
fi

if [[ "$DEPLOYMENT_MODE" == "cloud" ]]; then
  PROFILE="cloud"
  echo "[run.sh] Modo cloud (STORAGE_DRIVER=$STORAGE_DRIVER) → profile cloud"
else
  PROFILE="selfhosted"
  if [[ "$STORAGE_DRIVER" == "r2" ]]; then
    echo "[run.sh] WARN: STORAGE_DRIVER=r2 en selfhosted → forzado a local (disco)."
    export STORAGE_DRIVER=local
  fi
  echo "[run.sh] Modo selfhosted (STORAGE_DRIVER=$STORAGE_DRIVER) → profile selfhosted"
fi

exec docker compose --profile "$PROFILE" up "$@"
