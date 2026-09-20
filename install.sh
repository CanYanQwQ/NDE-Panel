#!/bin/sh

# 精简 Alpine 没有 bash 时,先用 /bin/sh 自举安装,再重新进入下面原有 Bash 逻辑。
# 普通 VPS 已有 bash 时只多一次 exec,不改变原安装流程。
if [ -z "${BASH_VERSION:-}" ]; then
  BASH_BIN="$(command -v bash 2>/dev/null || true)"
  if [ -z "$BASH_BIN" ]; then
    if command -v apk >/dev/null 2>&1; then
      echo "ℹ️ 未检测到 bash，正在安装脚本运行依赖..."
      apk add --no-cache bash >/dev/null || {
        echo "❌ 无法安装 bash，安装中止。"
        exit 1
      }
      BASH_BIN="$(command -v bash 2>/dev/null || true)"
    else
      echo "❌ 当前系统没有 bash，且无法使用 apk 自动安装。"
      exit 1
    fi
  fi
  exec "$BASH_BIN" "$0" "$@"
fi

# 获取系统架构
get_architecture() {
    ARCH=$(uname -m)
    case $ARCH in
        x86_64)
            echo "amd64"
            ;;
        aarch64|arm64)
            echo "arm64"
            ;;
        *)
            echo "amd64"  # 默认使用 amd64
            ;;
    esac
}

# 构建下载地址
build_download_url() {
    local ARCH=$(get_architecture)
    echo "https://github.com/CanYanQwQ/NDE-Panel/releases/latest/download/gost-${ARCH}"
}

INSTALL_DIR="/etc/gost"
FORCE_CN=0                                       # -c 强制走国内 GitHub 镜像(国内机器 ipinfo 常超时/失败)
GH_MIRROR="${GH_MIRROR:-https://ghfast.top/}"    # 国内 GitHub 加速镜像,可用环境变量覆盖
LOCAL_GOST_BINARY="${LOCAL_GOST_BINARY:-}"       # 可选:使用随脚本上传的本地 gost,避免回退到旧 latest 资产

# 非 systemd 环境(例如 Alpine/Docker)的轻量进程管理文件。
# 正常 VPS 仍然走下面原有的 systemd 服务,不改变原部署行为。
PID_FILE="$INSTALL_DIR/gost.pid"
LOG_FILE="$INSTALL_DIR/gost.log"

has_systemd() {
  command -v systemctl >/dev/null 2>&1 && [ -d /run/systemd/system ]
}

has_openrc() {
  command -v rc-service >/dev/null 2>&1 &&
    command -v rc-update >/dev/null 2>&1 &&
    command -v openrc-run >/dev/null 2>&1 &&
    { [ -d /run/openrc ] || command -v rc-status >/dev/null 2>&1; }
}

has_sysv() {
  local pid1
  pid1=$(ps -p 1 -o comm= 2>/dev/null | tr -d '[:space:]')
  case "$pid1" in
    init|busybox|runit|linuxrc) ;;
    *) return 1 ;;
  esac
  [ -d /etc/init.d ] &&
    { command -v update-rc.d >/dev/null 2>&1 || command -v chkconfig >/dev/null 2>&1; }
}

service_manager() {
  if has_systemd; then
    echo systemd
  elif has_openrc; then
    echo openrc
  elif has_sysv; then
    echo sysv
  else
    echo none
  fi
}

SUPERVISOR_FILE="$INSTALL_DIR/process-supervisor.sh"

write_process_supervisor() {
  cat > "$SUPERVISOR_FILE" <<'EOF'
#!/bin/sh
# Small foreground supervisor used by OpenRC/SysV and Docker entrypoints.
# Arguments: service-name executable [arguments...]
set -u

NAME="${1:-}"
shift || true
if [ -z "$NAME" ] || [ "$#" -eq 0 ]; then
  echo "usage: $0 <name> <command> [args...]" >&2
  exit 2
fi

INSTALL_DIR="/etc/gost"
PID_FILE="$INSTALL_DIR/$NAME.pid"
LOG_FILE="$INSTALL_DIR/$NAME.log"
CHILD=""
STOPPING=0
cd "$INSTALL_DIR" || exit 1

cleanup() {
  STOPPING=1
  if [ -n "$CHILD" ] && kill -0 "$CHILD" 2>/dev/null; then
    kill "$CHILD" 2>/dev/null || true
    sleep 1
    kill -9 "$CHILD" 2>/dev/null || true
  fi
  rm -f "$PID_FILE"
  exit 0
}
trap cleanup INT TERM HUP

while [ "$STOPPING" -eq 0 ]; do
  "$@" >> "$LOG_FILE" 2>&1 &
  CHILD=$!
  printf '%s\n' "$CHILD" > "$PID_FILE"
  wait "$CHILD"
  CODE=$?
  rm -f "$PID_FILE"
  CHILD=""
  [ "$STOPPING" -eq 1 ] && exit 0
  printf '%s supervisor: child exited with code %s, restarting in 3s\n' "$(date -u '+%Y-%m-%dT%H:%M:%SZ')" "$CODE" >> "$LOG_FILE"
  sleep 3
done
EOF
  chmod 755 "$SUPERVISOR_FILE"
}

write_node_entrypoint() {
  local entrypoint="$INSTALL_DIR/entrypoint.sh"
  cat > "$entrypoint" <<'EOF'
#!/bin/sh
# PID 1 entrypoint for a node container without systemd/OpenRC.
set -u
INSTALL_DIR="${GOST_INSTALL_DIR:-/etc/gost}"
GOST_BIN="$INSTALL_DIR/gost"
SINGBOX_BIN="$INSTALL_DIR/sing-box"
SINGBOX_CONFIG="$INSTALL_DIR/sing-box.json"
GOST_LOG="$INSTALL_DIR/gost.log"
GOST_PID_FILE="$INSTALL_DIR/gost.pid"
SINGBOX_LOG="$INSTALL_DIR/sing-box.log"
SINGBOX_PID_FILE="$INSTALL_DIR/sing-box.pid"
GOST_PID=""
SINGBOX_PID=""
STOPPING=0
stop_children() {
  STOPPING=1
  for pid in "$GOST_PID" "$SINGBOX_PID"; do
    if [ -n "$pid" ] && kill -0 "$pid" 2>/dev/null; then kill "$pid" 2>/dev/null || true; fi
  done
  sleep 1
  for pid in "$GOST_PID" "$SINGBOX_PID"; do
    if [ -n "$pid" ] && kill -0 "$pid" 2>/dev/null; then kill -9 "$pid" 2>/dev/null || true; fi
  done
  rm -f "$SINGBOX_PID_FILE"
  rm -f "$GOST_PID_FILE"
}
is_singbox_pid() {
  local pid="$1"
  [ -n "$pid" ] || return 1
  kill -0 "$pid" 2>/dev/null || return 1
  tr '\0' ' ' < "/proc/$pid/cmdline" 2>/dev/null | grep -q 'sing-box'
}
trap 'stop_children; exit 0' INT TERM HUP
if [ ! -x "$GOST_BIN" ]; then echo "gost binary is missing: $GOST_BIN" >&2; exit 1; fi
mkdir -p "$INSTALL_DIR"
cd "$INSTALL_DIR" || exit 1
"$GOST_BIN" >> "$GOST_LOG" 2>&1 &
GOST_PID=$!
printf '%s\n' "$GOST_PID" > "$GOST_PID_FILE"
while [ "$STOPPING" -eq 0 ]; do
  if ! kill -0 "$GOST_PID" 2>/dev/null; then stop_children; exit 1; fi
  if [ -x "$SINGBOX_BIN" ] && [ -s "$SINGBOX_CONFIG" ]; then
    FILE_PID=""
    if [ -f "$SINGBOX_PID_FILE" ]; then
      FILE_PID=$(cat "$SINGBOX_PID_FILE" 2>/dev/null || true)
    fi
    if is_singbox_pid "$FILE_PID"; then
      SINGBOX_PID="$FILE_PID"
    elif ! is_singbox_pid "$SINGBOX_PID"; then
      "$SINGBOX_BIN" run -c "$SINGBOX_CONFIG" >> "$SINGBOX_LOG" 2>&1 &
      SINGBOX_PID=$!
      printf '%s\n' "$SINGBOX_PID" > "$SINGBOX_PID_FILE"
    fi
  fi
  sleep 2
done
stop_children
EOF
  chmod 755 "$entrypoint"
}

write_openrc_service() {
  local name="$1"
  local binary="$2"
  shift 2
  local args="$*"
  cat > "/etc/init.d/$name" <<EOF
#!/sbin/openrc-run
name="$name"
description="$name service"
command="$SUPERVISOR_FILE"
command_args="$name $binary$args"
command_background=true
pidfile="/run/$name-supervisor.pid"
output_log="/etc/gost/$name-supervisor.log"
error_log="/etc/gost/$name-supervisor.log"

depend() {
  need net
  after firewall
}
EOF
  chmod 755 "/etc/init.d/$name"
}

write_sysv_service() {
  local name="$1"
  local binary="$2"
  shift 2
  local args="$*"
  cat > "/etc/init.d/$name" <<EOF
#!/bin/sh
### BEGIN INIT INFO
# Provides:          $name
# Required-Start:    \$remote_fs \$network
# Required-Stop:     \$remote_fs \$network
# Should-Start:      \$named
# Default-Start:     2 3 4 5
# Default-Stop:      0 1 6
# Short-Description: $name service
### END INIT INFO

DAEMON="$SUPERVISOR_FILE"
ARGS="$name $binary$args"
PIDFILE="/run/$name-supervisor.pid"
LOGFILE="/etc/gost/$name-supervisor.log"

start() {
  if [ -f "\$PIDFILE" ] && kill -0 "\$(cat \"\$PIDFILE\")" 2>/dev/null; then
    return 0
  fi
  nohup "\$DAEMON" \$ARGS >>"\$LOGFILE" 2>&1 &
  echo \$! >"\$PIDFILE"
}
stop() {
  if [ -f "\$PIDFILE" ]; then
    kill "\$(cat \"\$PIDFILE\")" 2>/dev/null || true
    rm -f "\$PIDFILE"
  fi
}
status() {
  if [ -f "\$PIDFILE" ] && kill -0 "\$(cat \"\$PIDFILE\")" 2>/dev/null; then
    echo "$name is running"
    return 0
  fi
  echo "$name is stopped"
  return 1
}
case "\${1:-}" in
  start|stop|status) "\$1" ;;
  restart) stop; start ;;
  *) echo "Usage: \$0 {start|stop|status|restart}"; exit 2 ;;
esac
EOF
  chmod 755 "/etc/init.d/$name"
}

install_native_service() {
  local name="$1"
  local binary="$2"
  shift 2
  local args="$*"
  local manager
  manager=$(service_manager)
  case "$manager" in
    openrc)
      write_openrc_service "$name" "$binary" "$args"
      rc-update add "$name" default >/dev/null || return 1
      ;;
    sysv)
      write_sysv_service "$name" "$binary" "$args"
      if command -v update-rc.d >/dev/null 2>&1; then
        update-rc.d "$name" defaults >/dev/null || return 1
      else
        chkconfig --add "$name" >/dev/null 2>&1 || true
        chkconfig --level 2345 "$name" on >/dev/null 2>&1 || return 1
      fi
      ;;
    *)
      return 1
      ;;
  esac
}

start_native_service() {
  local name="$1"
  case "$(service_manager)" in
    openrc) rc-service "$name" start ;;
    sysv) "/etc/init.d/$name" start ;;
    *) return 1 ;;
  esac
}

stop_native_service() {
  local name="$1"
  case "$(service_manager)" in
    openrc) rc-service "$name" stop >/dev/null 2>&1 || true ;;
    sysv) "/etc/init.d/$name" stop >/dev/null 2>&1 || true ;;
    *) return 0 ;;
  esac
}

disable_native_service() {
  local name="$1"
  case "$(service_manager)" in
    openrc) rc-update del "$name" default >/dev/null 2>&1 || true ;;
    sysv)
      if command -v update-rc.d >/dev/null 2>&1; then
        update-rc.d -f "$name" remove >/dev/null 2>&1 || true
      else
        chkconfig --del "$name" >/dev/null 2>&1 || true
      fi
      ;;
    *) ;;
  esac
}

remove_native_service() {
  local name="$1"
  disable_native_service "$name"
  rm -f "/etc/init.d/$name" "/run/$name-supervisor.pid"
}

is_fallback_gost_running() {
  [ -f "$PID_FILE" ] || return 1
  local pid
  pid=$(cat "$PID_FILE" 2>/dev/null)
  [ -n "$pid" ] && kill -0 "$pid" 2>/dev/null
}

stop_fallback_gost() {
  if [ -f "$PID_FILE" ]; then
    local pid
    pid=$(cat "$PID_FILE" 2>/dev/null)
    if [ -n "$pid" ] && kill -0 "$pid" 2>/dev/null; then
      kill "$pid" 2>/dev/null || true
      for _ in 1 2 3 4 5; do
        kill -0 "$pid" 2>/dev/null || break
        sleep 1
      done
      kill -9 "$pid" 2>/dev/null || true
    fi
    rm -f "$PID_FILE"
  fi
}

start_fallback_gost() {
  # systemd 原配置带 WorkingDirectory=/etc/gost;非 systemd 也必须保持同样的 cwd,
  # 否则 gost 会在调用脚本的目录找 config.json,导致启动失败。
  nohup sh -c 'cd "$1" && exec ./gost' _ "$INSTALL_DIR" >> "$LOG_FILE" 2>&1 < /dev/null &
  local pid=$!
  echo "$pid" > "$PID_FILE"
  sleep 1
  if kill -0 "$pid" 2>/dev/null; then
    return 0
  fi
  rm -f "$PID_FILE"
  return 1
}


show_menu() {
  echo "==============================================="
  echo "              管理脚本"
  echo "==============================================="
  echo "请选择操作："
  echo "1. 安装"
  echo "2. 更新"  
  echo "3. 卸载"
  echo "4. 退出"
  echo "==============================================="
}

# 删除脚本自身
delete_self() {
  echo ""
  echo "🗑️ 操作已完成，正在清理脚本文件..."
  SCRIPT_PATH="$(readlink -f "$0" 2>/dev/null || realpath "$0" 2>/dev/null || echo "$0")"
  sleep 1
  rm -f "$SCRIPT_PATH" && echo "✅ 脚本文件已删除" || echo "❌ 删除脚本文件失败"
}

# 检查并安装 tcpkill
check_and_install_tcpkill() {
  # 检查 tcpkill 是否已安装
  if command -v tcpkill &> /dev/null; then
    return 0
  fi
  
  # 检测操作系统类型
  OS_TYPE=$(uname -s)
  
  # 检查是否需要 sudo
  if [[ $EUID -ne 0 ]]; then
    SUDO_CMD="sudo"
  else
    SUDO_CMD=""
  fi
  
  if [[ "$OS_TYPE" == "Darwin" ]]; then
    if command -v brew &> /dev/null; then
      brew install dsniff &> /dev/null
    fi
    return 0
  fi
  
  # 检测 Linux 发行版并安装对应的包
  if [ -f /etc/os-release ]; then
    . /etc/os-release
    DISTRO=$ID
  elif [ -f /etc/redhat-release ]; then
    DISTRO="rhel"
  elif [ -f /etc/debian_version ]; then
    DISTRO="debian"
  else
    return 0
  fi
  
  case $DISTRO in
    ubuntu|debian)
      $SUDO_CMD apt update &> /dev/null
      $SUDO_CMD apt install -y dsniff &> /dev/null
      ;;
    centos|rhel|fedora)
      if command -v dnf &> /dev/null; then
        $SUDO_CMD dnf install -y dsniff &> /dev/null
      elif command -v yum &> /dev/null; then
        $SUDO_CMD yum install -y dsniff &> /dev/null
      fi
      ;;
    alpine)
      $SUDO_CMD apk add --no-cache dsniff &> /dev/null
      ;;
    arch|manjaro)
      $SUDO_CMD pacman -S --noconfirm dsniff &> /dev/null
      ;;
    opensuse*|sles)
      $SUDO_CMD zypper install -y dsniff &> /dev/null
      ;;
    gentoo)
      $SUDO_CMD emerge --ask=n net-analyzer/dsniff &> /dev/null
      ;;
    void)
      $SUDO_CMD xbps-install -Sy dsniff &> /dev/null
      ;;
  esac
  
  return 0
}


# 获取用户输入的配置参数
get_config_params() {
  if [[ -z "$SERVER_ADDR" || -z "$SECRET" ]]; then
    echo "请输入配置参数："
    
    if [[ -z "$SERVER_ADDR" ]]; then
      read -p "服务器地址: " SERVER_ADDR
    fi
    
    if [[ -z "$SECRET" ]]; then
      read -p "密钥: " SECRET
    fi
    
    if [[ -z "$SERVER_ADDR" || -z "$SECRET" ]]; then
      echo "❌ 参数不完整，操作取消。"
      exit 1
    fi
  fi
}

# 解析命令行参数
while getopts "a:s:c" opt; do
  case $opt in
    a) SERVER_ADDR="$OPTARG" ;;
    s) SECRET="$OPTARG" ;;
    c) FORCE_CN=1 ;;
    *) echo "❌ 无效参数"; exit 1 ;;
  esac
done

# 计算 gost 下载地址(国内或 -c 时走镜像;ipinfo 检测加超时,避免无网时卡死)
DOWNLOAD_URL=$(build_download_url)
if [ "$FORCE_CN" = "1" ]; then
  COUNTRY="CN"
else
  COUNTRY=$(curl -s --max-time 5 https://ipinfo.io/country 2>/dev/null || echo "")
fi
if [ "$COUNTRY" = "CN" ]; then
  DOWNLOAD_URL="${GH_MIRROR}${DOWNLOAD_URL}"
  echo "🌏 使用国内镜像: ${GH_MIRROR}"
fi

# 安装功能
install_gost() {
  echo "🚀 开始安装 GOST..."
  get_config_params

    # 检查并安装 tcpkill
  check_and_install_tcpkill
  

  mkdir -p "$INSTALL_DIR"

  # 停止并禁用已有服务
  local manager
  manager=$(service_manager)
  if [ "$manager" = "systemd" ]; then
    if systemctl list-units --full -all | grep -Fq "gost.service"; then
      echo "🔍 检测到已存在的gost服务"
      systemctl stop gost 2>/dev/null && echo "🛑 停止服务"
      systemctl disable gost 2>/dev/null && echo "🚫 禁用自启"
    fi
  elif [ "$manager" = "openrc" ] || [ "$manager" = "sysv" ]; then
    stop_native_service gost
    disable_native_service gost
  else
    # 没有可用服务管理器时,停止本脚本之前启动的后台进程
    stop_fallback_gost
  fi

  # 删除旧文件
  [[ -f "$INSTALL_DIR/gost" ]] && echo "🧹 删除旧文件 gost" && rm -f "$INSTALL_DIR/gost"

  # 下载/复制 gost
  if [[ -n "$LOCAL_GOST_BINARY" && -f "$LOCAL_GOST_BINARY" ]]; then
    echo "📦 使用随脚本上传的本地 gost: $LOCAL_GOST_BINARY"
    cp "$LOCAL_GOST_BINARY" "$INSTALL_DIR/gost"
  else
    echo "⬇️ 下载 gost 中..."
    curl -L "$DOWNLOAD_URL" -o "$INSTALL_DIR/gost"
  fi
  if [[ ! -f "$INSTALL_DIR/gost" || ! -s "$INSTALL_DIR/gost" ]]; then
    echo "❌ gost 获取失败，请检查本地二进制或下载链接。"
    exit 1
  fi
  chmod +x "$INSTALL_DIR/gost"
  echo "✅ 下载完成"

  # 打印版本
  echo "🔎 gost 版本：$($INSTALL_DIR/gost -V)"

  # 写入 config.json (安装时总是创建新的)
  CONFIG_FILE="$INSTALL_DIR/config.json"
  echo "📄 创建新配置: config.json"
  cat > "$CONFIG_FILE" <<EOF
{
  "addr": "$SERVER_ADDR",
  "secret": "$SECRET"
}
EOF

  # 写入 gost.json
  GOST_CONFIG="$INSTALL_DIR/gost.json"
  if [[ -f "$GOST_CONFIG" ]]; then
    echo "⏭️ 跳过配置文件: gost.json (已存在)"
  else
    echo "📄 创建新配置: gost.json"
    cat > "$GOST_CONFIG" <<EOF
{}
EOF
  fi

  # 加强权限
  chmod 600 "$INSTALL_DIR"/*.json

  write_process_supervisor
  write_node_entrypoint
  case "$manager" in
    systemd)
      # 创建 systemd 服务。network-online 避免开机网络尚未就绪时反复失败。
      SERVICE_FILE="/etc/systemd/system/gost.service"
      cat > "$SERVICE_FILE" <<EOF
[Unit]
Description=Gost Proxy Service
Wants=network-online.target
After=network-online.target

[Service]
WorkingDirectory=$INSTALL_DIR
ExecStart=$INSTALL_DIR/gost
Restart=on-failure
RestartSec=3

[Install]
WantedBy=multi-user.target
EOF

      if ! systemctl daemon-reload; then
        echo "❌ systemd 重载失败，未完成安装。"
        return 1
      fi
      if ! systemctl enable gost >/dev/null; then
        echo "❌ gost 开机自启动注册失败，未完成安装。"
        return 1
      fi
      if ! systemctl start gost; then
        echo "❌ gost 服务启动失败，请执行 journalctl -u gost -f"
        return 1
      fi
      echo "✅ 安装完成，gost 已启动并设置为开机启动。"
      echo "📁 配置目录: $INSTALL_DIR"
      echo "🔧 服务状态: $(systemctl is-active gost)"
      ;;
    openrc|sysv)
      if ! install_native_service gost "$INSTALL_DIR/gost"; then
        echo "❌ $manager 服务注册失败，未完成安装。"
        return 1
      fi
      if ! start_native_service gost; then
        echo "❌ $manager gost 服务启动失败，请检查 /etc/gost/gost.log"
        return 1
      fi
      echo "✅ 安装完成，gost 已由 $manager 注册为开机服务并启动。"
      echo "📁 配置目录: $INSTALL_DIR"
      ;;
    *)
      # 没有 init/服务管理器时只能保证当前启动周期,不能伪称支持重启自启。
      echo "⚠️ 未检测到 systemd/OpenRC/SysV，使用当前周期 fallback 启动。"
      echo "⚠️ 此环境重启自启动需要容器 entrypoint 或外部 supervisor。"
      echo "📦 Docker 请使用: ENTRYPOINT [\"/etc/gost/entrypoint.sh\"]"
      if start_fallback_gost; then
        echo "✅ gost 当前已在后台运行。"
        echo "📁 配置目录: $INSTALL_DIR"
        echo "📝 运行日志: $LOG_FILE"
        echo "🆔 进程文件: $PID_FILE"
      else
        echo "❌ gost 启动失败，请查看: $LOG_FILE"
        return 1
      fi
      ;;
  esac
}

# 更新功能
update_gost() {
  echo "🔄 开始更新 GOST..."
  
  if [[ ! -d "$INSTALL_DIR" ]]; then
    echo "❌ GOST 未安装，请先选择安装。"
    return 1
  fi
  
  echo "📥 使用下载地址: $DOWNLOAD_URL"
  
  # 检查并安装 tcpkill
  check_and_install_tcpkill
  
  # 先获取新版本
  if [[ -n "$LOCAL_GOST_BINARY" && -f "$LOCAL_GOST_BINARY" ]]; then
    echo "📦 使用随脚本上传的本地 gost: $LOCAL_GOST_BINARY"
    cp "$LOCAL_GOST_BINARY" "$INSTALL_DIR/gost.new"
  else
    echo "⬇️ 下载最新版本..."
    curl -L "$DOWNLOAD_URL" -o "$INSTALL_DIR/gost.new"
  fi
  if [[ ! -f "$INSTALL_DIR/gost.new" || ! -s "$INSTALL_DIR/gost.new" ]]; then
    echo "❌ gost 获取失败。"
    return 1
  fi

  local manager
  manager=$(service_manager)

  # 停止服务
  if [ "$manager" = "systemd" ]; then
    if systemctl list-units --full -all | grep -Fq "gost.service"; then
      echo "🛑 停止 gost 服务..."
      systemctl stop gost
    fi
  elif [ "$manager" = "openrc" ] || [ "$manager" = "sysv" ]; then
    stop_native_service gost
  else
    stop_fallback_gost
  fi

  # 替换文件
  mv "$INSTALL_DIR/gost.new" "$INSTALL_DIR/gost"
  chmod +x "$INSTALL_DIR/gost"
  
  # 打印版本
  echo "🔎 新版本：$($INSTALL_DIR/gost -V)"

  # 重启服务
  echo "🔄 重启服务..."
  if [ "$manager" = "systemd" ]; then
    if ! systemctl start gost; then
      echo "❌ gost 启动失败，请查看 journalctl -u gost -f"
      return 1
    fi
  elif [ "$manager" = "openrc" ] || [ "$manager" = "sysv" ]; then
    write_process_supervisor
    write_node_entrypoint
    if ! install_native_service gost "$INSTALL_DIR/gost" || ! start_native_service gost; then
      echo "❌ $manager gost 服务启动失败，请查看 $LOG_FILE"
      return 1
    fi
  elif ! start_fallback_gost; then
    echo "❌ gost 启动失败，请查看: $LOG_FILE"
    return 1
  fi

  echo "✅ 更新完成，服务已重新启动。"
}

# 卸载功能
uninstall_gost() {
  echo "🗑️ 开始卸载 GOST..."
  
  read -p "确认卸载 GOST 吗？此操作将删除所有相关文件 (y/N): " confirm
  if [[ "$confirm" != "y" && "$confirm" != "Y" ]]; then
    echo "❌ 取消卸载"
    return 0
  fi

  # 停止并禁用服务
  local manager
  manager=$(service_manager)
  if [ "$manager" = "systemd" ]; then
    if systemctl list-units --full -all | grep -Fq "gost.service"; then
      echo "🛑 停止并禁用 gost 服务..."
      systemctl stop gost 2>/dev/null
      systemctl disable gost 2>/dev/null
    fi
  elif [ "$manager" = "openrc" ] || [ "$manager" = "sysv" ]; then
    remove_native_service gost
  else
    stop_fallback_gost
  fi

  # 协议功能会在本机装 sing-box。按同一个服务管理器清理,不留下开机残留。
  if [ "$manager" = "systemd" ]; then
    if systemctl list-units --full -all | grep -Fq "sing-box.service"; then
      echo "🛑 停止并禁用 sing-box 服务..."
      systemctl stop sing-box 2>/dev/null
      systemctl disable sing-box 2>/dev/null
    fi
  elif [ "$manager" = "openrc" ] || [ "$manager" = "sysv" ]; then
    remove_native_service sing-box
  fi

  # 删除服务文件
  if [[ -f "/etc/systemd/system/gost.service" ]]; then
    rm -f "/etc/systemd/system/gost.service"
    echo "🧹 删除服务文件"
  fi
  if [[ -f "/etc/systemd/system/sing-box.service" ]]; then
    rm -f "/etc/systemd/system/sing-box.service"
    echo "🧹 删除 sing-box 服务文件"
  fi
  # sing-box 的 systemd 覆盖配置(排查重启限流时可能加过)
  rm -rf /etc/systemd/system/sing-box.service.d 2>/dev/null

  # target 的 .wants 里残留的软链接。正常情况 systemctl disable 会删掉,
  # 但服务本身已经异常、或当初是手工 enable 的话就会留下来 ——
  # 结果是 systemctl list-units --all 里一直挂着一条 not-found,看着像没卸干净
  find /etc/systemd /run/systemd \( -name 'gost.service' -o -name 'sing-box.service' \) -delete 2>/dev/null

  # 删除安装目录(gost 二进制、sing-box 二进制、配置、自签证书都在这里)
  if [[ -d "$INSTALL_DIR" ]]; then
    rm -rf "$INSTALL_DIR"
    echo "🧹 删除安装目录: $INSTALL_DIR"
  fi

  # 重载 systemd 并清掉 failed 记录
  if has_systemd; then
    systemctl daemon-reload
    systemctl reset-failed 2>/dev/null
  fi

  echo "✅ 卸载完成(gost + sing-box + 配置 + 证书 已全部清除)"
}

# 主逻辑
main() {
  # 如果提供了命令行参数，直接执行安装
  if [[ -n "$SERVER_ADDR" && -n "$SECRET" ]]; then
    if install_gost; then
      delete_self
      exit 0
    else
      delete_self
      exit 1
    fi
  fi

  # 显示交互式菜单
  while true; do
    show_menu
    read -p "请输入选项 (1-5): " choice
    
    case $choice in
      1)
        if install_gost; then
          delete_self
          exit 0
        else
          delete_self
          exit 1
        fi
        ;;
      2)
        update_gost
        delete_self
        exit 0
        ;;
      3)
        uninstall_gost
        delete_self
        exit 0
        ;;
      4)
        block_protocol
        delete_self
        exit 0
        ;;
      5)
        echo "👋 退出脚本"
        delete_self
        exit 0
        ;;
      *)
        echo "❌ 无效选项，请输入 1-5"
        echo ""
        ;;
    esac
  done
}

# 执行主函数
main