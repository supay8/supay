#!/usr/bin/env sh
set -eu

: "${API_URL:=http://localhost:8081}"
: "${SMOKE_INVOICE_JSON:?SMOKE_INVOICE_JSON is required}"
: "${SMOKE_API_KEY:?SMOKE_API_KEY is required}"
: "${SMOKE_ANNUL_REASON:=1}"
: "${SMOKE_IDEMPOTENCY_KEY:=self-hosted-smoke-$(date +%s)}"

if ! command -v jq >/dev/null 2>&1; then
  printf 'jq is required to parse the invoice response\n' >&2
  exit 1
fi

tmp_dir=$(mktemp -d)
trap 'rm -rf "$tmp_dir"' EXIT HUP INT TERM

curl --fail --silent --show-error "$API_URL/health" >/dev/null

invoice_response=$(curl --fail --silent --show-error \
  -X POST "$API_URL/v1/invoices/emit" \
  -H 'Content-Type: application/json' \
  -H "X-API-Key: $SMOKE_API_KEY" \
  -H "Idempotency-Key: $SMOKE_IDEMPOTENCY_KEY" \
  --data "$SMOKE_INVOICE_JSON")

invoice_id=$(printf '%s' "$invoice_response" | jq -er '.id')
invoice_status=$(printf '%s' "$invoice_response" | jq -er '.status')
case "$invoice_status" in
  ACCEPTED|OBSERVED) ;;
  *)
    printf 'Invoice %s was not emitted online (status=%s)\n' "$invoice_id" "$invoice_status" >&2
    exit 1
    ;;
esac

curl --fail --silent --show-error \
  -H "X-API-Key: $SMOKE_API_KEY" \
  "$API_URL/v1/invoices/$invoice_id/xml" >"$tmp_dir/invoice.xml"
curl --fail --silent --show-error \
  -H "X-API-Key: $SMOKE_API_KEY" \
  "$API_URL/v1/invoices/$invoice_id/pdf" >"$tmp_dir/invoice.pdf"

if ! grep -aq '<' "$tmp_dir/invoice.xml"; then
  printf 'Downloaded XML for invoice %s is empty or invalid\n' "$invoice_id" >&2
  exit 1
fi
if [ "$(dd if="$tmp_dir/invoice.pdf" bs=5 count=1 2>/dev/null)" != '%PDF-' ]; then
  printf 'Downloaded PDF for invoice %s is empty or invalid\n' "$invoice_id" >&2
  exit 1
fi

curl --fail --silent --show-error \
  -X POST \
  -H 'Content-Type: application/json' \
  -H "X-API-Key: $SMOKE_API_KEY" \
  --data "{\"codigo_motivo\":$SMOKE_ANNUL_REASON}" \
  "$API_URL/v1/invoices/$invoice_id/annul" >"$tmp_dir/annul.json"

annul_status=$(jq -er '.status' "$tmp_dir/annul.json")
if [ "$annul_status" != 'CANCELLED' ]; then
  printf 'Invoice %s was not cancelled (status=%s)\n' "$invoice_id" "$annul_status" >&2
  exit 1
fi

printf 'Self-hosted smoke passed for invoice %s\n' "$invoice_id"
