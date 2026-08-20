#!/bin/sh
set -eu

if [ "$#" -ne 1 ]; then
  echo "usage: $0 <image>" >&2
  exit 2
fi

IMAGE=$1
CONTAINER_ID=
CONTENTS=$(mktemp)

cleanup() {
  rm -f "$CONTENTS"
  if [ -n "$CONTAINER_ID" ]; then
    docker rm "$CONTAINER_ID" >/dev/null || true
  fi
}
trap cleanup EXIT INT TERM

CONTAINER_ID=$(docker create "$IMAGE")
docker export "$CONTAINER_ID" | tar -tf - >"$CONTENTS"

for path in \
  opt/rdc-exporter/bin/rdc-exporter \
  opt/rocm/bin/rocm-smi \
  opt/rocm/lib/librdc_bootstrap.so.1 \
  opt/rocm/lib/rdc/librdc_rocp.so.1 \
  opt/rocm/share/rocprofiler-sdk; do
  if ! grep -Fqx "$path" "$CONTENTS" && ! grep -Fqx "$path/" "$CONTENTS"; then
    echo "missing required image path: /$path" >&2
    exit 1
  fi
done

docker image inspect "$IMAGE" \
  --format 'image={{.Id}} size={{.Size}} entrypoint={{json .Config.Entrypoint}}'
