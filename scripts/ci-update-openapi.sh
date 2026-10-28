#!/usr/bin/env bash
# Scheduled CI entrypoint: refresh OpenAPI, regenerate client, push.
# Modeled on gitlab.com/brokerage-api/ticker-symbols (.gitlab-ci.yml).
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

if [[ -f "$ROOT/.env" ]]; then
  set -a
  # shellcheck disable=SC1091
  source "$ROOT/.env"
  set +a
fi

export PATH="$(go env GOPATH)/bin:${PATH}"

if ! command -v oapi-codegen >/dev/null 2>&1; then
  echo "installing oapi-codegen..."
  go install github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen@latest
fi

echo "fetching latest OpenAPI spec and regenerating client..."
FETCH_SPEC=1 ./scripts/generate.sh

echo "tidying modules..."
go mod tidy

echo "running unit tests..."
go test ./...

if [[ "${SKIP_PUSH:-0}" == "1" ]]; then
  echo "SKIP_PUSH=1; leaving changes uncommitted"
  git status --short
  exit 0
fi

echo "configuring git..."
git config user.email "${GIT_AUTHOR_EMAIL:-cicd@gitlab.com}"
git config user.name "${GIT_AUTHOR_NAME:-GitLab CI/CD}"

git add openapi.json client/generated.go go.mod go.sum

if git diff --cached --quiet; then
  echo "OpenAPI client is already up to date; nothing to push"
  exit 0
fi

branch="${CI_COMMIT_REF_NAME:-$(git rev-parse --abbrev-ref HEAD)}"
project_path="${CI_PROJECT_PATH:-torn-city/go-torn-client}"
remote_url="https://oauth2:${GITLAB_TOKEN}@gitlab.com/${project_path}.git"

echo "committing regenerated OpenAPI client..."
git commit -m "CI: regenerate Torn API client from upstream OpenAPI"

if [[ -z "${GITLAB_TOKEN:-}" ]]; then
  echo "GITLAB_TOKEN is required to push" >&2
  exit 1
fi

echo "pushing to GitLab..."
git remote remove gitlab_origin 2>/dev/null || true
git remote add gitlab_origin "$remote_url"
git push gitlab_origin "HEAD:${branch}" -o ci.skip

echo "done"
