#!/usr/bin/env bash
# validate-nginx.sh — Validate app/nginx.conf: syntax (`nginx -t`) + security scan (gixy).
#
# The config is a deploy-time template, so validation can't run on the raw file:
#   1. ${VITE_API_URL} / ${VITE_DNS_CHECK_DOMAIN} are substituted by envsubst in
#      the Dockerfile. nginx parses `$...` as variable references even inside
#      quoted strings, so the un-rendered template fails with "unknown variable".
#      We render it with dummy values (we only check structure, not the URLs).
#   2. `include /etc/nginx/access_rules.conf` is mounted at deploy time. The
#      nginx -t step stubs it with an empty file; the gixy step uses
#      --disable-includes so it doesn't try to follow host-only includes.
#
# Stage 1 (nginx -t) runs in the SAME nginx image pinned in app/Dockerfile, so the
# directive set tested matches production exactly.
# Stage 2 (gixy) uses gixy-ng — the maintained fork — for security findings such as
# version disclosure, add_header pitfalls, and SSRF/host-header issues.
#
# Usage: ./scripts/validate-nginx.sh [path/to/nginx.conf]
#   Defaults to app/nginx.conf relative to the repo root.
#   SKIP_GIXY=1 runs syntax only (e.g. offline — gixy pip-installs in a container).
#
# Requirements: docker, envsubst (gettext). The gixy stage needs network on first
# run to pip-install gixy-ng inside the python container.
#
# Exit codes:
#   0 — configuration is valid and clean
#   1 — nginx -t syntax test failed
#   2 — missing prerequisite (docker / envsubst / config file)
#   3 — gixy ran and reported security findings (CI treats as advisory)
#   4 — gixy could not run (image pull / pip / docker failure; CI must NOT ignore)

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"

CONF="${1:-${REPO_ROOT}/app/nginx.conf}"
DOCKERFILE="${REPO_ROOT}/app/Dockerfile"
GIXY_IMAGE="${GIXY_IMAGE:-python:3.12-alpine}"

# gixy-ng (getpagespeed fork) bundles performance opinions alongside security
# checks. Skip the non-security ones so the gate reflects security posture:
#   try_files_is_evil_too            — perf blog opinion; try_files is the standard
#                                      SPA routing pattern for this static bundle.
#   worker_rlimit_nofile_vs_connections — ops/fd tuning, managed at the container
#                                      runtime level, not a config security issue.
GIXY_SKIPS="try_files_is_evil_too,worker_rlimit_nofile_vs_connections"

# Dummy values — we only care about structure, not the substituted URLs.
export VITE_API_URL="${VITE_API_URL:-https://api.example.com}"
export VITE_DNS_CHECK_DOMAIN="${VITE_DNS_CHECK_DOMAIN:-check.example.com}"

err() { echo "validate-nginx: $*" >&2; }

command -v docker   >/dev/null 2>&1 || { err "docker not found"; exit 2; }
command -v envsubst >/dev/null 2>&1 || { err "envsubst not found (install gettext)"; exit 2; }
[ -f "${CONF}" ] || { err "config not found: ${CONF}"; exit 2; }

# Stay in sync with production: use the nginx image pinned in app/Dockerfile.
NGINX_IMAGE="$(sed -n 's/^FROM \(nginx:[^ ]*\).*/\1/p' "${DOCKERFILE}" 2>/dev/null | head -1)"
NGINX_IMAGE="${NGINX_IMAGE:-nginx:alpine}"

# Render into a temp dir UNDER the repo. snap-confined docker cannot bind-mount
# /tmp, and it rejects single-file mounts, so we mount this directory instead.
WORK="$(mktemp -d "${REPO_ROOT}/.nginx-validate.XXXXXX")"
trap 'rm -rf "${WORK}"' EXIT

REL="${CONF#"${REPO_ROOT}/"}"
envsubst '${VITE_API_URL} ${VITE_DNS_CHECK_DOMAIN}' < "${CONF}" > "${WORK}/nginx.conf"

echo "validate-nginx: [1/2] syntax check ${REL} (${NGINX_IMAGE})"
docker run --rm -v "${WORK}:/work:ro" "${NGINX_IMAGE}" sh -c '
  cp /work/nginx.conf /etc/nginx/nginx.conf &&
  : > /etc/nginx/access_rules.conf &&
  nginx -t
' || { err "nginx -t failed"; exit 1; }

if [ "${SKIP_GIXY:-0}" = "1" ]; then
  echo "validate-nginx: [2/2] gixy scan skipped (SKIP_GIXY=1)"
else
  echo "validate-nginx: [2/2] security scan (gixy-ng)"
  # Distinguish "gixy ran and found issues" from "gixy never ran". The inner
  # script exits 40 if setup (pip) fails; docker itself returns 125-127 if the
  # image can't be pulled or the container can't start. Everything else non-zero
  # is gixy's own exit — i.e. real findings. Conflating the two would let a
  # broken scanner pass silently when CI downgrades findings to a warning.
  set +e
  docker run --rm -e GIXY_SKIPS="${GIXY_SKIPS}" -v "${WORK}:/work:ro" "${GIXY_IMAGE}" sh -c '
    pip install --quiet --disable-pip-version-check --root-user-action=ignore gixy-ng || exit 40
    gixy --disable-includes --skips="${GIXY_SKIPS}" /work/nginx.conf
  '
  rc=$?
  set -e
  case "${rc}" in
    0) ;;
    40|125|126|127) err "gixy could not run (setup/docker failure, rc=${rc})"; exit 4 ;;
    *) err "gixy reported security findings (rc=${rc})"; exit 3 ;;
  esac
fi

echo "validate-nginx: OK"
