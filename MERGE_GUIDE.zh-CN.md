# 上游同步与本仓库扩展说明

本仓库以 `Charles-0509/Grok-Register` 为基础，在其上增加 Docker/Web 管理和输出链路控制。

本仓库现已独立维护。AI 执行更新前必须先阅读根目录 `AGENTS.md`，不得直接合并或覆盖上游分支。

当前上游审查状态：

| 项目 | 内容 |
|---|---|
| 上游仓库 | `https://github.com/Charles-0509/Grok-Register` |
| 上次已审查上游提交 | `862a434` |
| 本仓库最初基于的上游提交 | `a8d87bd` |
| 提交时间 | `2026-07-24 12:02:28 +0800` |

“上次已审查上游提交”表示已经检查到该提交，不表示审查范围内的所有提交都已采用。选择性移植时，必须在同步记录中单独列出实际采用和跳过的内容。

## 下次合并上游的原则

以目标仓库当前内容为基底，只合并上游确定更新的核心逻辑，再保留本仓库扩展。

| 类型 | 处理方式 |
|---|---|
| 上游协议、清障、Turnstile、邮箱、OAuth 内部实现 | 通常可以采用上游更新 |
| Docker/Web 管理 | 保留本仓库扩展 |
| 输出控制 | 保留本仓库扩展 |
| `data/`、`bin/`、`.env`、`work/` | 运行或临时文件，不参与合并 |

## 本仓库扩展清单

| 功能 | 关键位置 |
|---|---|
| Docker 一键服务 | `Dockerfile`、`docker-compose.yml`、`docker/entrypoint.sh`、`.env.docker.example` |
| Web 管理端 | `cmd/web/main.go`、`internal/webui/server.go` |
| SSO 输出开关 | `internal/config/config.go`、`internal/pipeline/pipeline.go`，环境变量 `OUTPUT_SSO_ENABLED` |
| grok2api SSO 文件 | `internal/cpa/cpa.go`、`internal/pipeline/pipeline.go`、`internal/home/home.go`，环境变量 `OUTPUT_GROK2API_SSO_ENABLED`，输出 `grok2api/tokens.txt`（采用上游布局，保留开关） |
| CPA 输出开关 | `internal/config/config.go`、`internal/pipeline/pipeline.go`，环境变量 `OUTPUT_CPA_ENABLED` |
| CPA 上传前置 | `internal/pipeline/pipeline.go`，上传实际生效条件为 `OUTPUT_CPA_ENABLED=1` 且 `CPA_UPLOAD_ENABLED=1` |
| 目标为 1 时的 Turnstile 等待修复 | `internal/pipeline/pipeline.go` 中的 `turnstileMintNeed` |
| Web 配置联动显示 | `internal/webui/server.go` |
| 配置模板 | `config.env.example`、`internal/config/example.env`、`scripts/install.sh` |

## 输出语义

默认三开关均为 `1`，行为与上游完整流水线一致。语义冲突时以上游为准；下列为显式扩展。

| 配置 | 结果 |
|---|---|
| `OUTPUT_SSO_ENABLED=1`（默认） | 输出 `accounts.txt` 和 `auth-sessions.jsonl` |
| `OUTPUT_SSO_ENABLED=0` | 不写 SSO 文件；若 CPA=1 仍走 OAuth/CPA |
| `OUTPUT_GROK2API_SSO_ENABLED=1`（默认） | 在 SSO 输出开启时，额外输出 `grok2api/tokens.txt` |
| `OUTPUT_CPA_ENABLED=1`（默认） | 与上游一致：OAuth、探活、CPA JSON；目标按 CPA 就绪计 |
| `OUTPUT_CPA_ENABLED=0` | 本仓扩展：拿到 SSO 即计完成，跳过 OAuth/CPA/上传 |
| `CPA_UPLOAD_ENABLED=1` | 仅当 `OUTPUT_CPA_ENABLED=1` 时自动上传 |
| `TURNSTILE_WORKERS` | 仅 Web/Docker 启动传 `-j` 的便利默认；`config.Load` 忽略（上游） |

## 合并后验证

```powershell
go test ./internal/config ./internal/oauth ./internal/pipeline ./internal/webui ./internal/cpa ./internal/inventory ./internal/protocol ./cmd/web
```

Linux 容器构建：

```powershell
$env:GOOS='linux'
$env:GOARCH='amd64'
go build -o bin/grok-linux ./cmd/grok
go build -o bin/grok-web-linux ./cmd/web
```

Docker 配置检查：

```powershell
docker compose -f docker-compose.yml config --quiet
```

## 更新容器

推荐：

```powershell
docker compose up -d --build --force-recreate
```

快速热更新：

```powershell
docker compose up -d
docker cp bin/grok-linux grok-register:/usr/local/bin/grok
docker cp bin/grok-web-linux grok-register:/usr/local/bin/grok-web
docker restart grok-register
```

## 同步记录


### 2026-07-24 上游同步（reoauth 重登 862a434）

| 项目 | 内容 |
|---|---|
| 同步分支 | `sync/upstream-20260724b` |
| 审查范围 | `2a98bf9..862a434` |
| 新的审查截止 | `862a434` |
| 实际移植 | `4d3349b` 新增 `grok reoauth`：解析 inspection/CPA/accounts/auth-sessions，优先 refresh_token、回退 device SSO，写出新 CPA；`e3d9931` 配置 CPA 上传时自动入库，`--upload`/`--no-upload` 覆盖；`862a434` 解析 inspection JSON 时剥离 UTF-8 BOM；`oauth.Client.Refresh` |
| 明确跳过 | 无（3 个提交均为 reoauth 核心功能，与 Docker/WebUI/`OUTPUT_*` 无冲突） |
| 兼容调整 | `cmd/grok/main.go` cherry-pick 冲突仅 help 文案：保留本仓「按当前输出配置计数」与默认节奏顺序，采用上游 reoauth 说明；README CLI 示例补充 `reoauth`；reoauth 上传仍跟 `CPA_UPLOAD_*`（与注册流水线 `OUTPUT_CPA_ENABLED` 约束独立，CLI 用途即 CPA 重登/入库） |
| 验证结果 | `go test` config/oauth/pipeline/webui/cpa/inventory/protocol/reoauth/clearance/logx/turnstile/cmd/web 通过；`GOOS=linux go build` grok/grok-web 通过；`docker compose --env-file .env.docker.example config --quiet` 通过 |

### 2026-07-24 上游同步（OAuth 严格确认 + 清障预热 2a98bf9）

| 项目 | 内容 |
|---|---|
| 同步分支 | `sync/upstream-20260724` |
| 审查范围 | `3a46f26..2a98bf9` |
| 新的审查截止 | `2a98bf9` |
| 实际移植 | `8f378de`/`3b4cc62`/`3c3de17`/`0deed4f` OAuth：严格 device approve、consent 表单解析、SSO reject 检测、Confirm 不附 CF clearance cookie、invalid_grant 重试；`1fd64ba`/`af6f1a1`/`2a98bf9` 清障：`:40080` 未监听时先起栈、默认 CLEARANCE_PROXY=privoxy、等 FS/WARP 就绪再预热、accounts/auth 优先与重试 |
| 明确跳过 | 无（7 个提交均为 OAuth/清障核心修复，与本仓扩展兼容） |
| 兼容调整 | `pipeline.go` 手工合并：保留 `OUTPUT_*`、`turnstileMintNeed`、`shouldRunCPAFlow`；SSO settle 2s 仅在走 OAuth/CPA 路径时执行；`oauth`/`clearance`/`prewarm.sh` 整文件采用上游 |
| 验证结果 | `go test` config/oauth/pipeline/webui/cpa/inventory/protocol/clearance/cmd/web/logx/turnstile 通过；`GOOS=linux go build` grok/grok-web 通过；`docker compose --env-file .env.docker.example config --quiet` 通过 |

### 2026-07-23 上游同步（xauth 依赖 3a46f26）

| 项目 | 内容 |
|---|---|
| 同步分支 | `sync/upstream-20260723c` |
| 审查范围 | `7dc93c0..3a46f26` |
| 新的审查截止 | `3a46f26` |
| 实际移植 | `3a46f26` install：精简 Debian 上与 `xvfb` 一并安装 `xauth`、`x11-xserver-utils`，避免 `xvfb-run` 报 `xauth command not found` 导致 offscreen Turnstile 失败 |
| 明确跳过 | 无（上游仅此提交）；Dockerfile 未改（上游未动；容器走 Playwright headless 回退，本次不扩大范围） |
| 兼容调整 | 仅 `scripts/install.sh` 依赖列表；Docker/WebUI/`OUTPUT_*` 不变 |
| 验证结果 | `go test` config/oauth/pipeline/webui/cpa/inventory/protocol/logx/turnstile/cmd/web 通过；`GOOS=linux go build` grok/grok-web 通过；`docker compose --env-file .env.docker.example config --quiet` 通过。本机 Windows `go test ./...` 仍因 daemon flock/Setsid 失败（上游既有，Linux/容器目标不受影响） |

### 2026-07-23 上游同步（稳定默认 7dc93c0）

| 项目 | 内容 |
|---|---|
| 同步分支 | `sync/upstream-20260723b` |
| 审查范围 | `3a66a6a..7dc93c0` |
| 新的审查截止 | `7dc93c0` |
| 实际移植 | `7dc93c0` 生产稳定默认：`OAUTH_MIN_INTERVAL_SEC=6`、`OAUTH_RETRY_SEC=60`、`PROBE_WARMUP_SEC=5`；Probe 无参默认 warmup 5s；交互线程回车默认 2 的说明与模板同步 |
| 明确跳过 | 上游 README 整页替换（仅保留本仓库文档结构；默认节奏说明在 CLI help / 配置模板中同步） |
| 兼容调整 | Docker（`.env.docker.example`、`entrypoint`）、Web 配置联动仍读配置文件；本仓库 `OUTPUT_*` 不受影响 |
| 验证结果 | `go test` config/cpa/pipeline/webui/inventory/logx 通过；linux build grok/grok-web 通过；docker compose config 通过 |

### 2026-07-23 上游同步（解卡修复 3a66a6a）

| 项目 | 内容 |
|---|---|
| 同步分支 | `sync/upstream-20260723` |
| 审查范围 | `3804a47..3a66a6a` |
| 新的审查截止 | `3a66a6a` |
| 实际移植 | `3a66a6a` 流水线解卡：Q TTL 过期释放 reserved 座位（`PutQWithExpire`/`onExpire`）、Q TTL 2m→8m、q_pending/t·q 槽位跟随 `--thread`、P workers 跟踪 S、OAuth device 429 重试、Probe 5xx/timeout 更长退避 |
| 明确跳过 | 无（该提交全部为核心修复，与本仓库扩展兼容） |
| 兼容调整 | 保留 `OUTPUT_*`、`turnstileMintNeed`、Docker/WebUI；解卡逻辑采用上游实现，不重复造轮子 |
| 功能对比结论 | 上游已实现 Q 座位泄漏修复（本仓库此前卡在 reserved 不释放）；采用上游代码。本仓库独有：Docker、WebUI、输出开关、target=1 Turnstile 等待修复 |
| 验证结果 | `go test` config/pipeline/webui/cpa/inventory/logx 通过；`GOOS=linux go build` grok/grok-web 通过；`docker compose --env-file .env.docker.example config --quiet` 通过 |

### 2026-07-23 上游同步


| 项目 | 内容 |
|---|---|
| 同步分支 | `sync/upstream-20260723` |
| 审查范围 | `a8d87bd..3804a47` |
| 新的审查截止 | `3804a47` |
| 实际移植 | `671f48d` Turnstile offscreen xvfb/headless fallback；`fc4682a` 全局 OAuth pacing、OAUTH_WORKERS、PROBE_ENABLED/PROBE_WARMUP_SEC、`grok2api/tokens.txt`、`grok stop` 清障栈、install 安全升级合并、daemon 优雅停止等待；`3804a47` 日志等级过滤（`grok logs --debug|--info|--warn|--error`） |
| 明确跳过 | 上游 README 整体替换（保留本仓库 Docker/WebUI 文档与克隆地址）；上游无 Docker/WebUI 的部分不影响 |
| 兼容调整 | 保留 `OUTPUT_SSO_ENABLED` / `OUTPUT_GROK2API_SSO_ENABLED` / `OUTPUT_CPA_ENABLED` 与 `turnstileMintNeed`；grok2api 改用上游 `AppendGrok2APIToken` + `outputs/<run>/grok2api/tokens.txt`；CPA 上传仍要求 `OUTPUT_CPA_ENABLED=1` 且 `CPA_UPLOAD_ENABLED=1`；Docker/WebUI/配置模板同步新键与默认 OAuth 间隔 4s、重试 45s |
| 验证结果 | `go test`：config/pipeline/webui/cpa/logx 通过；`GOOS=linux go build` grok/grok-web 通过；`docker compose --env-file .env.docker.example config --quiet` 通过。本机 Windows 直接 `go build` 仍受 daemon flock/Setsid 限制（上游既有，Linux/容器目标不受影响） |

### 初始独立维护基准

| 项目 | 内容 |
|---|---|
| 审查截止 | `a8d87bd` |
| 实际采用 | 以该提交为代码基准，并加入本仓库扩展清单中的功能 |
| 跳过内容 | 无 |
| 验证 | 见上方“合并后验证” |

后续每次同步在本节顶部新增记录，使用以下模板：

```md
### YYYY-MM-DD 上游同步

| 项目 | 内容 |
|---|---|
| 同步分支 | `sync/upstream-YYYYMMDD` |
| 审查范围 | `<previous-reviewed>..<new-reviewed>` |
| 新的审查截止 | `<commit>` |
| 实际移植 | `<提交或功能列表>` |
| 明确跳过 | `<提交、功能和原因>` |
| 兼容调整 | `<为 Docker/WebUI/输出行为所做的调整>` |
| 验证结果 | `<实际运行的命令和结果>` |
```
