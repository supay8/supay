#!/usr/bin/env sh
set -eu

: "${PROJECT_ID:?PROJECT_ID is required}"
: "${APPLY_GCP:?Set APPLY_GCP=yes to create GCP resources}"

if [ "$APPLY_GCP" != yes ]; then
  printf 'Refusing to mutate GCP unless APPLY_GCP=yes\n' >&2
  exit 1
fi
if ! command -v gcloud >/dev/null 2>&1; then
  printf 'gcloud is required\n' >&2
  exit 1
fi

configured_project=$(gcloud config get-value project 2>/dev/null)
if [ "$configured_project" != "$PROJECT_ID" ]; then
  printf 'gcloud project mismatch: configured=%s expected=%s\n' "$configured_project" "$PROJECT_ID" >&2
  exit 1
fi

runtime="supay-runtime@$PROJECT_ID.iam.gserviceaccount.com"
if ! gcloud iam service-accounts describe "$runtime" --project "$PROJECT_ID" >/dev/null 2>&1; then
  gcloud iam service-accounts create supay-runtime \
    --project "$PROJECT_ID" \
    --display-name 'Supay Cloud Run runtime'
fi

for secret in \
  supay-database-url \
  supay-backend-secret \
  supay-encryption-key \
  supay-r2-account-id \
  supay-r2-access-key-id \
  supay-r2-secret-access-key \
  supay-siat-codigo-sistema
do
  if ! gcloud secrets describe "$secret" --project "$PROJECT_ID" >/dev/null 2>&1; then
    gcloud secrets create "$secret" --project "$PROJECT_ID" --replication-policy automatic
  fi
  gcloud secrets add-iam-policy-binding "$secret" \
    --project "$PROJECT_ID" \
    --member "serviceAccount:$runtime" \
    --role roles/secretmanager.secretAccessor >/dev/null
done

printf '%s\n' \
  'Secret containers and runtime IAM are ready.' \
  'Add secret values with: printf %s "$VALUE" | gcloud secrets versions add SECRET --data-file=-'
