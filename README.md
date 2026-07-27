# Github Downloader

Github Downloader 是一个 GitHub Release 安装包下载入口：即使本地访问 GitHub 不稳定，也可以通过本站代理获取 release 文件。

只需打开 https://gh-downloader.com ，输入仓库地址，Github Downloader 会自动识别用户的操作系统和架构，选出最合适的安装包和安装步骤，比如 Windows 的 `.exe`/`.msi`、MacBook 的 Intel 或 Apple Silicon 版本、Linux 的 AppImage 或压缩包。

用户不用手动挑文件、查命令，也不用在 GitHub 下载失败或速度慢时反复重试。

## 工作方式

Github Downloader 会同步公开 release 元数据，并识别每个支持平台的安装包。分析结果会持久化保存，后续请求同一仓库时不需要重复分析。

下载由 NGINX 对外提供。Go 应用把稳定的 `/dl/...` 路径解析为 GitHub release asset，NGINX 缓存成功的下载响应，避免重复请求同一个文件时反复从 GitHub 下载。详见 [下载架构](docs/download-architecture.md)。

## 自托管

服务通过 Docker Compose 部署。Compose 会启动 NGINX、Go 应用和 Postgres。NGINX 监听容器内 `8080` 端口，宿主机绑定地址和端口由 `GH_DOWNLOADER_BIND`、`GH_DOWNLOADER_PORT` 配置，默认绑定地址为 `127.0.0.1`。

如果宿主机已有反向代理负责 HTTPS 终止，应把请求转发到 Compose 暴露的 NGINX 端口，不要直接转发到 Go 应用容器。

OpenAI 配置必须显式提供：

- `OPENAI_API_KEY`
- `OPENAI_BASE_URL`
- `OPENAI_MODEL`

`GH_DOWNLOADER_GITHUB_TOKEN` 为可选配置，用于提高公开仓库的 GitHub REST API 速率限制。

```bash
OPENAI_API_KEY=<key> \
OPENAI_BASE_URL=<base-url> \
OPENAI_MODEL=<model> \
GH_DOWNLOADER_GITHUB_TOKEN=<token> \
docker compose up -d --build
```

只有当 Compose NGINX 端口需要被其它机器直接访问时，才设置 `GH_DOWNLOADER_BIND=0.0.0.0`。

## CI

CI 会在 GitHub-hosted runner 上运行 `bash scripts/run_test.sh`。
