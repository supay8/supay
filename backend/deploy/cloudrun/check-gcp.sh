#!/usr/bin/env sh
set -eu

: "${PROJECT_ID:?PROJECT_ID is required}"
: "${REGION:?REGION is required}"

if ! command -v gcloud >/dev/null 2>&1; then
  printf 'gcloud is required\n' >&2
  exit 1
fi

account=$(gcloud config get-value account 2>/dev/null)
configured_project=$(gcloud config get-value project 2>/dev/null)
if [ -z "$account" ] || [ "$account" = '(unset)' ]; then
  printf 'gcloud has no active account\n' >&2
  exit 1
fi
if [ "$configured_project" != "$PROJECT_ID" ]; then
  printf 'gcloud project mismatch: configured=%s expected=%s\n' "$configured_project" "$PROJECT_ID" >&2
  exit 1
fi

runtime="supay-runtime@$PROJECT_ID.iam.gserviceaccount.com"
gcloud iam service-accounts describe "$runtime" --project "$PROJECT_ID" >/dev/null

missing=0
for secret in \
  supay-database-url \
  supay-backend-secret \
  supay-encryption-key \
  supay-r2-account-id \
  supay-r2-access-key-id \
  supay-r2-secret-access-key \
  supay-siat-codigo-sistema
do
  if ! gcloud secrets versions list "$secret" \
    --project "$PROJECT_ID" \
    --filter='state=ENABLED' \
    --limit=1 \
    --format='value(name)' | grep -q .
  then
    printf 'missing enabled secret version: %s\n' "$secret" >&2
    missing=1
  fi
  if ! gcloud secrets get-iam-policy "$secret" \
    --project "$PROJECT_ID" \
    --flatten='bindings[].members' \
    --filter="bindings.role=roles/secretmanager.secretAccessor AND bindings.members=serviceAccount:$runtime" \
    --format='value(bindings.role)' | grep -q '^roles/secretmanager.secretAccessor$'
  then
    printf 'runtime lacks secretAccessor on: %s\n' "$secret" >&2
    missing=1
  fi
done

if [ "$missing" -ne 0 ]; then
  exit 1
fi

service_url=$(gcloud run services describe supay-beta --project "$PROJECT_ID" --region "$REGION" --format='value(status.url)' 2>/dev/null || true)
printf 'GCP preflight passed for %s (%s); service=%s\n' "$PROJECT_ID" "$account" "${service_url:-not deployed}"
