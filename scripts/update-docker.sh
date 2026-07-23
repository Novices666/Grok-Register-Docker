#!/usr/bin/env bash
# Grok-Register-Docker 一键更新（Linux）
#
# 在仓库根目录或 scripts/ 下执行：
#   bash scripts/update-docker.sh
#   bash scripts/update-docker.sh --no-git
#   bash scripts/update-docker.sh --pull-only
#   bash scripts/update-docker.sh --prune
#
# 默认行为：
#   1. 检查 Docker / Compose
#   2. （可选）git pull --ff-only 更新代码
#   3. 拉取依赖镜像（warp / privoxy / flaresolverr）
#   4. 重新构建并强制重建容器
#   5. 打印状态与最近日志
#
# 不会删除 ./data/grok、.env 或历史输出。

set -euo pipefail

# ---------------------------------------------------------------------------
# 路径与默认值
# ---------------------------------------------------------------------------
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"
cd "$ROOT_DIR"

DO_GIT=1
DO_BUILD=1
DO_RECREATE=1
DO_PRUNE=0
SKIP_CONFIRM=0
ENV_FILE=".env"
COMPOSE_FILE="docker-compose.yml"
BRANCH=""
GIT_REMOTE="origin"

# ---------------------------------------------------------------------------
# 输出
# ---------------------------------------------------------------------------
log()  { printf '[*] %s\n' "$*"; }
ok()   { printf '[+] %s\n' "$*"; }
warn() { printf '[!] %s\n' "$*" >&2; }
die()  { printf '[x] %s\n' "$*" >&2; exit 1; }

usage() {
  cat <<'EOF'
用法: bash scripts/update-docker.sh [选项]

选项:
  --no-git            不执行 git pull，只更新/重建镜像
  --pull-only         只拉取第三方镜像，不重建本地 grok-register
  --no-recreate       构建后不 force-recreate，仅 up -d
  --prune             更新后清理悬空镜像 (docker image prune -f)
  --env-file PATH     指定 env 文件（默认 .env）
  --compose-file PATH 指定 compose 文件（默认 docker-compose.yml）
  --branch NAME       git pull 时切换到指定分支（默认保持当前分支）
  --remote NAME       git remote（默认 origin）
  -y, --yes           跳过确认提示
  -h, --help          显示帮助

示例:
  # 常规更新：拉代码 + 拉依赖镜像 + 重建并重启
  bash scripts/update-docker.sh

  # 只刷新依赖镜像（不改代码、不重建主应用）
  bash scripts/update-docker.sh --pull-only -y

  # 本地已改过代码，只重建并重启
  bash scripts/update-docker.sh --no-git

说明:
  - 数据目录 ./data/grok 与 .env 会保留，不会被清理。
  - 更新前若任务正在跑，容器重建会中断当前注册；脚本会提示确认。
  - 需要 Docker Compose v2，且能访问构建/镜像源。
EOF
}

# ---------------------------------------------------------------------------
# 参数
# ---------------------------------------------------------------------------
while [ $# -gt 0 ]; do
  case "$1" in
    --no-git)       DO_GIT=0; shift ;;
    --pull-only)    DO_BUILD=0; DO_RECREATE=0; shift ;;
    --no-recreate)  DO_RECREATE=0; shift ;;
    --prune)        DO_PRUNE=1; shift ;;
    --env-file)     ENV_FILE="${2:-}"; shift 2 ;;
    --compose-file) COMPOSE_FILE="${2:-}"; shift 2 ;;
    --branch)       BRANCH="${2:-}"; shift 2 ;;
    --remote)       GIT_REMOTE="${2:-}"; shift 2 ;;
    -y|--yes)       SKIP_CONFIRM=1; shift ;;
    -h|--help)      usage; exit 0 ;;
    *)
      die "未知参数: $1（使用 --help 查看用法）"
      ;;
  esac
done

[ -n "$ENV_FILE" ] || die "--env-file 不能为空"
[ -n "$COMPOSE_FILE" ] || die "--compose-file 不能为空"
[ -f "$COMPOSE_FILE" ] || die "找不到 compose 文件: $COMPOSE_FILE（请在仓库根目录执行）"

# ---------------------------------------------------------------------------
# 前置检查
# ---------------------------------------------------------------------------
require_cmd() {
  command -v "$1" >/dev/null 2>&1 || die "未找到命令: $1"
}

check_os() {
  case "$(uname -s 2>/dev/null || echo unknown)" in
    Linux) ;;
    Darwin)
      warn "脚本按 Linux 部署路径编写；在 macOS 上可尝试继续，但未专门测试。"
      ;;
    *)
      die "不支持的系统（请在 Linux 上使用）"
      ;;
  esac
}

compose() {
  # shellcheck disable=SC2086
  docker compose -f "$COMPOSE_FILE" --env-file "$ENV_FILE" "$@"
}

check_docker() {
  require_cmd docker
  docker info >/dev/null 2>&1 || die "Docker 未运行或当前用户无权限（可试: sudo usermod -aG docker \$USER）"
  docker compose version >/dev/null 2>&1 || die "需要 Docker Compose v2（docker compose）"
}

check_env() {
  if [ ! -f "$ENV_FILE" ]; then
    if [ -f ".env.docker.example" ]; then
      die "缺少 $ENV_FILE。请先: cp .env.docker.example .env 并设置 WEB_PASSWORD"
    fi
    die "缺少 $ENV_FILE"
  fi
  if ! grep -Eq '^[[:space:]]*WEB_PASSWORD=[^[:space:]#]+' "$ENV_FILE" 2>/dev/null; then
    warn "$ENV_FILE 中未检测到非空 WEB_PASSWORD，compose 可能启动失败"
  fi
}

# ---------------------------------------------------------------------------
# Git 更新
# ---------------------------------------------------------------------------
git_update() {
  [ "$DO_GIT" = 1 ] || { log "跳过 git pull (--no-git)"; return 0; }
  require_cmd git
  [ -d .git ] || die "当前目录不是 git 仓库，无法 pull。可使用 --no-git 仅更新镜像"

  if [ -n "$(git status --porcelain 2>/dev/null || true)" ]; then
    warn "工作区有未提交修改，git pull --ff-only 可能失败"
    git status --short || true
    if [ "$SKIP_CONFIRM" != 1 ]; then
      printf '仍尝试 git pull？[y/N] '
      read -r ans || true
      case "${ans:-}" in
        y|Y|yes|YES) ;;
        *) die "已取消。可先提交/暂存本地修改，或加 --no-git" ;;
      esac
    fi
  fi

  if [ -n "$BRANCH" ]; then
    log "切换到分支: $BRANCH"
    git fetch "$GIT_REMOTE" "$BRANCH"
    git checkout "$BRANCH"
  else
    log "获取远程更新: $GIT_REMOTE"
    git fetch "$GIT_REMOTE" --prune
  fi

  local current
  current="$(git rev-parse --abbrev-ref HEAD 2>/dev/null || echo HEAD)"
  log "当前分支: $current"

  local before after
  before="$(git rev-parse HEAD)"
  if git pull --ff-only "$GIT_REMOTE" "$current"; then
    after="$(git rev-parse HEAD)"
    if [ "$before" = "$after" ]; then
      ok "代码已是最新 ($before)"
    else
      ok "代码已更新: ${before:0:8} -> ${after:0:8}"
      git log --oneline "$before..$after" || true
    fi
  else
    die "git pull --ff-only 失败。请手动处理分支分歧，或使用 --no-git"
  fi
}

# ---------------------------------------------------------------------------
# 镜像与服务
# ---------------------------------------------------------------------------
confirm_restart() {
  [ "$SKIP_CONFIRM" = 1 ] && return 0
  if ! docker compose -f "$COMPOSE_FILE" --env-file "$ENV_FILE" ps --status running -q 2>/dev/null | grep -q .; then
    return 0
  fi
  warn "检测到运行中的容器；重建会短暂中断服务（进行中的注册任务会中断）。"
  printf '继续更新并重建？[y/N] '
  read -r ans || true
  case "${ans:-}" in
    y|Y|yes|YES) ;;
    *) die "已取消" ;;
  esac
}

pull_images() {
  log "拉取 compose 依赖镜像..."
  # 本地 build 的 grok-register 仍会尝试 pull 基础层；失败不致命时由 build 补救
  if compose pull --ignore-buildable 2>/dev/null; then
    ok "依赖镜像已拉取"
  elif compose pull; then
    ok "镜像已拉取"
  else
    warn "部分镜像 pull 失败，将继续尝试 build / up"
  fi
}

rebuild_and_up() {
  if [ "$DO_BUILD" = 1 ]; then
    log "构建本地镜像 grok-register:local ..."
    compose build
    ok "构建完成"
  else
    log "跳过本地构建 (--pull-only)"
  fi

  if [ "$DO_BUILD" = 0 ] && [ "$DO_RECREATE" = 0 ]; then
    log "仅拉取镜像模式：重新拉起已有服务（不强制 recreate）"
    compose up -d
  elif [ "$DO_RECREATE" = 1 ]; then
    log "启动并 force-recreate 全部服务..."
    compose up -d --build --force-recreate
  else
    log "启动服务（不 force-recreate）..."
    compose up -d --build
  fi
  ok "服务已更新"
}

show_status() {
  log "服务状态:"
  compose ps || true
  echo
  log "最近 grok-register 日志:"
  compose logs --tail 40 grok-register 2>/dev/null || true
}

prune_dangling() {
  [ "$DO_PRUNE" = 1 ] || return 0
  log "清理悬空镜像..."
  docker image prune -f
  ok "清理完成"
}

# ---------------------------------------------------------------------------
# 主流程
# ---------------------------------------------------------------------------
main() {
  check_os
  check_docker
  check_env

  log "仓库目录: $ROOT_DIR"
  log "Compose:  $COMPOSE_FILE"
  log "Env:      $ENV_FILE"
  echo

  # 校验 compose 配置可读
  log "校验 compose 配置..."
  compose config --quiet
  ok "compose 配置有效"
  echo

  git_update
  echo
  confirm_restart
  pull_images
  echo
  rebuild_and_up
  echo
  prune_dangling
  show_status

  echo
  ok "更新完成。数据目录 ./data/grok 与 $ENV_FILE 均已保留。"
  log "Web 控制台默认: http://127.0.0.1:\${WEB_PORT:-8090}"
  log "查看日志: docker compose -f $COMPOSE_FILE --env-file $ENV_FILE logs -f grok-register"
}

main
