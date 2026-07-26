#!/usr/bin/env bash
# Deploy gh-downloader with Docker Compose.
#
# Usage:
#   OPENAI_API_KEY=... OPENAI_BASE_URL=... OPENAI_MODEL=... bash scripts/deploy.sh

set -e

require_env() {
    local name="$1"
    if [[ -z "${!name:-}" ]]; then
        echo "$name is required" >&2
        exit 1
    fi
}

require_env OPENAI_API_KEY
require_env OPENAI_BASE_URL
require_env OPENAI_MODEL

port="${GH_DOWNLOADER_PORT:-8080}"
bind="${GH_DOWNLOADER_BIND:-127.0.0.1}"
nginx_cache_dir="${GH_DOWNLOADER_NGINX_CACHE_DIR:-./.data/nginx-cache}"

export GH_DOWNLOADER_PORT="$port"
export GH_DOWNLOADER_BIND="$bind"
export GH_DOWNLOADER_NGINX_CACHE_DIR="$nginx_cache_dir"

mkdir -p "$nginx_cache_dir"

docker compose up -d --build
docker compose exec -T nginx nginx -t
docker compose exec -T nginx nginx -s reload
docker compose ps
