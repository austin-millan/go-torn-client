#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

export PATH="$(go env GOPATH)/bin:${PATH}"

SPEC_SRC="${SPEC_SRC:-https://www.torn.com/swagger/openapi.json}"
SPEC="$ROOT/openapi.json"

if ! command -v oapi-codegen >/dev/null 2>&1; then
  echo "installing oapi-codegen..."
  go install github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen@latest
fi

if [[ "${FETCH_SPEC:-0}" == "1" ]]; then
  echo "fetching $SPEC_SRC"
  curl -fsSL -A "go-torn-client/generate" -o "$SPEC" "$SPEC_SRC"
fi

python3 "$ROOT/scripts/preprocess_openapi.py" "$SPEC"
oapi-codegen -config "$ROOT/oapi-codegen.yaml" "$SPEC"
gofmt -w "$ROOT/client/generated.go"
echo "generated $ROOT/client/generated.go"
