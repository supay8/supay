#!/usr/bin/env sh
set -eu

script_dir=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)

: "${PROJECT_ID:?PROJECT_ID is required}"
: "${REGION:?REGION is required}"
: "${APPLY_GCP:?Set APPLY_GCP=yes to deploy}"
: "${MANIFEST_DIR:=$script_dir/rendered}"

if [ "$APPLY_GCP" != yes ]; then
  printf 'Refusing to deploy unless APPLY_GCP=yes\n' >&2
  exit 1
fi
if [ ! -f "$MANIFEST_DIR/migrate-job.yaml" ] || [ ! -f "$MANIFEST_DIR/service.yaml" ]; then
  printf 'Render and inspect the manifests before deploying\n' >&2
  exit 1
fi

"$script_dir/check-gcp.sh"

gcloud run jobs replace "$MANIFEST_DIR/migrate-job.yaml" \
  --project "$PROJECT_ID" \
  --region "$REGION"
gcloud run jobs execute supay-migrate \
  --project "$PROJECT_ID" \
  --region "$REGION" \
  --wait

gcloud run services replace "$MANIFEST_DIR/service.yaml" \
  --project "$PROJECT_ID" \
  --region "$REGION"

gcloud run services describe supay-beta \
  --project "$PROJECT_ID" \
  --region "$REGION" \
  --format='value(status.url)'
