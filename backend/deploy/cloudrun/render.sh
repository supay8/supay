#!/usr/bin/env sh
set -eu

script_dir=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)

: "${PROJECT_ID:?PROJECT_ID is required}"
: "${PROJECT_NUMBER:?PROJECT_NUMBER is required}"
: "${REGION:?REGION is required}"
: "${TAG:?TAG is required and must be immutable}"
: "${AUTH_HOST:?AUTH_HOST is required, without https://}"
: "${APP_HOST:?APP_HOST is required, without https://}"
: "${R2_BUCKET:=supay-beta-private}"
: "${ENABLE_EMAIL:=false}"
: "${OUTPUT_DIR:=$script_dir/rendered}"
: "${IMAGE:=$REGION-docker.pkg.dev/$PROJECT_ID/supay/backend:$TAG}"

case "$TAG" in
  latest|LATEST) printf 'TAG must be immutable; latest is forbidden\n' >&2; exit 1 ;;
esac
case "$AUTH_HOST:$APP_HOST" in
  *://*|*/*) printf 'AUTH_HOST and APP_HOST must be hostnames without scheme or path\n' >&2; exit 1 ;;
esac
case "$ENABLE_EMAIL" in
  true)
    : "${EMAIL_WORKER_URL:?EMAIL_WORKER_URL is required when ENABLE_EMAIL=true}"
    : "${SMTP_HOST:?SMTP_HOST is required when ENABLE_EMAIL=true}"
    : "${EMAIL_FROM:?EMAIL_FROM is required when ENABLE_EMAIL=true}"
    ;;
  false) ;;
  *) printf 'ENABLE_EMAIL must be true or false\n' >&2; exit 1 ;;
esac

export PROJECT_ID PROJECT_NUMBER REGION TAG IMAGE AUTH_HOST APP_HOST R2_BUCKET
export EMAIL_WORKER_URL="${EMAIL_WORKER_URL-}"
export SMTP_HOST="${SMTP_HOST-}"
export EMAIL_FROM="${EMAIL_FROM-}"

mkdir -p "$OUTPUT_DIR"
vars='${PROJECT_ID} ${PROJECT_NUMBER} ${REGION} ${TAG} ${IMAGE} ${AUTH_HOST} ${APP_HOST} ${R2_BUCKET} ${EMAIL_WORKER_URL} ${SMTP_HOST} ${EMAIL_FROM}'

render() {
  input=$1
  output=$2
  envsubst "$vars" <"$script_dir/$input" >"$OUTPUT_DIR/$output"
}

render service.yaml service.yaml
render migrate-job.yaml migrate-job.yaml
if [ "$ENABLE_EMAIL" = true ]; then
  render email-worker.yaml email-worker.yaml
  render email-dispatcher-job.yaml email-dispatcher-job.yaml
else
  rm -f "$OUTPUT_DIR/email-worker.yaml" "$OUTPUT_DIR/email-dispatcher-job.yaml"
fi

python3 - "$OUTPUT_DIR" "$ENABLE_EMAIL" <<'PY'
import pathlib
import sys
import yaml

root = pathlib.Path(sys.argv[1])
email_enabled = sys.argv[2] == "true"
files = sorted(root.glob("*.yaml"))
if not files:
    raise SystemExit("no rendered manifests")

docs = {}
for path in files:
    text = path.read_text(encoding="utf-8")
    if "${" in text:
        raise SystemExit(f"unresolved placeholder in {path}")
    docs[path.name] = yaml.safe_load(text)

service = docs["service.yaml"]
annotations = service["spec"]["template"]["metadata"]["annotations"]
if annotations.get("autoscaling.knative.dev/minScale") != "1":
    raise SystemExit("service minScale must be 1 for launch week")
if service["metadata"]["annotations"].get("run.googleapis.com/ingress") != "internal-and-cloud-load-balancing":
    raise SystemExit("service ingress must exclude direct public run.app traffic")

container = service["spec"]["template"]["spec"]["containers"][0]
env = {item["name"]: item.get("value") for item in container["env"]}
required = {
    "DEPLOYMENT_MODE": "cloud",
    "AUTO_MIGRATE": "false",
    "STORAGE_DRIVER": "r2",
    "INVOICE_EMAIL_ENABLED": "false",
}
for name, expected in required.items():
    if env.get(name) != expected:
        raise SystemExit(f"{name} must be {expected!r}")
if "R2_PUBLIC_URL" in env:
    raise SystemExit("R2_PUBLIC_URL must not be configured")
if not container["image"] or container["image"].endswith(":latest"):
    raise SystemExit("service image must use an immutable non-latest tag")
if not env.get("BETTER_AUTH_URL", "").startswith("https://"):
    raise SystemExit("BETTER_AUTH_URL must be HTTPS")
if not env.get("CORS_ALLOWED_ORIGINS", "").startswith("https://"):
    raise SystemExit("CORS_ALLOWED_ORIGINS must be HTTPS")

migrate = docs["migrate-job.yaml"]
command = migrate["spec"]["template"]["spec"]["template"]["spec"]["containers"][0]["command"]
if command != ["/app/supay-migrate"]:
    raise SystemExit("migration Job must run /app/supay-migrate")

if email_enabled and not {"email-worker.yaml", "email-dispatcher-job.yaml"}.issubset(docs):
    raise SystemExit("email manifests missing")
if not email_enabled and ({"email-worker.yaml", "email-dispatcher-job.yaml"} & set(docs)):
    raise SystemExit("email manifests rendered while email is disabled")

print(f"validated {len(files)} Cloud Run manifest(s) in {root}")
PY

printf 'Rendered image: %s\n' "$IMAGE"
