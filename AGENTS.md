# AI Repository Instructions

## 仓库身份

本仓库是独立维护项目：

- 当前仓库：`https://github.com/Novices666/Grok-Register-Docker`
- 参考上游：`https://github.com/Charles-0509/Grok-Register`
- 默认分支：`main`
- 上游同步说明：`MERGE_GUIDE.zh-CN.md`

本仓库不再依赖 GitHub Fork 关系。参考上游仅用于检查和选择性吸收核心更新，不是本仓库的发布源。

处理上游更新前，必须完整阅读 `MERGE_GUIDE.zh-CN.md`。

## 核心维护规则

1. 将当前仓库内容作为实现和行为基准。
2. 不得直接用上游文件、分支或提交覆盖本仓库。
3. 不得在 `main` 上直接执行 `git merge upstream/main`。
4. 不得把 `origin` 改成上游地址。
5. 不得丢弃、覆盖或回滚用户已有的未提交修改。
6. 上游更新必须在独立的 `sync/upstream-YYYYMMDD` 分支处理。
7. 只移植已经理解、确认需要且能通过本仓库测试的上游变化。
8. Docker、WebUI、输出控制和本仓库配置兼容性优先于上游文件结构。
9. 未完成验证时，不得更新同步基准或声称同步完成。
10. 未经用户明确授权，不得提交、推送、创建 Pull Request 或发布版本。

## 分支命名规则

除只读检查外，代码或文档修改应从最新的本仓库 `main` 创建独立分支，不直接在 `main` 上开发。

| 变更类型 | 分支格式 | 示例 |
|---|---|---|
| 新功能 | `feat/<简短-kebab-case-说明>` | `feat/webui-history-preview` |
| 缺陷修复 | `fix/<简短-kebab-case-说明>` | `fix/config-propagation` |
| 文档 | `docs/<简短-kebab-case-说明>` | `docs/deployment-guide` |
| 重构 | `refactor/<简短-kebab-case-说明>` | `refactor/pipeline-workers` |
| 测试 | `test/<简短-kebab-case-说明>` | `test/output-gating` |
| 工程维护 | `chore/<简短-kebab-case-说明>` | `chore/update-toolchain` |
| 上游同步 | `sync/upstream-YYYYMMDD` | `sync/upstream-20260723` |

分支说明使用小写英文、数字和连字符，不使用空格、下划线或个人/工具名前缀。`sync/upstream-*` 仅用于按本文件流程处理参考上游更新。

## 本仓库必须保留的扩展

| 扩展 | 关键位置 |
|---|---|
| Docker 一键服务 | `Dockerfile`、`docker-compose.yml`、`docker/entrypoint.sh`、`.env.docker.example` |
| Web 管理端 | `cmd/web/main.go`、`internal/webui/` |
| SSO 输出控制 | `internal/config/config.go`、`internal/pipeline/pipeline.go`，配置 `OUTPUT_SSO_ENABLED` |
| grok2api SSO 输出 | `internal/cpa/cpa.go`、`internal/pipeline/pipeline.go`，配置 `OUTPUT_GROK2API_SSO_ENABLED` |
| CPA 链路控制 | `internal/config/config.go`、`internal/pipeline/pipeline.go`，配置 `OUTPUT_CPA_ENABLED` |
| CPA 自动上传约束 | 只有 `OUTPUT_CPA_ENABLED=1` 且 `CPA_UPLOAD_ENABLED=1` 时上传 |
| 目标为 1 时的等待修复 | `internal/pipeline/pipeline.go` 中的 `turnstileMintNeed` |
| Web 配置联动 | `internal/webui/server.go`、`internal/webui/templates/` |
| 配置模板一致性 | `config.env.example`、`internal/config/example.env`、`.env.docker.example`、`docker/entrypoint.sh` |

修改共享核心逻辑时，必须确认这些扩展仍然生效。

## 上游更新流程

### 1. 检查工作区

先执行：

```powershell
git status --short --branch
git remote -v
```

如果存在用户修改，保留这些修改并在其基础上工作。除非用户明确要求，否则不要暂存、提交或清理它们。

### 2. 配置参考上游

仅在尚未配置时添加：

```powershell
git remote add upstream https://github.com/Charles-0509/Grok-Register.git
```

然后获取更新：

```powershell
git fetch upstream --prune
```

### 3. 创建同步分支

从本仓库当前 `main` 创建分支：

```powershell
git switch main
git switch -c sync/upstream-YYYYMMDD
```

如果工作区有未提交修改，切换分支前先判断是否会影响用户工作；不得自行 stash、reset 或 checkout 覆盖。

### 4. 确定审查范围

从 `MERGE_GUIDE.zh-CN.md` 记录的“上次已审查上游提交”开始检查：

```powershell
git log --oneline <last-reviewed>..upstream/main
git diff --stat <last-reviewed>..upstream/main
git diff <last-reviewed>..upstream/main -- cmd internal scripts config.env.example
```

先输出上游变化摘要，将提交分成：

- 建议移植
- 无需移植
- 需要用户决定

涉及大范围结构调整、行为不明确或可能删除本仓库功能时，先向用户说明影响，不要自行继续。

### 5. 选择性移植

通常可以评估并吸收：

- x.ai 协议或接口兼容更新
- 邮箱提供方实现和重试逻辑
- OAuth、CPA 探活和上传修复
- Turnstile、浏览器和 Cloudflare 兼容修复
- TLS 指纹、代理和清障核心逻辑
- 明确的并发、资源泄漏或错误处理修复

默认不要直接采用：

- 覆盖 Docker 或 WebUI 的文件
- 删除输出开关或改变现有输出语义的修改
- 把配置模板恢复成上游版本的修改
- 改回上游仓库地址、安装路径或 README 的修改
- 与本仓库运行方式无关的大规模重构

独立且无冲突的上游修复可以考虑 `git cherry-pick`。与本仓库扩展交叉的修改应理解后手工移植，并补充或更新测试。

## 输出行为约束

默认 `OUTPUT_*=1`，与上游完整流水线同构。语义冲突以上游为准。

| 配置 | 必须保持的行为 |
|---|---|
| `OUTPUT_SSO_ENABLED=1`（默认） | 输出 `accounts.txt` 和 `auth-sessions.jsonl` |
| `OUTPUT_SSO_ENABLED=0` | 不写 SSO 文件；CPA=1 时仍执行 OAuth/CPA |
| `OUTPUT_GROK2API_SSO_ENABLED=1`（默认） | 在 SSO 输出开启时额外输出 `grok2api/tokens.txt` |
| `OUTPUT_CPA_ENABLED=0` | 本仓扩展：获得 SSO 后计完成，跳过 OAuth、探活和 CPA JSON |
| `OUTPUT_CPA_ENABLED=1`（默认） | 与上游一致：OAuth、探活并输出 CPA JSON；目标按 CPA 就绪 |
| `CPA_UPLOAD_ENABLED=1` | 仅在 CPA 输出开启时允许自动上传 |
| `TURNSTILE_WORKERS` | 不得被 `config.Load` 当作 CLI 线程来源；仅 Web/Docker 启动 `-j` 便利默认 |

修改流水线、配置或计数逻辑时，必须为相关组合保留或增加测试。

## 配置同步约束

新增、删除或修改配置项时，同时检查：

- `config.env.example`
- `internal/config/example.env`
- `internal/config/config.go`
- `.env.docker.example`
- `docker-compose.yml`
- `docker/entrypoint.sh`
- `internal/webui/server.go`
- `internal/webui/templates/index.html`
- `README.md`

不能只修改其中一个入口，导致 CLI、Docker 和 Web 使用不同默认值。

## 验证要求

至少运行：

```powershell
go test ./...
go build -o bin/grok ./cmd/grok
go build -o bin/grok-web ./cmd/web
docker compose --env-file .env.docker.example config --quiet
```

涉及 Dockerfile、依赖、入口脚本或运行镜像时，再运行：

```powershell
docker compose --env-file .env.docker.example build
```

涉及 WebUI 时，应检查：

- 登录认证
- 启动和停止
- 状态与日志刷新
- 配置读取和保存
- 输出联动显示
- 历史记录、下载和删除
- 桌面与移动端布局

无法运行的验证必须在结果中明确说明原因。

## 同步记录

同步完成且验证通过后，更新 `MERGE_GUIDE.zh-CN.md`，至少记录：

- 上次已审查上游提交
- 本次实际移植的上游提交或功能
- 明确跳过的提交或功能及原因
- 冲突处理和本仓库兼容调整
- 实际运行的测试及结果

“已审查提交”表示 AI 已检查到该上游提交，不代表该提交已全部合并。选择性移植时，必须把“已审查”和“已采用”分开记录。

## 安全与仓库卫生

- 不读取、输出或提交 `.env`、`data/`、账号、Token、Cookie、密钥或生成的 SSO / CPA 文件。
- 不提交 `bin/`、日志、运行状态或临时文件。
- 不执行 `git reset --hard`、`git clean -fd` 或覆盖式 checkout。
- 不因同步上游而删除本仓库测试。
- 文档中的克隆、安装和更新地址必须指向 `Novices666/Grok-Register-Docker`；上游地址只用于来源说明和同步流程。
