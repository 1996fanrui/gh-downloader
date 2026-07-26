#!/usr/bin/env bash
# Deploy gh-downloader with Docker Compose.
#
# Usage:
#   OPENAI_API_KEY=... OPENAI_BASE_URL=... OPENAI_MODEL=... bash scripts/deploy.sh
#   GH_DOWNLOADER_PUBLIC_URL=https://gh-downloader.com bash scripts/deploy.sh

set -e

require_env() {
    local name="$1"
    if [[ -z "${!name:-}" ]]; then
        echo "$name is required" >&2
        exit 1
    fi
}

normalize_path() {
    local value="$1"
    case "$value" in
        "~")
            printf '%s\n' "$HOME"
            ;;
        "~/"*)
            printf '%s/%s\n' "$HOME" "${value#"~/"}"
            ;;
        *)
            printf '%s\n' "$value"
            ;;
    esac
}

probe_url() {
    local url="$1"
    local label="$2"
    local status=""

    for _ in {1..30}; do
        status="$(curl -sS --max-time 5 -o /dev/null -w '%{http_code}' "$url" 2>/dev/null || true)"
        if [[ "$status" == 200 ]]; then
            echo "$label health is reachable."
            return 0
        fi
        sleep 2
    done

    echo "$label health failed: ${status:-none}" >&2
    return 1
}

require_env OPENAI_API_KEY
require_env OPENAI_BASE_URL
require_env OPENAI_MODEL

port="${GH_DOWNLOADER_PORT:-8080}"
bind="${GH_DOWNLOADER_BIND:-127.0.0.1}"
nginx_cache_dir="$(normalize_path "${GH_DOWNLOADER_NGINX_CACHE_DIR:-./.data/nginx-cache}")"
probe_host="$bind"
if [[ "$probe_host" == "0.0.0.0" || "$probe_host" == "::" || "$probe_host" == "[::]" ]]; then
    probe_host="127.0.0.1"
fi

export GH_DOWNLOADER_PORT="$port"
export GH_DOWNLOADER_BIND="$bind"
export GH_DOWNLOADER_NGINX_CACHE_DIR="$nginx_cache_dir"

mkdir -p "$nginx_cache_dir"

docker compose up -d --build
docker compose exec -T nginx nginx -t
docker compose exec -T nginx nginx -s reload
docker compose ps

probe_url "http://${probe_host}:${port}/healthz" "local"

if [[ -n "${GH_DOWNLOADER_PUBLIC_URL:-}" ]]; then
    public_url="${GH_DOWNLOADER_PUBLIC_URL%/}"
    probe_url "${public_url}/healthz" "public"
fi
