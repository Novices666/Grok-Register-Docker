# Grok-Register-Docker

面向 Docker 部署的 Grok 注册与输出管理工具，提供 Web 控制台、CLI、代理清障链路以及可配置的 SSO / CPA 输出。

本仓库由 [Charles-0509/Grok-Register](https://github.com/Charles-0509/Grok-Register) 衍生，现由本仓库独立维护。上游更新不会自动合并，具体同步原则见 [MERGE_GUIDE.zh-CN.md](MERGE_GUIDE.zh-CN.md)。

> 请仅在遵守目标服务条款及所在地法律法规的前提下使用。账号、代理、邮箱和输出凭据均由使用者自行管理。

## 功能

- Docker Compose 一键构建和运行
- 带 Basic Auth 的 Web 管理控制台
- 在浏览器中启动、停止并查看注册进度和日志
- 在浏览器中编辑运行配置、检查 CPA Management 连接
- 浏览、下载或删除历史运行结果
- 内置 WARP、Privoxy 和 FlareSolverr 清障服务
- 保留 `grok` CLI，可用于脚本或终端运维
- SSO、grok2api SSO、CPA 三类输出可独立控制
- CPA JSON 可选自动上传至 Management API
- 数据通过宿主机目录持久化，重建容器不会丢失

## 快速开始

### 1. 环境要求

- Git
- Docker Engine 或 Docker Desktop
- Docker Compose v2
- 建议至少 2 核 CPU、4 GiB 内存

首次构建需要下载 Go、Playwright、CloakBrowser 及相关容器镜像，请确保 Docker 可以访问对应镜像源。

### 2. 获取并配置

```bash
git clone https://github.com/Novices666/Grok-Register-Docker.git
cd Grok-Register-Docker
cp .env.docker.example .env
```

打开 `.env`，至少修改 Web 管理密码：

```env
WEB_USERNAME=admin
WEB_PASSWORD=请替换为强密码
```

不要提交 `.env`。邮箱密钥、CPA Management Key 等敏感配置也只应保存在本地。

### 3. 启动

```bash
docker compose up -d --build
docker compose ps
```

默认访问地址：

```text
http://127.0.0.1:8090
```

使用 `.env` 中的 `WEB_USERNAME` 和 `WEB_PASSWORD` 登录。

Web 端口默认只绑定到 `127.0.0.1`。在远程服务器部署时，推荐使用 SSH 隧道访问：

```bash
ssh -L 8090:127.0.0.1:8090 user@server
```

然后在本机打开 `http://127.0.0.1:8090`。

### 4. 首次运行

进入 Web 控制台后：

1. 在“配置”中确认邮箱、代理和输出选项。
2. 根据需要填写 CPA Management 地址和密钥。
3. 保存配置。
4. 返回运行页，设置目标数量和并发线程。
5. 启动任务并观察实时状态和日志。

低配机器建议并发线程设为 `1`。目标数量范围为 `1-10000`，并发范围为 `1-8`。

## Web 控制台

Web 控制台包含以下能力：

| 页面/功能 | 说明 |
|---|---|
| 运行状态 | 查看当前阶段、成功数量、目标和运行状态 |
| 启动与停止 | 设置目标数量、并发线程并控制任务 |
| 日志 | 查看最近一次运行日志 |
| 输出配置 | 控制 SSO、grok2api SSO 和 CPA 链路 |
| 邮箱配置 | 配置临时邮箱、testmail 或自建邮箱服务 |
| CPA 上传 | 配置并检查 Management API |
| 历史记录 | 查看每次运行的 SSO、CPA、丢弃文件和日志 |
| 下载 | 按全部、SSO、CPA、丢弃文件或日志下载 |
| 删除 | 仅隐藏历史记录，或同时删除记录和实际文件 |

Web 服务使用 HTTP Basic Auth。它不提供 HTTPS，且接口可以启动任务、修改配置和下载凭据，因此不要直接暴露到公网。公网场景应置于带 HTTPS 和额外访问控制的反向代理之后。

## 常用运维命令

```bash
# 查看服务状态
docker compose ps

# 查看主服务日志
docker compose logs -f grok-register

# 查看全部服务日志
docker compose logs -f

# 重启主服务
docker compose restart grok-register

# 停止并删除容器，保留 ./data/grok
docker compose down

# 重新构建并启动
docker compose up -d --build --force-recreate
```

在容器内使用 CLI：

```bash
docker compose exec grok-register grok status
docker compose exec grok-register grok logs
docker compose exec grok-register grok stop
docker compose exec grok-register grok upload
docker compose exec grok-register grok help
```

容器默认由 Web 服务保持运行，通常直接在 Web 控制台操作即可。

## 输出和数据

宿主机数据保存在：

```text
./data/grok/
├── config.env
├── config.env.example
├── run.pid
├── run.lock
├── state.json
├── logs/
└── outputs/
    └── <run_id>/
        ├── SSO/
        │   ├── accounts.txt
        │   └── auth-sessions.jsonl
        ├── grok2api/
        │   └── tokens.txt
        ├── CPA/
        └── discarded/
```

| 配置 | 行为 |
|---|---|
| `OUTPUT_SSO_ENABLED=1` | 输出 `accounts.txt` 和 `auth-sessions.jsonl` |
| `OUTPUT_GROK2API_SSO_ENABLED=1` | 在 SSO 输出开启时额外输出 `grok2api/tokens.txt`（上游布局） |
| `OUTPUT_CPA_ENABLED=0` | 注册取得 SSO 后计为成功，跳过 OAuth、CPA 探活和 CPA JSON |
| `OUTPUT_CPA_ENABLED=1` | 继续执行 OAuth、CPA 探活并输出 CPA JSON |
| `CPA_UPLOAD_ENABLED=1` | CPA 输出开启时，将成功文件上传到 Management API |

`OUTPUT_GROK2API_SSO_ENABLED` 依赖 `OUTPUT_SSO_ENABLED`。

`CPA_UPLOAD_ENABLED` 依赖 `OUTPUT_CPA_ENABLED`。

Web 控制台会读取同一数据目录，因此容器重建后仍可查看以前的结果。输出文件包含敏感凭据，请限制 `data/` 的访问权限并妥善备份。

## 主要配置

Docker 启动配置来自根目录 `.env`。容器首次启动时会在 `./data/grok/config.env` 创建运行配置，之后可通过 Web 控制台修改。

### Web

| 变量 | 默认值 | 说明 |
|---|---|---|
| `WEB_ENABLED` | `1` | 是否启动 Web 控制台 |
| `WEB_PORT` | `8090` | 宿主机监听端口 |
| `WEB_USERNAME` | `admin` | Web 用户名 |
| `WEB_PASSWORD` | 无 | 必填，Web 管理密码 |

### 邮箱

| 模式 | 所需配置 |
|---|---|
| `EMAIL_MODE=tempmail` | 无额外密钥，使用公共临时邮箱 |
| `EMAIL_MODE=testmail` | `TESTMAIL_API_KEY`、`TESTMAIL_NAMESPACE`，可选 `TESTMAIL_DOMAIN` |
| `EMAIL_MODE=custom` | `EMAIL_DOMAIN`、`EMAIL_API` |

### 代理与清障

Compose 默认流量路径为：

```text
grok-register -> privoxy -> WARP
                 |
                 +-> FlareSolverr
```

| 变量 | 默认值 | 说明 |
|---|---|---|
| `CLEARANCE_ENABLED` | `1` | 启用清障逻辑 |
| `CLEARANCE_MODE` | `auto` | `auto`、`always` 或 `never` |
| `REGISTER_PROXY` | `http://privoxy:8118` | 注册请求代理 |
| `FLARESOLVERR_URL` | `http://flaresolverr:8191` | FlareSolverr 服务地址 |
| `TURNSTILE_PROVIDER` | `browser` | Turnstile 提供方式 |
| `TURNSTILE_MODE` | `offscreen` | 浏览器运行模式 |
| `WARP_LICENSE_KEY` | 空 | 可选 WARP+ License |

根目录一键 Docker 栈**仅向宿主机映射 Web 管理端口**（清障组件只在 compose 内网互通）：

| 服务 | 宿主机地址 | 说明 |
|---|---|---|
| Web 控制台 | `127.0.0.1:8090` | 唯一需要暴露的端口，可用 `WEB_PORT` 修改 |
| WARP / Privoxy / FlareSolverr | 不映射到宿主机 | 容器内通过 `privoxy:8118`、`flaresolverr:8191` 访问 |

若本机原生运行 `grok`、需要单独拉起清障栈，请使用 [clearance/docker-compose.yml](clearance/docker-compose.yml)，那时才会在本机映射 `40000/40080/8191`。

### CPA Management

```env
OUTPUT_CPA_ENABLED=1
CPA_UPLOAD_ENABLED=1
CPA_MANAGEMENT_BASE=http://host.docker.internal:8317/v0/management
CPA_MANAGEMENT_KEY=your-key
```

容器访问宿主机服务时使用 `host.docker.internal`。Compose 已为 Linux 添加对应的 `host-gateway` 映射。

完整 Docker 配置模板见 [.env.docker.example](.env.docker.example)，完整运行配置模板见 [config.env.example](config.env.example)。

## 更新

更新前建议备份 `.env` 和 `data/grok`：

```bash
git pull --ff-only
docker compose up -d --build --force-recreate
docker compose ps
```

查看启动日志：

```bash
docker compose logs --tail 200 grok-register
```

不要使用会删除 `data/grok` 的清理命令，除非已经确认不再需要历史输出和配置。

## 原生 CLI

不使用根目录 Docker Compose 时，也可以在 Linux 或 macOS 上直接构建 CLI：

```bash
go build -o bin/grok ./cmd/grok
./bin/grok help
```

常用命令：

```bash
grok start
grok start -t 10 --thread 2
grok status
grok logs -f
grok stop
grok config
grok upload
```

原生运行还需要 Python、Playwright/CloakBrowser，以及可用的代理或清障服务。相关安装脚本仍保留在 `scripts/install.sh`，使用本仓库版本时应明确指定仓库：

Linux：

```bash
sudo bash scripts/install.sh \
  --repo https://github.com/Novices666/Grok-Register-Docker.git \
  --branch main
```

macOS：

```bash
bash scripts/install.sh \
  --repo https://github.com/Novices666/Grok-Register-Docker.git \
  --branch main
```

本仓库的主要维护路径是 Docker Compose；原生 CLI 更适合开发、调试和已有环境。

## 开发与验证

```bash
go test ./...
go build -o bin/grok ./cmd/grok
go build -o bin/grok-web ./cmd/web
docker compose config --quiet
docker compose build
```

目录结构：

```text
Grok-Register-Docker/
├── cmd/grok/                 # CLI
├── cmd/web/                  # Web 服务入口
├── internal/                 # 注册、配置、输出及 Web 逻辑
├── internal/webui/templates/ # Web 页面
├── scripts/                  # 安装和 Turnstile 辅助脚本
├── clearance/                # 独立清障配置
├── docker/                   # 主容器入口
├── Dockerfile
├── docker-compose.yml
├── .env.docker.example
├── AGENTS.md                 # AI 维护与上游同步规则
└── MERGE_GUIDE.zh-CN.md
```

## 项目关系与维护策略

本仓库独立维护 Docker 部署、Web 管理、输出控制和相关兼容性功能。原始上游：

- [Charles-0509/Grok-Register](https://github.com/Charles-0509/Grok-Register)

上游的新提交只作为更新来源进行审查，不直接覆盖本仓库。协议、邮箱、OAuth、Turnstile 等核心更新会按需移植，同时保留本仓库的 Docker、WebUI 和输出行为。详细规则和当前同步基准见 [MERGE_GUIDE.zh-CN.md](MERGE_GUIDE.zh-CN.md)。

## License

本项目沿用上游项目的授权约定。使用、修改或分发前，请同时确认本仓库及上游项目的许可与第三方依赖条款。
