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

if [[ -z "${CI_JOB_TOKEN:-}" ]]; then
  echo "CI_JOB_TOKEN is required to push (run in CI with Job token Git push enabled)" >&2
  exit 1
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
server_host="${CI_SERVER_HOST:-gitlab.com}"
project_path="${CI_PROJECT_PATH:?CI_PROJECT_PATH is required}"
remote_url="https://gitlab-ci-token:${CI_JOB_TOKEN}@${server_host}/${project_path}.git"

echo "committing regenerated OpenAPI client..."
git commit -m "CI: regenerate Torn API client from upstream OpenAPI"

echo "pushing to GitLab..."
git remote set-url origin "$remote_url"
git push origin "HEAD:${branch}" -o ci.skip

echo "done"
