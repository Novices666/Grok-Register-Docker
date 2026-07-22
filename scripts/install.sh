#!/usr/bin/env bash
# Grok-Register 一键部署脚本（Debian / Ubuntu）
#
# 一行安装（默认）:
#   curl -fsSL https://raw.githubusercontent.com/Charles-0509/Grok-Register/main/scripts/install.sh | sudo bash
#
# 自定义示例:
#   curl -fsSL ... | sudo bash -s -- --command grok-reg --install-dir /opt/Grok-Register --home /root/.grok
#   curl -fsSL ... | sudo COMMAND_NAME=grok-reg INSTALL_DIR=/data/Grok-Register bash
#
# 选项 / 环境变量见下方 usage。

set -euo pipefail

# ---------------------------------------------------------------------------
# 默认值（可被环境变量或 CLI 覆盖）
# ---------------------------------------------------------------------------
COMMAND_NAME="${COMMAND_NAME:-grok}"
INSTALL_DIR="${INSTALL_DIR:-/opt/Grok-Register}"
# 空 = 跟随运行用户 home 下的 .grok
GROK_HOME_OPT="${GROK_HOME:-}"
BIN_DIR="${BIN_DIR:-/usr/local/bin}"
SHARE_DIR="${SHARE_DIR:-/usr/local/share/grok-reg}"
VENV_DIR="${VENV_DIR:-/opt/cloakbrowser-venv}"
REPO_URL="${REPO_URL:-https://github.com/Charles-0509/Grok-Register.git}"
BRANCH="${BRANCH:-main}"
GO_VERSION="${GO_VERSION:-1.24.4}"
SKIP_DOCKER="${SKIP_DOCKER:-0}"
SKIP_CLEARANCE="${SKIP_CLEARANCE:-0}"
SKIP_BROWSER="${SKIP_BROWSER:-0}"
SKIP_GO_INSTALL="${SKIP_GO_INSTALL:-0}"
START_CLEARANCE="${START_CLEARANCE:-1}"
NONINTERACTIVE="${NONINTERACTIVE:-1}"

usage() {
  cat <<'EOF'
Grok-Register 一键部署

用法:
  install.sh [选项]

选项:
  --command NAME        CLI 命令名（默认 grok；可改成 grok-reg 等避免冲突）
  --install-dir PATH    源码安装目录（默认 /opt/Grok-Register）
  --home PATH           数据目录 GROK_HOME（默认 ~运行用户/.grok）
  --bin-dir PATH        二进制安装目录（默认 /usr/local/bin）
  --share-dir PATH      mint 脚本共享目录（默认 /usr/local/share/grok-reg）
  --venv-dir PATH       Python venv（默认 /opt/cloakbrowser-venv）
  --repo URL            Git 仓库地址
  --branch NAME         Git 分支（默认 main）
  --go-version VER      安装的 Go 版本（默认 1.24.4）
  --skip-docker         不安装 Docker
  --skip-clearance      不起 clearance 清障栈
  --skip-browser        不装 Playwright/CloakBrowser
  --skip-go             不自动安装 Go（需系统已有 go）
  --no-start-clearance  装完 Docker 但不 docker compose up
  -h, --help            显示帮助

等价环境变量:
  COMMAND_NAME INSTALL_DIR GROK_HOME BIN_DIR SHARE_DIR VENV_DIR
  REPO_URL BRANCH GO_VERSION SKIP_DOCKER SKIP_CLEARANCE SKIP_BROWSER
  SKIP_GO_INSTALL START_CLEARANCE

示例:
  # 命令改名为 grok-reg，避免与其它 grok CLI 冲突
  curl -fsSL ... | sudo bash -s -- --command grok-reg

  # 自定义安装目录与数据目录
  curl -fsSL ... | sudo bash -s -- --install-dir /data/Grok-Register --home /data/grok-data
EOF
}

log()  { printf '[*] %s\n' "$*"; }
ok()   { printf '[✓] %s\n' "$*"; }
warn() { printf '[!] %s\n' "$*" >&2; }
die()  { printf '[x] %s\n' "$*" >&2; exit 1; }

# ---------------------------------------------------------------------------
# 参数解析
# ---------------------------------------------------------------------------
while [ $# -gt 0 ]; do
  case "$1" in
    --command) COMMAND_NAME="$2"; shift 2 ;;
    --install-dir) INSTALL_DIR="$2"; shift 2 ;;
    --home) GROK_HOME_OPT="$2"; shift 2 ;;
    --bin-dir) BIN_DIR="$2"; shift 2 ;;
    --share-dir) SHARE_DIR="$2"; shift 2 ;;
    --venv-dir) VENV_DIR="$2"; shift 2 ;;
    --repo) REPO_URL="$2"; shift 2 ;;
    --branch) BRANCH="$2"; shift 2 ;;
    --go-version) GO_VERSION="$2"; shift 2 ;;
    --skip-docker) SKIP_DOCKER=1; shift ;;
    --skip-clearance) SKIP_CLEARANCE=1; START_CLEARANCE=0; shift ;;
    --skip-browser) SKIP_BROWSER=1; shift ;;
    --skip-go) SKIP_GO_INSTALL=1; shift ;;
    --no-start-clearance) START_CLEARANCE=0; shift ;;
    -h|--help) usage; exit 0 ;;
    *) die "未知参数: $1（--help 查看用法）" ;;
  esac
done

# 校验命令名
case "$COMMAND_NAME" in
  *[!a-zA-Z0-9._-]*|"") die "非法命令名: $COMMAND_NAME" ;;
esac

# ---------------------------------------------------------------------------
# root
# ---------------------------------------------------------------------------
if [ "$(id -u)" -ne 0 ]; then
  if command -v sudo >/dev/null 2>&1; then
    log "需要 root，通过 sudo 重新执行..."
    exec sudo -E env \
      COMMAND_NAME="$COMMAND_NAME" \
      INSTALL_DIR="$INSTALL_DIR" \
      GROK_HOME="$GROK_HOME_OPT" \
      BIN_DIR="$BIN_DIR" \
      SHARE_DIR="$SHARE_DIR" \
      VENV_DIR="$VENV_DIR" \
      REPO_URL="$REPO_URL" \
      BRANCH="$BRANCH" \
      GO_VERSION="$GO_VERSION" \
      SKIP_DOCKER="$SKIP_DOCKER" \
      SKIP_CLEARANCE="$SKIP_CLEARANCE" \
      SKIP_BROWSER="$SKIP_BROWSER" \
      SKIP_GO_INSTALL="$SKIP_GO_INSTALL" \
      START_CLEARANCE="$START_CLEARANCE" \
      bash "$0" \
      --command "$COMMAND_NAME" \
      --install-dir "$INSTALL_DIR" \
      ${GROK_HOME_OPT:+--home "$GROK_HOME_OPT"} \
      --bin-dir "$BIN_DIR" \
      --share-dir "$SHARE_DIR" \
      --venv-dir "$VENV_DIR" \
      --repo "$REPO_URL" \
      --branch "$BRANCH" \
      --go-version "$GO_VERSION" \
      $([ "$SKIP_DOCKER" = 1 ] && echo --skip-docker) \
      $([ "$SKIP_CLEARANCE" = 1 ] && echo --skip-clearance) \
      $([ "$SKIP_BROWSER" = 1 ] && echo --skip-browser) \
      $([ "$SKIP_GO_INSTALL" = 1 ] && echo --skip-go) \
      $([ "$START_CLEARANCE" = 0 ] && echo --no-start-clearance)
  fi
  die "请使用 root 或 sudo 运行本脚本"
fi

# 数据目录：未指定时用 root 的 ~/.grok
if [ -z "$GROK_HOME_OPT" ]; then
  GROK_HOME_OPT="/root/.grok"
fi

export DEBIAN_FRONTEND=noninteractive
export PATH="${PATH}:/usr/local/go/bin"

ARCH="$(uname -m)"
case "$ARCH" in
  x86_64|amd64) GO_ARCH=amd64 ;;
  aarch64|arm64) GO_ARCH=arm64 ;;
  *) die "暂不支持架构: $ARCH（仅 amd64/arm64）" ;;
esac

if [ ! -f /etc/os-release ]; then
  die "仅支持 Debian/Ubuntu（需要 /etc/os-release）"
fi
# shellcheck source=/dev/null
. /etc/os-release
case "${ID:-}" in
  debian|ubuntu) ;;
  *) warn "未识别发行版 ID=${ID:-?}，将按 Debian/Ubuntu 继续尝试" ;;
esac

echo
echo "=============================================="
echo " Grok-Register 一键部署"
echo "=============================================="
echo "  命令名:     $COMMAND_NAME"
echo "  源码目录:   $INSTALL_DIR"
echo "  数据目录:   $GROK_HOME_OPT"
echo "  二进制:     $BIN_DIR/$COMMAND_NAME"
echo "  脚本共享:   $SHARE_DIR"
echo "  Python venv:$VENV_DIR"
echo "  仓库:       $REPO_URL ($BRANCH)"
echo "=============================================="
echo

# ---------------------------------------------------------------------------
# 0. 系统包
# ---------------------------------------------------------------------------
log "安装系统依赖..."
apt-get update -y
# libasound: trixie 用 t64
ALSA_PKG=libasound2t64
if ! apt-cache show libasound2t64 >/dev/null 2>&1; then
  ALSA_PKG=libasound2
fi
apt-get install -y --no-install-recommends \
  git curl ca-certificates gnupg lsb-release \
  build-essential make \
  python3 python3-pip python3-venv \
  libnss3 libnspr4 libatk1.0-0 libatk-bridge2.0-0 libcups2 \
  libdrm2 libxkbcommon0 libxcomposite1 libxdamage1 libxfixes3 \
  libxrandr2 libgbm1 "$ALSA_PKG" libpango-1.0-0 libcairo2 \
  fonts-liberation fonts-noto-cjk \
  || warn "部分包安装失败，可稍后手动补齐"

ok "系统依赖就绪"

# ---------------------------------------------------------------------------
# 1. Go
# ---------------------------------------------------------------------------
need_go=0
if ! command -v go >/dev/null 2>&1; then
  need_go=1
elif ! go version 2>/dev/null | grep -qE 'go1\.(2[1-9]|[3-9][0-9])'; then
  warn "检测到较旧 Go: $(go version 2>/dev/null || true)，将尝试安装 ${GO_VERSION}"
  need_go=1
fi

if [ "$need_go" = 1 ]; then
  if [ "$SKIP_GO_INSTALL" = 1 ]; then
    die "系统无可用 Go 1.21+ 且指定了 --skip-go"
  fi
  log "安装 Go ${GO_VERSION} (${GO_ARCH})..."
  tmp="/tmp/go${GO_VERSION}.linux-${GO_ARCH}.tar.gz"
  curl -fsSL -o "$tmp" "https://go.dev/dl/go${GO_VERSION}.linux-${GO_ARCH}.tar.gz"
  rm -rf /usr/local/go
  tar -C /usr/local -xzf "$tmp"
  rm -f "$tmp"
  echo 'export PATH=$PATH:/usr/local/go/bin' >/etc/profile.d/go.sh
  export PATH="/usr/local/go/bin:${PATH}"
  ok "Go $(go version)"
else
  ok "使用已有 Go: $(go version)"
fi

# ---------------------------------------------------------------------------
# 2. Docker
# ---------------------------------------------------------------------------
if [ "$SKIP_DOCKER" != 1 ]; then
  if ! command -v docker >/dev/null 2>&1; then
    log "安装 Docker..."
    curl -fsSL https://get.docker.com | sh
    systemctl enable --now docker 2>/dev/null || true
  else
    ok "Docker 已存在: $(docker --version 2>/dev/null || true)"
  fi
  if ! docker compose version >/dev/null 2>&1; then
    log "安装 docker compose plugin..."
    apt-get install -y docker-compose-plugin 2>/dev/null || true
  fi
  ok "Docker: $(docker --version 2>/dev/null || echo '?')"
else
  warn "已跳过 Docker 安装"
fi

# ---------------------------------------------------------------------------
# 3. 源码
# ---------------------------------------------------------------------------
log "同步源码 → $INSTALL_DIR"
mkdir -p "$(dirname "$INSTALL_DIR")"
if [ -d "$INSTALL_DIR/.git" ]; then
  git -C "$INSTALL_DIR" remote set-url origin "$REPO_URL" || true
  git -C "$INSTALL_DIR" fetch origin
  git -C "$INSTALL_DIR" checkout "$BRANCH"
  git -C "$INSTALL_DIR" reset --hard "origin/$BRANCH"
else
  rm -rf "$INSTALL_DIR"
  git clone --branch "$BRANCH" --depth 1 "$REPO_URL" "$INSTALL_DIR"
fi
# 兼容旧路径软链
if [ "$INSTALL_DIR" = "/opt/Grok-Register" ]; then
  ln -sfn "$INSTALL_DIR" /opt/Grok-Reg 2>/dev/null || true
fi
ok "源码: $(git -C "$INSTALL_DIR" log -1 --oneline 2>/dev/null || echo ok)"

# ---------------------------------------------------------------------------
# 4. 编译安装 CLI
# ---------------------------------------------------------------------------
log "编译并安装 CLI → $BIN_DIR/$COMMAND_NAME"
export PATH="/usr/local/go/bin:${PATH}"
cd "$INSTALL_DIR"
mkdir -p bin
go build -ldflags "-s -w -X main.version=0.1.0" -o "bin/${COMMAND_NAME}" ./cmd/grok
install -d "$BIN_DIR"
install -m 755 "bin/${COMMAND_NAME}" "${BIN_DIR}/${COMMAND_NAME}"
install -d "$SHARE_DIR"
install -m 755 scripts/turnstile_mint.py "${SHARE_DIR}/turnstile_mint.py"
install -m 755 scripts/turnstile_pool.py "${SHARE_DIR}/turnstile_pool.py"
# 也保留仓库内 scripts 供 GROK_TURNSTILE_* 使用
ok "已安装 ${BIN_DIR}/${COMMAND_NAME}"
ok "已安装 mint 脚本 → $SHARE_DIR"

# ---------------------------------------------------------------------------
# 5. Playwright + CloakBrowser
# ---------------------------------------------------------------------------
if [ "$SKIP_BROWSER" != 1 ]; then
  log "安装 Python venv + Playwright + CloakBrowser → $VENV_DIR"
  python3 -m venv "$VENV_DIR"
  "${VENV_DIR}/bin/pip" install -U pip
  "${VENV_DIR}/bin/pip" install -r "$INSTALL_DIR/scripts/requirements-turnstile.txt"
  # CloakBrowser chromium 装到 root home
  HOME=/root "${VENV_DIR}/bin/python" -m cloakbrowser install || \
    "${VENV_DIR}/bin/python" -m cloakbrowser install
  ok "浏览器依赖就绪"
else
  warn "已跳过浏览器依赖（Turnstile 将不可用）"
fi

# ---------------------------------------------------------------------------
# 6. 清障栈
# ---------------------------------------------------------------------------
if [ "$SKIP_CLEARANCE" != 1 ] && [ "$SKIP_DOCKER" != 1 ] && [ "$START_CLEARANCE" = 1 ]; then
  if command -v docker >/dev/null 2>&1; then
    log "启动 clearance 清障栈..."
    if [ -f "$INSTALL_DIR/clearance/docker-compose.yml" ]; then
      (cd "$INSTALL_DIR/clearance" && docker compose up -d) || warn "clearance 启动失败，可稍后手动: cd $INSTALL_DIR/clearance && docker compose up -d"
      (cd "$INSTALL_DIR/clearance" && docker compose ps) || true
    fi
  fi
else
  warn "未启动 clearance（SKIP_CLEARANCE/SKIP_DOCKER/no-start）"
fi

# ---------------------------------------------------------------------------
# 7. 数据目录 + 默认 config + 环境片段
# ---------------------------------------------------------------------------
log "准备数据目录 $GROK_HOME_OPT"
mkdir -p "$GROK_HOME_OPT" "$GROK_HOME_OPT/logs" "$GROK_HOME_OPT/outputs"
chmod 700 "$GROK_HOME_OPT" 2>/dev/null || true

# 同步 example
if [ -x "${BIN_DIR}/${COMMAND_NAME}" ]; then
  # worker-less: 直接写 example
  if [ -f "$INSTALL_DIR/internal/config/example.env" ]; then
    cp -f "$INSTALL_DIR/internal/config/example.env" "$GROK_HOME_OPT/config.env.example"
  elif [ -f "$INSTALL_DIR/config.env.example" ]; then
    cp -f "$INSTALL_DIR/config.env.example" "$GROK_HOME_OPT/config.env.example"
  fi
fi

if [ ! -f "$GROK_HOME_OPT/config.env" ]; then
  log "写入默认 config.env（EMAIL_MODE=tempmail）"
  cat >"$GROK_HOME_OPT/config.env" <<EOF
# 由 install.sh 生成 — 也可用 ${COMMAND_NAME} config 编辑
# 完整说明见: ${GROK_HOME_OPT}/config.env.example

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
CPA_UPLOAD_VERIFY=1
CPA_UPLOAD_MODE=multipart
EOF
  chmod 600 "$GROK_HOME_OPT/config.env"
else
  ok "保留已有 config.env"
fi

# profile 片段：GROK_HOME / GROK_PYTHON / PATH
PROFILE_SNIPPET="/etc/profile.d/grok-register.sh"
cat >"$PROFILE_SNIPPET" <<EOF
# Grok-Register (generated by install.sh)
export PATH="\$PATH:/usr/local/go/bin:${BIN_DIR}"
export GROK_HOME="${GROK_HOME_OPT}"
export GROK_PYTHON="${VENV_DIR}/bin/python"
export GROK_TURNSTILE_SCRIPT="${SHARE_DIR}/turnstile_mint.py"
export GROK_TURNSTILE_POOL_SCRIPT="${SHARE_DIR}/turnstile_pool.py"
export CLOAKBROWSER_SUPPRESS_FONT_WARNING=1
EOF
chmod 644 "$PROFILE_SNIPPET"

# 当前 root shell 也 export
export GROK_HOME="$GROK_HOME_OPT"
export GROK_PYTHON="${VENV_DIR}/bin/python"
export GROK_TURNSTILE_SCRIPT="${SHARE_DIR}/turnstile_mint.py"
export GROK_TURNSTILE_POOL_SCRIPT="${SHARE_DIR}/turnstile_pool.py"
export CLOAKBROWSER_SUPPRESS_FONT_WARNING=1

# root bashrc（方便交互）
if [ -f /root/.bashrc ] && ! grep -q 'GROK_HOME=' /root/.bashrc 2>/dev/null; then
  {
    echo ""
    echo "# Grok-Register"
    echo "export GROK_HOME=\"${GROK_HOME_OPT}\""
    echo "export GROK_PYTHON=\"${VENV_DIR}/bin/python\""
    echo "export GROK_TURNSTILE_SCRIPT=\"${SHARE_DIR}/turnstile_mint.py\""
    echo "export GROK_TURNSTILE_POOL_SCRIPT=\"${SHARE_DIR}/turnstile_pool.py\""
    echo "export CLOAKBROWSER_SUPPRESS_FONT_WARNING=1"
  } >>/root/.bashrc
fi

# ---------------------------------------------------------------------------
# 完成
# ---------------------------------------------------------------------------
echo
echo "=============================================="
ok "部署完成"
echo "=============================================="
echo
echo "  命令:     ${COMMAND_NAME} help"
echo "  源码:     ${INSTALL_DIR}"
echo "  配置:     ${GROK_HOME_OPT}/config.env"
echo "  示例:     ${GROK_HOME_OPT}/config.env.example"
echo "  环境:     ${PROFILE_SNIPPET}"
echo
echo "快速开始:"
echo "  export GROK_HOME=${GROK_HOME_OPT}"
echo "  export GROK_PYTHON=${VENV_DIR}/bin/python"
echo "  ${COMMAND_NAME} start                 # 交互输入数量与线程"
echo "  ${COMMAND_NAME} start -t 10 --thread 2"
echo "  ${COMMAND_NAME} status"
echo "  ${COMMAND_NAME} logs -f"
echo "  ${COMMAND_NAME} config                # 编辑配置"
echo
if [ "$COMMAND_NAME" != "grok" ]; then
  echo "提示: 命令名为 ${COMMAND_NAME}（不是 grok），避免与其它 CLI 冲突。"
fi
echo "若 clearance 未 healthy: cd ${INSTALL_DIR}/clearance && docker compose up -d && docker compose ps"
echo
"${BIN_DIR}/${COMMAND_NAME}" help 2>/dev/null || true
