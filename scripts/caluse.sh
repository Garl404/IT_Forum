#!/bin/sh
# ============================================================================
#  CALUSE 网站启动脚本
#
#  兼容主流 Linux 发行版：
#    systemd  : Debian/Ubuntu/CentOS/RHEL/Rocky/Alma/Fedora/Arch/openSUSE
#    SysV init: 老旧 Debian/CentOS 6
#    OpenRC   : Alpine/Gentoo
#    手动模式 : 任何有 sh 的系统
#
#  用法:
#    ./caluse.sh start     启动（自动构建/健康检查）
#    ./caluse.sh stop      停止
#    ./caluse.sh restart   重启
#    ./caluse.sh status    查看状态
#    ./caluse.sh logs      跟踪日志
#    ./caluse.sh build     仅编译
#    ./caluse.sh install   安装为系统服务（需 root，自动识别 init 系统）
#    ./caluse.sh uninstall 卸载系统服务
#
#  可选环境变量:
#    CALUSE_BACKEND  后端目录（默认: 本脚本同级 ../backend）
#    CALUSE_CONFIG   配置文件（默认: $CALUSE_BACKEND/config.yaml）
#    CALUSE_USER     服务运行用户（install 时生效，默认当前用户）
#    CALUSE_GOPROXY  Go 模块代理（默认 goproxy.cn）
# ============================================================================

APP_NAME="caluse-server"
DEFAULT_PORT=8080
START_TIMEOUT=30

# ---------- 路径解析 ----------
SCRIPT_DIR=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
if [ -n "${CALUSE_BACKEND:-}" ]; then
  BACKEND_DIR=$CALUSE_BACKEND
else
  BACKEND_DIR="$SCRIPT_DIR/../backend"
fi
BACKEND_DIR=$(CDPATH= cd -- "$BACKEND_DIR" 2>/dev/null && pwd || echo "$BACKEND_DIR")
CONFIG_FILE="${CALUSE_CONFIG:-$BACKEND_DIR/config.yaml}"
PID_FILE="$BACKEND_DIR/data/caluse.pid"
LOG_FILE="$BACKEND_DIR/data/caluse.log"
RUN_USER="${CALUSE_USER:-$(id -un 2>/dev/null || echo root)}"

# 从配置文件解析端口（失败则用默认）
if [ -f "$CONFIG_FILE" ]; then
  CFG_PORT=$(sed -n 's/^[[:space:]]*port:[[:space:]]*\([0-9]*\).*/\1/p' "$CONFIG_FILE" | head -1)
  PORT=${CFG_PORT:-$DEFAULT_PORT}
else
  PORT=$DEFAULT_PORT
fi
HEALTH_URL="${CALUSE_HEALTH_URL:-http://127.0.0.1:$PORT/api/health}"

log()  { echo "[caluse] $(date '+%F %T') $*"; }
fail() { echo "[caluse] 错误: $*" >&2; exit 1; }

# ---------- 基础能力 ----------
is_running() {
  [ -f "$PID_FILE" ] || return 1
  PID=$(cat "$PID_FILE" 2>/dev/null)
  case "$PID" in
    ''|*[!0-9]*) return 1 ;;
  esac
  kill -0 "$PID" 2>/dev/null
}

health_ok() {
  if command -v curl >/dev/null 2>&1; then
    curl -sf --max-time 2 "$HEALTH_URL" >/dev/null 2>&1
  elif command -v wget >/dev/null 2>&1; then
    wget -q --timeout=2 -O /dev/null "$HEALTH_URL" 2>/dev/null
  else
    return 1
  fi
}

# ---------- 构建 ----------
cmd_build() {
  if [ ! -d "$BACKEND_DIR" ]; then
    fail "后端目录不存在: $BACKEND_DIR（可用 CALUSE_BACKEND 指定）"
  fi
  cd "$BACKEND_DIR" || fail "无法进入 $BACKEND_DIR"
  if ! command -v go >/dev/null 2>&1; then
    if [ -x "$APP_NAME" ]; then
      log "未检测到 Go，使用已有二进制 $APP_NAME"
      return 0
    fi
    fail "未检测到 Go 工具链（需 Go 1.22+），且不存在预编译二进制。请安装 Go 或将编译好的 caluse-server 放入 $BACKEND_DIR"
  fi
  log "正在编译（GOPROXY=${CALUSE_GOPROXY:-https://goproxy.cn,direct}）..."
  GOPROXY="${CALUSE_GOPROXY:-https://goproxy.cn,direct}" go build -o "$APP_NAME" ./cmd/server || fail "编译失败"
  log "编译完成: $BACKEND_DIR/$APP_NAME"
}

# ---------- 启动 ----------
cmd_start() {
  if is_running; then
    log "CALUSE 已在运行 (PID $PID)，健康检查: $(health_ok && echo 通过 || echo 未响应)"
    return 0
  fi
  cmd_build || exit 1
  cd "$BACKEND_DIR" || fail "无法进入 $BACKEND_DIR"
  mkdir -p data uploads
  log "正在启动 CALUSE（端口 $PORT）..."
  nohup ./"$APP_NAME" -config "$CONFIG_FILE" >>"$LOG_FILE" 2>&1 &
  NEW_PID=$!
  echo "$NEW_PID" > "$PID_FILE"
  i=0
  while [ $i -lt $START_TIMEOUT ]; do
    if health_ok; then
      log "启动成功 (PID $NEW_PID) → http://127.0.0.1:$PORT"
      return 0
    fi
    kill -0 "$NEW_PID" 2>/dev/null || break
    sleep 1
    i=$((i + 1))
  done
  if kill -0 "$NEW_PID" 2>/dev/null; then
    log "进程已启动但健康检查超时，请查看日志: tail -f $LOG_FILE"
    return 0
  fi
  rm -f "$PID_FILE"
  log "启动失败，最近日志:"
  tail -n 8 "$LOG_FILE" 2>/dev/null || true
  return 1
}

# ---------- 停止 ----------
cmd_stop() {
  if ! is_running; then
    log "CALUSE 未在运行"
    rm -f "$PID_FILE"
    return 0
  fi
  log "正在停止 (PID $PID)..."
  kill "$PID" 2>/dev/null
  i=0
  while kill -0 "$PID" 2>/dev/null && [ $i -lt 15 ]; do
    sleep 1
    i=$((i + 1))
  done
  if kill -0 "$PID" 2>/dev/null; then
    log "进程未响应，强制终止"
    kill -9 "$PID" 2>/dev/null
    sleep 1
  fi
  rm -f "$PID_FILE"
  log "已停止"
}

# ---------- 状态 ----------
cmd_status() {
  if is_running; then
    echo "状态   : 运行中 (PID $PID)"
    echo "健康检查: $(health_ok && echo '通过 ✓' || echo '未响应 ✗')"
    echo "地址   : http://127.0.0.1:$PORT"
    echo "日志   : $LOG_FILE"
    return 0
  fi
  echo "状态: 未运行"
  return 1
}

cmd_logs() {
  [ -f "$LOG_FILE" ] || fail "日志文件不存在: $LOG_FILE"
  tail -n 50 -f "$LOG_FILE"
}

# ---------- 安装为系统服务 ----------
cmd_install() {
  if [ "${2:-}" = "--dry-run" ]; then
    DRY_RUN=1
  else
    DRY_RUN=0
    [ "$(id -u)" -eq 0 ] || fail "install 需要 root 权限：sudo $0 install"
  fi

  UNIT=$(cat <<EOF
[Unit]
Description=CALUSE IT Technology Forum
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
User=$RUN_USER
WorkingDirectory=$BACKEND_DIR
ExecStart=$BACKEND_DIR/$APP_NAME -config $CONFIG_FILE
Restart=on-failure
RestartSec=3s
TimeoutStopSec=15s
LimitNOFILE=65535
NoNewPrivileges=true
ProtectSystem=full
PrivateTmp=true
ReadWritePaths=$BACKEND_DIR/data $BACKEND_DIR/uploads

[Install]
WantedBy=multi-user.target
EOF
)

  INITD=$(cat <<EOF
#!/bin/sh
### BEGIN INIT INFO
# Provides:          caluse
# Required-Start:    \$network \$remote_fs
# Required-Stop:     \$network \$remote_fs
# Default-Start:     2 3 4 5
# Default-Stop:      0 1 6
# Short-Description: CALUSE IT forum
# Description:       CALUSE IT technology forum server
### END INIT INFO
CALUSE_BACKEND="$BACKEND_DIR" "$SCRIPT_DIR/caluse.sh" "\$1"
exit \$?
EOF
)

  if [ $DRY_RUN -eq 1 ]; then
    echo "=== 检测结果（dry-run） ==="
    detect_init
    echo "=== systemd unit ==="
    echo "$UNIT"
    echo "=== SysV/OpenRC init 脚本 ==="
    echo "$INITD"
    return 0
  fi

  # 数据目录权限
  mkdir -p "$BACKEND_DIR/data" "$BACKEND_DIR/uploads"
  chown -R "$RUN_USER" "$BACKEND_DIR/data" "$BACKEND_DIR/uploads" 2>/dev/null || true

  # 先停掉手动启动的实例，交给服务管理
  cmd_stop 2>/dev/null || true

  INIT_SYSTEM=$(detect_init)
  case "$INIT_SYSTEM" in
    systemd)
      echo "$UNIT" > /etc/systemd/system/caluse.service
      systemctl daemon-reload
      systemctl enable caluse.service
      systemctl start caluse.service
      log "已安装 systemd 服务并启动（systemctl status caluse）"
      ;;
    openrc)
      echo "$INITD" > /etc/init.d/caluse
      chmod +x /etc/init.d/caluse
      rc-update add caluse default 2>/dev/null || true
      /etc/init.d/caluse start
      log "已安装 OpenRC 服务并启动（rc-service caluse status）"
      ;;
    sysv-deb)
      echo "$INITD" > /etc/init.d/caluse
      chmod +x /etc/init.d/caluse
      update-rc.d caluse defaults
      /etc/init.d/caluse start
      log "已安装 SysV 服务并启动（service caluse status）"
      ;;
    sysv-rpm)
      echo "$INITD" > /etc/init.d/caluse
      chmod +x /etc/init.d/caluse
      chkconfig --add caluse 2>/dev/null || true
      chkconfig caluse on 2>/dev/null || true
      /etc/init.d/caluse start
      log "已安装 SysV 服务并启动（service caluse status）"
      ;;
    *)
      fail "未能识别 init 系统，请手动使用 $0 start 管理进程"
      ;;
  esac

  log "提示: 默认端口 $PORT。若需 80/443 端口请配置反向代理（Nginx/Caddy），并开启 config.yaml 的 trust_proxy"
  if command -v firewall-cmd >/dev/null 2>&1; then
    log "防火墙: firewall-cmd --permanent --add-port=$PORT/tcp && firewall-cmd --reload"
  elif command -v ufw >/dev/null 2>&1; then
    log "防火墙: ufw allow $PORT/tcp"
  fi
}

detect_init() {
  if [ -d /run/systemd/system ] && command -v systemctl >/dev/null 2>&1; then
    echo "systemd"
  elif command -v rc-update >/dev/null 2>&1; then
    echo "openrc"
  elif command -v update-rc.d >/dev/null 2>&1; then
    echo "sysv-deb"
  elif command -v chkconfig >/dev/null 2>&1; then
    echo "sysv-rpm"
  else
    echo "unknown"
  fi
}

cmd_uninstall() {
  [ "$(id -u)" -eq 0 ] || fail "uninstall 需要 root 权限"
  cmd_stop 2>/dev/null || true
  case "$(detect_init)" in
    systemd)
      systemctl disable caluse.service 2>/dev/null || true
      systemctl stop caluse.service 2>/dev/null || true
      rm -f /etc/systemd/system/caluse.service
      systemctl daemon-reload
      ;;
    openrc)
      rc-update del caluse default 2>/dev/null || true
      /etc/init.d/caluse stop 2>/dev/null || true
      rm -f /etc/init.d/caluse
      ;;
    sysv-deb)
      update-rc.d -f caluse remove 2>/dev/null || true
      /etc/init.d/caluse stop 2>/dev/null || true
      rm -f /etc/init.d/caluse
      ;;
    sysv-rpm)
      chkconfig --del caluse 2>/dev/null || true
      /etc/init.d/caluse stop 2>/dev/null || true
      rm -f /etc/init.d/caluse
      ;;
  esac
  log "已卸载系统服务（数据文件保留在 $BACKEND_DIR/data）"
}

# ---------- 入口 ----------
case "${1:-}" in
  start)    cmd_start ;;
  stop)     cmd_stop ;;
  restart)  cmd_stop; cmd_start ;;
  status)   cmd_status ;;
  logs)     cmd_logs ;;
  build)    cmd_build ;;
  install)  cmd_install "$@" ;;
  uninstall) cmd_uninstall ;;
  *)
    echo "CALUSE 网站启动脚本"
    echo ""
    echo "用法: $0 {start|stop|restart|status|logs|build|install|uninstall}"
    echo ""
    echo "  start      启动网站（自动编译 + 健康检查）"
    echo "  stop       停止网站"
    echo "  restart    重启"
    echo "  status     查看运行状态"
    echo "  logs       实时查看日志"
    echo "  build      仅编译后端"
    echo "  install    安装为系统服务（自动识别 systemd/OpenRC/SysV，需 root）"
    echo "  uninstall  卸载系统服务"
    echo ""
    echo "环境变量: CALUSE_BACKEND  CALUSE_CONFIG  CALUSE_USER  CALUSE_GOPROXY"
    exit 1
    ;;
esac
