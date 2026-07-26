# Github Downloader

Github Downloader finds the best release asset for macOS, Windows, or Linux
from public GitHub repositories and proxies downloads through a cache-friendly
entry point.

Users open a hosted instance, enter a repository URL, and download the
recommended package.

## How It Works

Github Downloader syncs public release metadata and identifies the installer
asset for each supported platform. The analysis result is stored, so later
requests for the same repository do not need to run analysis again.

Downloads are served through NGINX. The Go application resolves stable `/dl/...`
paths to GitHub release assets, and NGINX caches successful download responses
so repeated requests for the same asset do not repeatedly download the file from
GitHub. See [Download Architecture](docs/download-architecture.md).

## Self Hosting

The service is deployed with Docker Compose. Compose runs NGINX, the Go
application, and Postgres. NGINX listens on container port `8080`, and the host
bind address and port are configured with `GH_DOWNLOADER_BIND` and
`GH_DOWNLOADER_PORT`. The default bind address is `127.0.0.1`.

If a host-level reverse proxy terminates HTTPS, it should forward requests to
the Compose NGINX host port. It should not forward traffic directly to the Go
application container.

OpenAI configuration must be provided explicitly:

- `OPENAI_API_KEY`
- `OPENAI_BASE_URL`
- `OPENAI_MODEL`

`GH_DOWNLOADER_GITHUB_TOKEN` is optional. It increases the GitHub REST API rate
limit for public repositories.

```bash
OPENAI_API_KEY=<key> \
OPENAI_BASE_URL=<base-url> \
OPENAI_MODEL=<model> \
GH_DOWNLOADER_GITHUB_TOKEN=<token> \
docker compose up -d --build
```

Set `GH_DOWNLOADER_BIND=0.0.0.0` only when the Compose NGINX port should be
reachable directly from other machines.

## CI

CI runs `bash scripts/run_test.sh` on a GitHub-hosted runner.
