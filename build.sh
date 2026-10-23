#!/usr/bin/env sh
# Builds terraform-ls with embedded provider schemas, using the same go build
# flags as the release build in .github/workflows/build.yml.
#
# Provider schemas are downloaded from this fork's release (published by
# .github/workflows/provider-schemas.yml) when missing, falling back to the
# slow `go generate`. Delete internal/schemas/data/registry.terraform.io to
# refresh them.
#
# Usage: ./build.sh [output path, default: bin/terraform-ls]
set -eu

cd "$(dirname "$0")"

SCHEMAS_URL="https://github.com/haug1/terraform-ls/releases/download/provider-schemas/provider-schemas.tar.gz"
OUT="${1:-bin/terraform-ls}"

if [ ! -d internal/schemas/data/registry.terraform.io ]; then
  if ! curl -fsSL "$SCHEMAS_URL" | tar -xz -C internal/schemas; then
    echo "downloading provider schemas failed, generating them instead" >&2
    go generate ./internal/schemas
  fi
fi

go build \
  -o "$OUT" \
  -trimpath \
  -buildvcs=false \
  -ldflags "-s -w" \
  .
