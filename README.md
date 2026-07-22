# Grok-Register

Grok 免费号 **注册 → OAuth → CPA 可用 JSON** 二合一 CLI（Go）。

一条命令后台跑完，产物可直接导入 CPA / cliproxy 类网关。

```bash
grok start                 # 交互：数量 + 线程(1–8)
grok start -t 10 --thread 2
grok status
grok logs -f
grok stop
grok config                # 编辑 ~/.grok/config.env
grok upload                # 手动上传 CPA JSON 到 Management API
```

---

## 近期特性

| 特性 | 说明 |
|------|------|
| **testmail** | `EMAIL_MODE=testmail`，GitHub Student Pack 等；`TESTMAIL_API_KEY` / `NAMESPACE` / `DOMAIN` |
| **Turnstile 常驻池** | 默认 `turnstile_pool.py` 多浏览器复用 → 回退 one-shot mint → chromedp |
| **全局座位上限** | `done + reserved ≤ target`，避免多线程超开邮箱/注册 |
| **交互 `start` / `config`** | 数量与线程**不写** `config.env`；`grok config` 打开配置并刷新 example |
| **CPA 宿主机路径** | `CPA_MANAGEMENT_BASE=http://127.0.0.1:8317/v0/management`；自动改写 docker 主机名 |
| **一键安装** | `scripts/install.sh`：可改命令名、安装目录、数据目录 |

---

## 一键安装（推荐，Debian / Ubuntu）

需要 **root / sudo**。会安装系统库、Go、Docker、源码、CLI、Playwright/CloakBrowser、清障栈，并写入默认 `config.env`。

### 默认一行

```bash
curl -fsSL https://raw.githubusercontent.com/Charles-0509/Grok-Register/main/scripts/install.sh | sudo bash
```

默认结果：

| 项 | 路径 / 值 |
|----|-----------|
| 命令 | `grok` → `/usr/local/bin/grok` |
| 源码 | `/opt/Grok-Register`（并软链 `/opt/Grok-Reg`） |
| 数据 | `/root/.grok`（`GROK_HOME`） |
| Python | `/opt/cloakbrowser-venv/bin/python` |
| mint 脚本 | `/usr/local/share/grok-reg/turnstile_{mint,pool}.py` |
| 清障 | `clearance/` compose（WARP / Privoxy / FlareSolverr） |

### 自定义命令名 / 目录

避免与其它 `grok` CLI 冲突，或改数据盘：

```bash
# 命令改名为 grok-reg
curl -fsSL https://raw.githubusercontent.com/Charles-0509/Grok-Register/main/scripts/install.sh | \
  sudo bash -s -- --command grok-reg

# 自定义安装目录 + 数据目录
curl -fsSL https://raw.githubusercontent.com/Charles-0509/Grok-Register/main/scripts/install.sh | \
  sudo bash -s -- \
    --command grok \
    --install-dir /data/Grok-Register \
    --home /data/grok-data

# 环境变量写法（等价）
curl -fsSL ... | sudo COMMAND_NAME=grok-reg INSTALL_DIR=/opt/Grok-Register GROK_HOME=/root/.grok bash
```

### 常用选项

| 选项 / 环境变量 | 默认 | 说明 |
|-----------------|------|------|
| `--command` / `COMMAND_NAME` | `grok` | CLI 命令名 |
| `--install-dir` / `INSTALL_DIR` | `/opt/Grok-Register` | 源码目录 |
| `--home` / `GROK_HOME` | `/root/.grok` | 配置与 outputs |
| `--bin-dir` / `BIN_DIR` | `/usr/local/bin` | 二进制目录 |
| `--share-dir` / `SHARE_DIR` | `/usr/local/share/grok-reg` | mint 脚本 |
| `--venv-dir` / `VENV_DIR` | `/opt/cloakbrowser-venv` | Python venv |
| `--repo` / `REPO_URL` | 本仓库 | 可改镜像源 |
| `--branch` / `BRANCH` | `main` | 分支 |
| `--skip-docker` | off | 不装 Docker |
| `--skip-clearance` | off | 不起清障栈 |
| `--skip-browser` | off | 不装 Playwright（Turnstile 不可用） |
| `--skip-go` | off | 不自动装 Go |
| `--no-start-clearance` | off | 装 Docker 但不 `compose up` |

本地已 clone 时：

```bash
cd /path/to/Grok-Register
sudo bash scripts/install.sh --command grok-reg --home /root/.grok
# 或
sudo make install APP=grok-reg
```

### 装完立刻跑

```bash
# 新 shell 会自动 source /etc/profile.d/grok-register.sh
# 当前 shell 可手动：
export GROK_HOME=/root/.grok
export GROK_PYTHON=/opt/cloakbrowser-venv/bin/python

grok start                 # 或你自定义的命令名
grok status
grok logs -f
```

若 clearance 未 healthy：

```bash
cd /opt/Grok-Register/clearance && docker compose up -d && docker compose ps
```

---

## 系统要求

| 组件 | 用途 | 不装会怎样 |
|------|------|------------|
| Go 1.21+ | 仅编译 CLI | 无法 build |
| Python 3.10+ + venv | Turnstile Playwright mint | 拿不到 token |
| Playwright + CloakBrowser | 无头过 CF Turnstile | `timeout` / `iframes=0` |
| Docker | 清障栈（强烈推荐） | 注册/邮箱/CF 更容易挂 |
| CPA Management（可选） | `grok upload` / 自动上传 | 本地仍有 `CPA/*.json` |

---

## 完整部署（手动分步）

> 一键脚本失败或需精细控制时使用。目标：系统依赖 → Go → Docker → 编译 → 无头浏览器 → 清障 → 配置 → 跑注册。

### 0. 系统依赖

```bash
sudo apt update
sudo apt install -y \
  git curl ca-certificates gnupg lsb-release \
  build-essential make \
  python3 python3-pip python3-venv \
  libnss3 libnspr4 libatk1.0-0 libatk-bridge2.0-0 libcups2 \
  libdrm2 libxkbcommon0 libxcomposite1 libxdamage1 libxfixes3 \
  libxrandr2 libgbm1 libasound2t64 libpango-1.0-0 libcairo2 \
  fonts-liberation fonts-noto-cjk
```

> 若 `libasound2t64` 不存在，改成 `libasound2`。

### 1. 安装 Go（仅编译需要，建议 1.21+）

```bash
cd /tmp
curl -fsSL -o go.tgz https://go.dev/dl/go1.24.4.linux-amd64.tar.gz
sudo rm -rf /usr/local/go
sudo tar -C /usr/local -xzf go.tgz
echo 'export PATH=$PATH:/usr/local/go/bin' | sudo tee /etc/profile.d/go.sh
export PATH=$PATH:/usr/local/go/bin
go version
```

### 2. 安装 Docker（清障栈用）

```bash
curl -fsSL https://get.docker.com | sudo sh
sudo systemctl enable --now docker
docker compose version || sudo apt install -y docker-compose-plugin
```

### 3. 拉取并编译安装

```bash
sudo mkdir -p /opt
cd /opt
sudo git clone https://github.com/Charles-0509/Grok-Register.git
cd /opt/Grok-Register

export PATH=$PATH:/usr/local/go/bin
make build
sudo make install
# 自定义命令名：
# make build APP=grok-reg && sudo make install APP=grok-reg

grok help
```

`sudo make install` 在已有 `bin/grok` 时**不会**再调 `go`（避免 root PATH 里没有 go）。

### 4. 无头浏览器：Playwright + CloakBrowser（**必做**）

```bash
sudo python3 -m venv /opt/cloakbrowser-venv
sudo /opt/cloakbrowser-venv/bin/pip install -U pip
sudo /opt/cloakbrowser-venv/bin/pip install -r /opt/Grok-Register/scripts/requirements-turnstile.txt
sudo /opt/cloakbrowser-venv/bin/python -m cloakbrowser install

echo 'export GROK_PYTHON=/opt/cloakbrowser-venv/bin/python' | sudo tee -a /root/.bashrc
echo 'export CLOAKBROWSER_SUPPRESS_FONT_WARNING=1' | sudo tee -a /root/.bashrc
export GROK_PYTHON=/opt/cloakbrowser-venv/bin/python
export CLOAKBROWSER_SUPPRESS_FONT_WARNING=1
```

**冒烟测试**（清障栈起来后）：

```bash
export GROK_PYTHON=/opt/cloakbrowser-venv/bin/python
$GROK_PYTHON /usr/local/share/grok-reg/turnstile_mint.py \
  --site-key 0x4AAAAAAAhr9JGVDZbrZOo0 \
  --url https://accounts.x.ai/sign-up \
  --proxy http://127.0.0.1:40080 \
  --timeout 70
echo exit:$?
```

### 5. 清障栈

```bash
cd /opt/Grok-Register/clearance
sudo docker compose up -d
sudo docker compose ps
```

| 端口 | 服务 |
|------|------|
| `127.0.0.1:40000` | WARP SOCKS5 |
| `127.0.0.1:40080` | Privoxy HTTP |
| `127.0.0.1:8191` | FlareSolverr |

### 6. 配置 `~/.grok/config.env`

```bash
sudo mkdir -p /root/.grok
# 也可：grok config（首次会生成 example + 可编辑）
sudo tee /root/.grok/config.env >/dev/null <<'EOF'
EMAIL_MODE=tempmail

CLEARANCE_ENABLED=1
REGISTER_PROXY=http://127.0.0.1:40080
FLARESOLVERR_URL=http://127.0.0.1:8191
CLEARANCE_PROXY=http://privoxy:8118
CLEARANCE_URLS=https://accounts.x.ai,https://x.ai,https://status.x.ai,https://console.x.ai,https://auth.x.ai

TURNSTILE_PROVIDER=browser

PROTOCOL_HTTP=1
HTTP_POOL_SIZE=8
TEMPMAIL_LOL_RETRIES=30
TEMPMAIL_LOL_MIN_INTERVAL_MS=1500

HTTPS_PROXY=http://127.0.0.1:40080
HTTP_PROXY=http://127.0.0.1:40080
NO_PROXY=127.0.0.1,localhost

PROBE_ENABLED=1

CPA_UPLOAD_ENABLED=0
CPA_MANAGEMENT_BASE=http://127.0.0.1:8317/v0/management
CPA_MANAGEMENT_KEY=
CPA_UPLOAD_TIMEOUT_SEC=30
CPA_UPLOAD_RETRIES=2
CPA_UPLOAD_NAME_TEMPLATE={email}.json
EOF
```

邮箱模式：

```env
# 1) 公共临时邮箱（默认，无需 token）
EMAIL_MODE=tempmail

# 2) testmail.app
# EMAIL_MODE=testmail
# TESTMAIL_API_KEY=你的_apikey
# TESTMAIL_NAMESPACE=你的_namespace
# TESTMAIL_DOMAIN=inbox.testmail.app

# 3) 自建域名
# EMAIL_MODE=custom
# EMAIL_DOMAIN=example.com
# EMAIL_API=http://127.0.0.1:8080
```

`tempmail` = tempmail.lol + mail.tm 系 fallback，**无需私人 Token**。  
`testmail` 密钥只写本地 `config.env`，勿提交仓库。

### 7. 启动与运维

```bash
export GROK_PYTHON=/opt/cloakbrowser-venv/bin/python
export CLOAKBROWSER_SUPPRESS_FONT_WARNING=1

grok start
grok start -t 10 --thread 3
grok status
grok logs -f
grok stop
grok config
grok upload
```

**数据目录**（`GROK_HOME`，默认 `~/.grok`）：

```text
~/.grok/
├── config.env / config.env.example
├── run.pid / run.lock / state.json
├── logs/run-yyyymmdd-HHMMSS.log
└── outputs/<run_id>/
    ├── SSO/
    ├── CPA/          # 探活成功，可导入
    └── discarded/
```

### 8. 更新

```bash
cd /opt/Grok-Register
sudo git pull
export PATH=$PATH:/usr/local/go/bin
make build && sudo make install
# 或重跑一键（会 reset 到 origin/branch，保留已有 config.env）：
# curl -fsSL .../install.sh | sudo bash
```

### macOS 备注

- Go / Docker Desktop 自行安装  
- Turnstile：venv + `requirements-turnstile.txt` + `python -m cloakbrowser install`  
- 清障：`cd clearance && docker compose up -d`  
- 一键脚本目前面向 Debian/Ubuntu

---

## 命令一览

| 命令 | 说明 |
|------|------|
| `grok start` | 交互：注册数量 + 并发线程(1–8) |
| `grok start -t N --thread M` | 目标 N（1–10000）；线程 M（1–8）；**计数 = CPA 探活成功数** |
| `grok status` | 进度、线程、当前步骤 |
| `grok logs` / `logs -f` | 最近日志 / 跟踪 |
| `grok stop` | 立即停止 |
| `grok config` | 打开 `config.env`，刷新 `config.env.example` |
| `grok upload` | 选最近 run，上传 CPA JSON |

---

## 配置补充

| 变量 | 说明 |
|------|------|
| `GROK_HOME` | 数据根，默认 `~/.grok` |
| `GROK_PYTHON` | mint/pool 用的 Python |
| `GROK_TURNSTILE_SCRIPT` | one-shot mint 路径 |
| `GROK_TURNSTILE_POOL_SCRIPT` | 常驻池路径 |
| `CHROME_PATH` | 强制 Chromium |
| `CLOAKBROWSER_SUPPRESS_FONT_WARNING` | 抑制 Linux 字体提示 |
| `EDITOR` | `grok config` 编辑器 |

完整模板：`~/.grok/config.env.example`（每次 start/config 同步）。

---

## 流水线

```text
清障预热 → S:Turnstile → P:邮箱+验证码 → C:注册拿 SSO
       → OAuth → 整备 CPA JSON → 探活 → 写 CPA/
       → (可选) 异步上传 Management API
```

- **TARGET**：仅 `CPA/` 探活成功数  
- **座位**：`done + reserved ≤ target`（全局，非每线程）  
- 自动上传失败**不**影响记成功  

---

## Turnstile

默认 `browser`：

1. 常驻池 `turnstile_pool.py`（`TURNSTILE_WORKERS`，约 2）  
2. 回退 one-shot `turnstile_mint.py`  
3. 再回退 chromedp  

默认**不**注入 FlareSolverr cookie/UA（除非 `GROK_TURNSTILE_INJECT_CLEARANCE=1`）。

可选 lite farm：

```env
TURNSTILE_PROVIDER=lite
LITE_SOLVER_URL=http://127.0.0.1:5072
```

### 代理：WARP vs HTTP 池

| | WARP + Privoxy（默认） | HTTP 代理池 |
|--|------------------------|-------------|
| 成本 | 低 | 按量 |
| 适合 | 个人小批量 | 冲量 / 多 IP |
| 配置 | 本机 compose | 池 + 轮换 |

---

## CPA 上传

```env
CPA_UPLOAD_ENABLED=1
CPA_MANAGEMENT_BASE=http://127.0.0.1:8317/v0/management
CPA_MANAGEMENT_KEY=...
```

- 宿主机跑 `grok` 必须用 `127.0.0.1`，不要写 `cli-proxy-api`  
- 新版本会自动改写 docker 主机名并补 `/v0/management`  
- 手动：`grok upload`

---

## 目录结构

```text
Grok-Register/
├── cmd/grok/
├── internal/
├── scripts/
│   ├── install.sh            # 一键部署
│   ├── turnstile_mint.py
│   ├── turnstile_pool.py
│   └── requirements-turnstile.txt
├── clearance/                # docker compose 清障
├── cloudflare/email-worker.js
├── Makefile                  # APP= 可改命令名
└── README.md
```

---

## 常见问题

**一键装完命令找不到**

```bash
export PATH=$PATH:/usr/local/bin
# 或重新登录 shell；见 /etc/profile.d/grok-register.sh
```

**`make build` go not found**

```bash
export PATH=$PATH:/usr/local/go/bin
make build && sudo make install
```

**`turnstile timeout` / `iframes=0`**

1. `GROK_PYTHON` 指向已装 playwright 的 venv  
2. `python -m cloakbrowser install` 已完成  
3. clearance healthy，`REGISTER_PROXY` 可用  

**`lookup cli-proxy-api: no such host`**

```env
CPA_MANAGEMENT_BASE=http://127.0.0.1:8317/v0/management
```

**邮箱建得特别多**

请更新到含全局 `reserved` 座位上限的版本。

---

## 开发

```bash
go test ./...
go build -o bin/grok ./cmd/grok
bash -n scripts/install.sh
```

---

## License

MIT（与上游 grok-free-register 思路一致；本仓库为 Go 重制版。）

---

## 友链

- [LinuxDo · Charles0509](https://linux.do/u/charles0509)
