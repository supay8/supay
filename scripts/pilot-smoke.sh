#!/usr/bin/env sh
set -eu

: "${API_URL:?API_URL is required (for example https://api.example.com)}"
: "${SMOKE_INVOICE_JSON:?SMOKE_INVOICE_JSON is required}"
: "${SMOKE_ANNUL_REASON:=90}"

if ! command -v jq >/dev/null 2>&1; then
  printf 'jq is required to parse API responses\n' >&2
  exit 1
fi

tmp_dir=$(mktemp -d)
trap 'rm -rf "$tmp_dir"' EXIT HUP INT TERM

api_url=${API_URL%/}
api_key=${SMOKE_API_KEY:-}

if [ -z "$api_key" ]; then
  : "${BACKEND_SECRET:?SMOKE_API_KEY or BACKEND_SECRET is required}"
  : "${SMOKE_BOOTSTRAP_JSON:?SMOKE_BOOTSTRAP_JSON is required when bootstrapping a company}"
  bootstrap_response=$(curl --fail --silent --show-error \
    -X POST "$api_url/internal/companies" \
    -H "X-Backend-Token: $BACKEND_SECRET" \
    -H 'Content-Type: application/json' \
    --data "$SMOKE_BOOTSTRAP_JSON")
  api_key=$(printf '%s' "$bootstrap_response" | jq -er '.api_key')
  printf 'Bootstrap created company %s; store the returned API key securely.\n' \
    "$(printf '%s' "$bootstrap_response" | jq -r '.company.id // .company_id // "(unknown)"')"
fi

auth_header="X-API-Key: $api_key"

curl --fail --silent --show-error "$api_url/health" >"$tmp_dir/health.json"
jq -e '.status == "ok"' "$tmp_dir/health.json" >/dev/null

invoice_response=$(curl --fail --silent --show-error \
  -X POST "$api_url/v1/invoices/emit" \
  -H 'Content-Type: application/json' \
  -H "$auth_header" \
  --data "$SMOKE_INVOICE_JSON")

invoice_id=$(printf '%s' "$invoice_response" | jq -er '.id')
invoice_status=$(printf '%s' "$invoice_response" | jq -er '.status')
case "$invoice_status" in
  ACCEPTED|OBSERVED) ;;
  *)
    printf 'Invoice %s did not finish online (status=%s)\n' "$invoice_id" "$invoice_status" >&2
    exit 1
    ;;
esac

curl --fail --silent --show-error \
  -H "$auth_header" \
  "$api_url/v1/invoices/$invoice_id" >"$tmp_dir/invoice.json"
jq -e --arg id "$invoice_id" '.id == $id and (.status == "ACCEPTED" or .status == "OBSERVED")' \
  "$tmp_dir/invoice.json" >/dev/null

curl --fail --silent --show-error \
  -H "$auth_header" \
  "$api_url/v1/invoices/$invoice_id/siat-status" >"$tmp_dir/siat-status.json"
jq -e 'type == "object"' "$tmp_dir/siat-status.json" >/dev/null

curl --fail --silent --show-error --location \
  -D "$tmp_dir/xml.headers" \
  -H "$auth_header" \
  "$api_url/v1/invoices/$invoice_id/xml" >"$tmp_dir/invoice.xml"
curl --fail --silent --show-error --location \
  -D "$tmp_dir/pdf.headers" \
  -H "$auth_header" \
  "$api_url/v1/invoices/$invoice_id/pdf" >"$tmp_dir/invoice.pdf"

if ! grep -aq '<' "$tmp_dir/invoice.xml"; then
  printf 'Downloaded XML for invoice %s is empty or invalid\n' "$invoice_id" >&2
  exit 1
fi
if [ "$(dd if="$tmp_dir/invoice.pdf" bs=5 count=1 2>/dev/null)" != '%PDF-' ]; then
  printf 'Downloaded PDF for invoice %s is empty or invalid\n' "$invoice_id" >&2
  exit 1
fi
if [ -n "${SMOKE_FORBIDDEN_PUBLIC_HOST:-}" ] && \
  grep -Fqi "$SMOKE_FORBIDDEN_PUBLIC_HOST" "$tmp_dir/xml.headers" "$tmp_dir/pdf.headers"; then
  printf 'A fiscal download exposed forbidden public host %s\n' "$SMOKE_FORBIDDEN_PUBLIC_HOST" >&2
  exit 1
fi

if [ -n "${TENANT_B_API_KEY:-}" ]; then
  for operation in read emit annul; do
    method=GET
    path="/v1/invoices/$invoice_id"
    data_args=
    case "$operation" in
      emit)
        method=POST
        path="$path/emit"
        ;;
      annul)
        method=POST
        path="$path/annul"
        data_args='{"codigo_motivo":90}'
        ;;
    esac
    if [ -n "$data_args" ]; then
      status=$(curl --silent --show-error -o "$tmp_dir/tenant-b-$operation.json" -w '%{http_code}' \
        -X "$method" -H 'Content-Type: application/json' -H "X-API-Key: $TENANT_B_API_KEY" \
        --data "$data_args" "$api_url$path")
    else
      status=$(curl --silent --show-error -o "$tmp_dir/tenant-b-$operation.json" -w '%{http_code}' \
        -X "$method" -H "X-API-Key: $TENANT_B_API_KEY" "$api_url$path")
    fi
    case "$status" in
      401|404) ;;
      *)
        printf 'Tenant B %s returned %s; expected 401 or 404\n' "$operation" "$status" >&2
        exit 1
        ;;
    esac
  done
fi

curl --fail --silent --show-error \
  -X POST \
  -H 'Content-Type: application/json' \
  -H "$auth_header" \
  --data "{\"codigo_motivo\":$SMOKE_ANNUL_REASON}" \
  "$api_url/v1/invoices/$invoice_id/annul" >"$tmp_dir/annul.json"
jq -e '.status == "CANCELLED"' "$tmp_dir/annul.json" >/dev/null

printf 'Pilot gate passed for invoice %s (emit, status, XML, PDF, tenant isolation%s, annul).\n' \
  "$invoice_id" "$(if [ -n "${TENANT_B_API_KEY:-}" ]; then printf ' checked'; else printf ' skipped'; fi)"
