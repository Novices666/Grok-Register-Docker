# Grok-Register

Grok 免费号 **注册 → OAuth → CPA 可用 JSON** 二合一 CLI（Go）。

一条命令后台跑完，产物可直接导入 CPA / cliproxy 类网关。

```bash
grok start -t 10
grok status
grok logs -f
grok stop
grok upload    # 手动上传 CPA JSON 到 Management API
```

---

## 功能

- 临时邮箱 / 自建域名邮箱注册
- 注册成功后立刻 Device Flow OAuth
- 整备 `cli-chat-proxy` + grok-cli headers 的 CPA JSON
- 可选探活；可选自动上传到 CPA Management API
- 内置 Cloudflare 清障 compose（WARP + Privoxy + FlareSolverr）
- Turnstile：默认 **Playwright + CloakBrowser**（与原 Python 注册机同路径），可选 lite farm

---

## 系统要求

| 组件 | 用途 |
|------|------|
| Go 1.21+ | 仅编译时 |
| Python 3.10+ + venv | Turnstile Playwright mint |
| Docker | 清障栈（强烈推荐） |
| CloakBrowser Chromium | 过 CF Turnstile（推荐） |

---

## 快速安装（Linux / macOS）

### 1. 拉取代码

```bash
git clone https://github.com/Charles-0509/Grok-Register.git
cd Grok-Register
```

### 2. 编译安装 `grok`

```bash
# 确保 go 在 PATH（常见：export PATH=$PATH:/usr/local/go/bin）
make build
sudo make install
# → /usr/local/bin/grok
# → /usr/local/share/grok-reg/turnstile_mint.py
grok help
```

`sudo make install` 在已有 `bin/grok` 时**不会**再调 `go`（避免 root PATH 里没有 go）。

### 3. Turnstile 依赖（必须，否则拿不到 token）

```bash
sudo apt update
sudo apt install -y python3 python3-pip python3-venv

python3 -m venv /opt/cloakbrowser-venv
/opt/cloakbrowser-venv/bin/pip install -U pip
/opt/cloakbrowser-venv/bin/pip install -r scripts/requirements-turnstile.txt
/opt/cloakbrowser-venv/bin/python -m cloakbrowser install

# 让 grok 找到这个 Python（root 跑注册机时写进 root 环境）
echo 'export GROK_PYTHON=/opt/cloakbrowser-venv/bin/python' >> ~/.bashrc
export GROK_PYTHON=/opt/cloakbrowser-venv/bin/python
```

可选：

```bash
export GROK_TURNSTILE_SCRIPT=/opt/Grok-Register/scripts/turnstile_mint.py
# 或 make install 后默认：/usr/local/share/grok-reg/turnstile_mint.py
```

### 4. 清障栈（强烈推荐）

```bash
cd clearance
docker compose up -d
docker compose ps   # warp / privoxy / flaresolverr 应为 healthy
cd ..
```

### 5. 启动

```bash
grok start -t 5
grok status
grok logs -f
```

首次会询问邮箱模式，配置写入 `~/.grok/config.env`。

---

## 命令一览

| 命令 | 说明 |
|------|------|
| `grok start` | 后台启动，默认目标 10 |
| `grok start -t N` | 目标 N（1–10000）；**计数 = 探活成功写入 CPA 的数量** |
| `grok status` | 未运行 / 运行中 / 错误；进度、线程、当前步骤 |
| `grok logs` | 最近一次完整日志 |
| `grok logs -f` | 实时跟踪日志 |
| `grok stop` | 立即停止 |
| `grok upload` | 交互选择最近 10 次 run，上传其中 CPA JSON |

数据目录默认 `~/.grok/`（可用 `GROK_HOME` 覆盖）。

---

## 输出目录

每次 `start` 生成一个 run：

```text
~/.grok/
├── config.env
├── run.pid / run.lock / state.json
├── logs/run-yyyymmdd-HHMMSS.log
└── outputs/
    └── yyyymmdd-HHMMSS/
        ├── SSO/          # accounts.txt, auth-sessions.jsonl
        ├── CPA/          # 探活成功的 CPA JSON（可导入）
        └── discarded/    # 探活失败
```

---

## 配置（`~/.grok/config.env`）

模板见仓库内 `config.env.example`。常用项：

```env
EMAIL_MODE=tempmail

CLEARANCE_ENABLED=1
REGISTER_PROXY=http://127.0.0.1:40080
FLARESOLVERR_URL=http://127.0.0.1:8191
CLEARANCE_PROXY=http://privoxy:8118

TURNSTILE_PROVIDER=browser
# TURNSTILE_PROVIDER=lite
# LITE_SOLVER_URL=http://127.0.0.1:5072

HTTPS_PROXY=http://127.0.0.1:40080
HTTP_PROXY=http://127.0.0.1:40080
NO_PROXY=127.0.0.1,localhost

PROBE_ENABLED=1

# CPA 自动上传（可选）
CPA_UPLOAD_ENABLED=0
CPA_MANAGEMENT_BASE=http://localhost:8317/v0/management
CPA_MANAGEMENT_KEY=
CPA_UPLOAD_TIMEOUT_SEC=30
CPA_UPLOAD_RETRIES=2
CPA_UPLOAD_NAME_TEMPLATE={email}.json
```

### 环境变量（进程级）

| 变量 | 说明 |
|------|------|
| `GROK_HOME` | 数据根目录，默认 `~/.grok` |
| `GROK_PYTHON` | 跑 `turnstile_mint.py` 的 Python |
| `GROK_TURNSTILE_SCRIPT` | mint 脚本路径 |
| `CHROME_PATH` | 强制指定 Chromium 可执行文件 |

---

## 流水线

```text
清障预热 → S:Turnstile → P:邮箱+验证码 → C:注册拿 SSO
       → 立刻 OAuth (HTTP device verify/approve)
       → 整备 CPA JSON → 探活 → 写 CPA/
       → (可选) 异步上传 Management API
```

- **TARGET**：仅 `CPA/` 探活成功计数  
- **自动上传失败**不影响账号记为成功  
- **邮箱预创建**按 target 限流，避免 target=5 时狂开邮箱  

---

## Turnstile 说明

默认 `browser`：

1. 优先调用 `scripts/turnstile_mint.py`（**Playwright + CloakBrowser**，对齐原 Python 注册机）  
2. 脚本不可用时回退 chromedp（在 CF 下成功率通常更低）  

因此服务器上 **必须** 安装：

```bash
pip install -r scripts/requirements-turnstile.txt   # 在 venv 里
python -m cloakbrowser install
export GROK_PYTHON=/path/to/venv/bin/python
```

可选外接 YesCaptcha 形 farm：

```env
TURNSTILE_PROVIDER=lite
LITE_SOLVER_URL=http://127.0.0.1:5072
```

仓库**不内置** farm 镜像。

---

## CPA 上传

### 自动

`CPA_UPLOAD_ENABLED=1` 且配置了 `CPA_MANAGEMENT_KEY` 时，每个成功 CPA JSON 会异步：

- 优先 `multipart` 字段 `file` → `POST .../auth-files`  
- 失败时回退 raw JSON + `?name=`  
- Header：`Authorization: Bearer` + `X-Management-Key`  
- 日志**不打印**密钥  

### 手动

```bash
grok upload
# 列出最近 10 个 outputs/<run_id>/
# 输入 1 或 1,2,3 多选上传
```

---

## 自建邮箱

```env
EMAIL_MODE=custom
EMAIL_DOMAIN=example.com
EMAIL_API=http://127.0.0.1:8080
```

参考 `cloudflare/email-worker.js` 配置 Cloudflare Email Routing catch-all。

---

## 目录结构

```text
Grok-Register/
├── cmd/grok/                 # CLI 入口
├── internal/                 # 业务包
│   ├── clearance/            # FlareSolverr prewarm
│   ├── turnstile/            # Playwright bridge + chromedp fallback + lite
│   ├── register pipeline…    # S/P/C + OAuth + CPA
│   └── cpa/                  # 落盘 + Management 上传
├── scripts/
│   ├── turnstile_mint.py     # 与原项目同逻辑的 mint
│   └── requirements-turnstile.txt
├── clearance/                # docker compose 清障栈
├── cloudflare/email-worker.js
├── config.env.example
├── Makefile
└── README.md
```

---

## 常见问题

**`make build` / `sudo make install` 报 go not found**  
```bash
export PATH=$PATH:/usr/local/go/bin
make build
sudo make install          # 已有 bin/grok 时不再调用 go
# 或：sudo install -m 755 bin/grok /usr/local/bin/grok
```

**`turnstile timeout` / `iframes=0`**  
1. 确认 `GROK_PYTHON` 指向已装 playwright 的 venv  
2. `python -m cloakbrowser install` 已完成  
3. `clearance` 容器 healthy，`REGISTER_PROXY` 可用  
4. `grok logs -f` 中是否出现 `playwright mint: ...` 具体错误  

**邮箱建得特别多**  
新版本会按 target 限制 P/Q；请更新到最新代码并 `make build && make install`。  

**只想手动导入 CPA**  
看 `~/.grok/outputs/<run>/CPA/*.json`，或 `grok upload`。

---

## 开发

```bash
go test ./...
go build -o bin/grok ./cmd/grok
```

---

## License

MIT（与上游 grok-free-register 思路一致；本仓库为 Go 重制版。）
