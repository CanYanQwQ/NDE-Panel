package socket

// 合体面板(协议+限速)· 节点端 sing-box 管理模块
// 面板通过 WebSocket 下发 SetSingboxConfig 命令,节点负责:
//   1) 确保 sing-box 外部二进制已安装(没有则下载 v1.13.12,和 s-ui 同版);
//   2) 写入面板生成的完整 sing-box 配置 JSON;
//   3) 按节点环境用 systemd/OpenRC/SysV 起/热重启 sing-box;真正没有 init 的容器由 entrypoint/supervisor 托管。
// sing-box 只在 127.0.0.1 监听,公网口由 gost 转发占用并限速(见 flux合体面板设计.md)。

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
)

const (
	installDir         = "/etc/gost" // 与 install.sh 的 INSTALL_DIR 一致,systemd WorkingDirectory 也是这
	singboxVersion     = "1.13.12"   // 与 s-ui 内嵌的 sing-box 版本对齐,配置格式兼容
	singboxServiceUnit = "/etc/systemd/system/sing-box.service"
	singboxInitScript  = "/etc/init.d/sing-box"
	processSupervisor  = "/etc/gost/process-supervisor.sh"
	singboxPidFile     = "/etc/gost/sing-box.pid"
	singboxLogFile     = "/etc/gost/sing-box.log"
)

// 串行化配置写入 + 重载,避免并发下发时打架
var singboxMu sync.Mutex

// sing-box 的安装进度。存在的理由是「刚建完协议」那一小段:
// sing-box 不是装节点时就有的,而是面板下发配置后才现下 57MB 的二进制,
// 这中间它当然没在跑 —— 面板只看 running 的话会立刻弹血红的
// 「协议全部不可用」,新用户第一眼就以为装坏了,而实际上只是还没装完。
// 报出「正在安装」,面板就能显示成灰色的等待态,只有真失败才转红。
var (
	singboxStateMu    sync.Mutex
	singboxInstalling bool
	// 上一次安装失败的原因。装成功或重新开始安装时清空 ——
	// 留着旧错误会让已经修好的机器一直显示红字。
	singboxInstallErr string
)

func setSingboxInstalling(v bool) {
	singboxStateMu.Lock()
	singboxInstalling = v
	if v {
		singboxInstallErr = ""
	}
	singboxStateMu.Unlock()
}

func setSingboxInstallErr(msg string) {
	singboxStateMu.Lock()
	singboxInstallErr = msg
	singboxStateMu.Unlock()
}

// SingboxProgress 给上报用:是否正在安装、以及上次安装失败的原因(没有则空)
func SingboxProgress() (installing bool, lastErr string) {
	singboxStateMu.Lock()
	defer singboxStateMu.Unlock()
	return singboxInstalling, singboxInstallErr
}

// SetSingboxConfig 命令下发的数据:面板给完整 sing-box 配置 + 可选国内下载镜像
type singboxConfigRequest struct {
	Config json.RawMessage `json:"config"`           // 完整 sing-box 配置(log/inbounds/outbounds…)
	Mirror string          `json:"mirror,omitempty"` // 国内 GitHub 镜像前缀(如 https://ghfast.top/),可空
}

func singboxBinPath() string    { return filepath.Join(installDir, "sing-box") }
func singboxConfigPath() string { return filepath.Join(installDir, "sing-box.json") }

// ---- 命令处理(在 routeCommand 里被调用)----

func (w *WebSocketReporter) handleSetSingboxConfig(data interface{}) error {
	singboxMu.Lock()
	defer singboxMu.Unlock()

	jsonData, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("序列化 sing-box 配置失败: %v", err)
	}
	var req singboxConfigRequest
	if err := json.Unmarshal(jsonData, &req); err != nil {
		return fmt.Errorf("解析 sing-box 配置失败: %v", err)
	}
	if len(req.Config) == 0 {
		return fmt.Errorf("sing-box 配置为空")
	}

	if err := ensureSingboxInstalled(req.Mirror); err != nil {
		return err
	}
	if err := ensureSelfCert(); err != nil { // Hysteria2/TUIC/AnyTLS 用的自签证书,没有则生成
		return err
	}
	if err := writeSingboxConfig(req.Config); err != nil {
		return err
	}
	if err := reloadSingbox(); err != nil {
		return err
	}
	return nil
}

func selfCertPath() string { return filepath.Join(installDir, "certs", "self.crt") }
func selfKeyPath() string  { return filepath.Join(installDir, "certs", "self.key") }

// ensureSelfCert 没有自签证书就用 sing-box 生成一张(Hysteria2/TUIC/AnyTLS 共用,客户端用 insecure=1)。
// 面板侧配置固定引用 /etc/gost/certs/self.crt|self.key。
func ensureSelfCert() error {
	crt := selfCertPath()
	if fi, err := os.Stat(crt); err == nil && fi.Size() > 0 {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(crt), 0o755); err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	out, err := exec.CommandContext(ctx, singboxBinPath(), "generate", "tls-keypair", "www.bing.com", "--months", "120").CombinedOutput()
	if err != nil {
		return fmt.Errorf("生成自签证书失败: %v, %s", err, string(out))
	}
	key, cert := splitPem(string(out))
	if key == "" || cert == "" {
		return fmt.Errorf("解析自签证书失败: %s", string(out))
	}
	if err := os.WriteFile(selfKeyPath(), []byte(key), 0o600); err != nil {
		return err
	}
	if err := os.WriteFile(crt, []byte(cert), 0o644); err != nil {
		return err
	}
	return nil
}

// splitPem 从 `sing-box generate tls-keypair` 输出里拆出 私钥块 和 证书块
func splitPem(out string) (key, cert string) {
	const kb, ke = "-----BEGIN PRIVATE KEY-----", "-----END PRIVATE KEY-----"
	const cb, ce = "-----BEGIN CERTIFICATE-----", "-----END CERTIFICATE-----"
	if i := strings.Index(out, kb); i >= 0 {
		if j := strings.Index(out, ke); j >= 0 {
			key = out[i:j+len(ke)] + "\n"
		}
	}
	if i := strings.Index(out, cb); i >= 0 {
		if j := strings.Index(out, ce); j >= 0 {
			cert = out[i:j+len(ce)] + "\n"
		}
	}
	return key, cert
}

func (w *WebSocketReporter) handleDeleteSingbox(data interface{}) error {
	singboxMu.Lock()
	defer singboxMu.Unlock()
	return stopSingbox()
}

// handleGenerateRealityKeypair 用 sing-box 生成 Reality 密钥对(比在后端手搓 x25519 可靠)
// 返回 {"privateKey": "...", "publicKey": "..."},面板存起来:私钥入服务端配置、公钥进客户端链接。
func (w *WebSocketReporter) handleGenerateRealityKeypair(data interface{}) (map[string]string, error) {
	fmt.Println("🔑 [reality] 收到,等 singboxMu 锁...")
	singboxMu.Lock()
	defer singboxMu.Unlock()
	fmt.Println("🔑 [reality] 已拿锁,检查 sing-box 是否就绪...")

	// 请求可带 mirror,用于首次下载 sing-box 二进制
	var req struct {
		Mirror string `json:"mirror,omitempty"`
	}
	if data != nil {
		if b, err := json.Marshal(data); err == nil {
			_ = json.Unmarshal(b, &req)
		}
	}
	if err := ensureSingboxInstalled(req.Mirror); err != nil {
		fmt.Printf("🔑 [reality] ensureSingboxInstalled 失败: %v\n", err)
		return nil, err
	}
	fmt.Println("🔑 [reality] sing-box 就绪, exec generate reality-keypair...")

	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	out, err := exec.CommandContext(ctx, singboxBinPath(), "generate", "reality-keypair").CombinedOutput()
	fmt.Printf("🔑 [reality] exec 返回 err=%v out=%q\n", err, string(out))
	if err != nil {
		return nil, fmt.Errorf("生成 reality 密钥失败: %v, %s", err, string(out))
	}
	priv, pub := parseRealityKeypair(string(out))
	if priv == "" || pub == "" {
		return nil, fmt.Errorf("解析 reality 密钥失败: %s", string(out))
	}
	fmt.Printf("🔑 [reality] 成功,priv=%d pub=%d 字符\n", len(priv), len(pub))
	return map[string]string{"privateKey": priv, "publicKey": pub}, nil
}

// parseRealityKeypair 解析 `sing-box generate reality-keypair` 的输出:
//
//	PrivateKey: xxxx
//	PublicKey: yyyy
func parseRealityKeypair(out string) (priv, pub string) {
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "PrivateKey:") {
			priv = strings.TrimSpace(strings.TrimPrefix(line, "PrivateKey:"))
		} else if strings.HasPrefix(line, "PublicKey:") {
			pub = strings.TrimSpace(strings.TrimPrefix(line, "PublicKey:"))
		}
	}
	return priv, pub
}

// ---- 安装 / 配置 / 服务管理 ----

// ensureAlpineRuntime 确保 Alpine 能运行官方 linux/amd64 sing-box 二进制。
// 官方包是 CGO disabled 但仍使用 glibc loader;Alpine 默认只有 musl,
// 表面上文件存在,exec 却会报 "no such file or directory"。
// 非 Alpine 完全跳过,不改变 Ubuntu/Debian 等正常 VPS。
func ensureAlpineRuntime() error {
	if _, err := os.Stat("/etc/alpine-release"); err != nil {
		return nil
	}
	if exec.Command("apk", "info", "-e", "gcompat").Run() == nil {
		return nil
	}
	out, err := exec.Command("apk", "add", "--no-cache", "gcompat").CombinedOutput()
	if err != nil {
		return fmt.Errorf("Alpine 安装 gcompat 失败: %v, %s", err, string(out))
	}
	return nil
}

// ensureSingboxInstalled 二进制不存在则下载指定版本并解压
func ensureSingboxInstalled(mirror string) error {
	if err := ensureAlpineRuntime(); err != nil {
		return err
	}
	bin := singboxBinPath()
	if fi, err := os.Stat(bin); err == nil && fi.Size() > 0 {
		return nil
	}

	// 走到这说明二进制不在,真要下载了 —— 从这一刻起面板显示「安装中」
	setSingboxInstalling(true)
	defer setSingboxInstalling(false)

	// 下载和解压当成一件事:任一步失败就换下一个源,
	// 免得国内机留下半个包却只报"解压失败",让人以为是归档坏了
	tmp := filepath.Join(installDir, "sing-box.tar.gz")
	var lastErr error
	for _, url := range singboxDownloadURLs(mirror) {
		if err := downloadFile(url, tmp); err != nil {
			os.Remove(tmp)
			lastErr = fmt.Errorf("%s: %v", url, err)
			fmt.Printf("⚠️ 下载 sing-box 失败,换下一个源: %v\n", lastErr)
			continue
		}
		if err := extractSingboxBinary(tmp, bin); err != nil {
			os.Remove(tmp)
			lastErr = fmt.Errorf("%s 解压失败: %v", url, err)
			fmt.Printf("⚠️ %v,换下一个源\n", lastErr)
			continue
		}
		os.Remove(tmp)
		if err := os.Chmod(bin, 0o755); err != nil {
			return fmt.Errorf("给 sing-box 加执行权限失败: %v", err)
		}
		fmt.Printf("✅ sing-box %s 安装完成(源: %s)\n", singboxVersion, url)
		return nil
	}
	msg := fmt.Sprintf("所有下载源都失败,最后一个 %v", lastErr)
	// 记下来报给面板:否则那台机只会显示「没装上」,而为什么装不上
	// 只能上机器翻 journalctl,这正是几个用户卡住的地方。
	setSingboxInstallErr(msg)
	return fmt.Errorf("%s", msg)
}

// GitHub 加速镜像,给国内机器兜底 —— 拼在完整 github 地址前面即可。
var singboxMirrors = []string{
	"https://ghfast.top/",
	"https://gh-proxy.com/",
	"https://ghproxy.net/",
}

// singboxDownloadURLs 按优先级列出候选下载地址:直连排第一(境外机秒过),
// 连不上或下到一半断流就顺着镜像往下换。
//
// 光靠调用方传 mirror 不够 —— 后台预装那条路径压根没传,国内机必然
// context deadline exceeded,所以兜底放在这里,谁调用都能自愈。
func singboxDownloadURLs(mirror string) []string {
	asset := fmt.Sprintf("sing-box-%s-linux-%s.tar.gz", singboxVersion, runtime.GOARCH)
	origin := fmt.Sprintf("https://github.com/SagerNet/sing-box/releases/download/v%s/%s", singboxVersion, asset)

	urls := make([]string, 0, len(singboxMirrors)+2)
	if mirror != "" {
		urls = append(urls, mirror+origin)
	}
	urls = append(urls, origin)
	for _, m := range singboxMirrors {
		if m != mirror {
			urls = append(urls, m+origin)
		}
	}
	return urls
}

func writeSingboxConfig(cfg json.RawMessage) error {
	if err := os.WriteFile(singboxConfigPath(), cfg, 0o600); err != nil {
		return fmt.Errorf("写 sing-box.json 失败: %v", err)
	}
	return nil
}

func hasSystemd() bool {
	_, err := exec.LookPath("systemctl")
	if err != nil {
		return false
	}
	_, err = os.Stat("/run/systemd/system")
	return err == nil
}

type serviceManager string

const (
	managerSystemd serviceManager = "systemd"
	managerOpenRC  serviceManager = "openrc"
	managerSysV    serviceManager = "sysv"
	managerNone    serviceManager = "none"
)

func hasOpenRC() bool {
	if _, err := exec.LookPath("rc-service"); err != nil {
		return false
	}
	if _, err := exec.LookPath("rc-update"); err != nil {
		return false
	}
	if _, err := exec.LookPath("openrc-run"); err != nil {
		return false
	}
	if _, err := os.Stat("/run/openrc"); err == nil {
		return true
	}
	_, err := exec.LookPath("rc-status")
	return err == nil
}

func hasSysV() bool {
	pid1, err := os.ReadFile("/proc/1/comm")
	if err != nil {
		return false
	}
	switch strings.TrimSpace(string(pid1)) {
	case "init", "busybox", "runit", "linuxrc":
	default:
		return false
	}
	if _, err := os.Stat("/etc/init.d"); err != nil {
		return false
	}
	if _, err := exec.LookPath("update-rc.d"); err == nil {
		return true
	}
	_, err = exec.LookPath("chkconfig")
	return err == nil
}

func currentServiceManager() serviceManager {
	if hasSystemd() {
		return managerSystemd
	}
	if hasOpenRC() {
		return managerOpenRC
	}
	if hasSysV() {
		return managerSysV
	}
	return managerNone
}

const processSupervisorScript = `#!/bin/sh
# Small foreground supervisor used by OpenRC/SysV and Docker entrypoints.
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
`

func ensureProcessSupervisor() error {
	if err := os.MkdirAll(installDir, 0o755); err != nil {
		return err
	}
	if existing, err := os.ReadFile(processSupervisor); err == nil && string(existing) == processSupervisorScript {
		return os.Chmod(processSupervisor, 0o755)
	}
	if err := os.WriteFile(processSupervisor, []byte(processSupervisorScript), 0o755); err != nil {
		return fmt.Errorf("写进程 supervisor 失败: %v", err)
	}
	return nil
}

func nativeSingboxScript(manager serviceManager) string {
	if manager == managerOpenRC {
		return fmt.Sprintf(`#!/sbin/openrc-run
name="sing-box"
description="sing-box service"
command="%s"
command_args="sing-box %s run -c %s"
command_background=true
pidfile="/run/sing-box-supervisor.pid"
output_log="%s"
error_log="%s"
depend() {
  need net
  after firewall
}
`, processSupervisor, singboxBinPath(), singboxConfigPath(), singboxLogFile, singboxLogFile)
	}
	return fmt.Sprintf(`#!/bin/sh
### BEGIN INIT INFO
# Provides:          sing-box
# Required-Start:    $remote_fs $network
# Required-Stop:     $remote_fs $network
# Should-Start:      $named
# Default-Start:     2 3 4 5
# Default-Stop:      0 1 6
# Short-Description: sing-box service
### END INIT INFO
DAEMON="%s"
ARGS="sing-box %s run -c %s"
PIDFILE="/run/sing-box-supervisor.pid"
LOGFILE="%s-supervisor.log"
start() {
  if [ -f "$PIDFILE" ] && kill -0 "$(cat "$PIDFILE")" 2>/dev/null; then return 0; fi
  nohup "$DAEMON" $ARGS >>"$LOGFILE" 2>&1 &
  echo $! >"$PIDFILE"
}
stop() {
  if [ -f "$PIDFILE" ]; then kill "$(cat "$PIDFILE")" 2>/dev/null || true; rm -f "$PIDFILE"; fi
}
status() {
  if [ -f "$PIDFILE" ] && kill -0 "$(cat "$PIDFILE")" 2>/dev/null; then echo "sing-box is running"; return 0; fi
  echo "sing-box is stopped"; return 1
}
case "${1:-}" in
  start|stop|status) "$1" ;;
  restart) stop; start ;;
  *) echo "Usage: $0 {start|stop|status|restart}"; exit 2 ;;
esac
`, processSupervisor, singboxBinPath(), singboxConfigPath(), installDir)
}

func enableNativeService(manager serviceManager, name string) error {
	switch manager {
	case managerOpenRC:
		if out, err := exec.Command("rc-update", "add", name, "default").CombinedOutput(); err != nil {
			return fmt.Errorf("rc-update add %s 失败: %v, %s", name, err, string(out))
		}
	case managerSysV:
		if _, err := exec.LookPath("update-rc.d"); err == nil {
			if out, err := exec.Command("update-rc.d", name, "defaults").CombinedOutput(); err != nil {
				return fmt.Errorf("update-rc.d %s 失败: %v, %s", name, err, string(out))
			}
		} else if out, err := exec.Command("chkconfig", "--add", name).CombinedOutput(); err != nil {
			return fmt.Errorf("chkconfig --add %s 失败: %v, %s", name, err, string(out))
		} else if out, err := exec.Command("chkconfig", "--level", "2345", name, "on").CombinedOutput(); err != nil {
			return fmt.Errorf("chkconfig enable %s 失败: %v, %s", name, err, string(out))
		}
	}
	return nil
}

func startNativeService(manager serviceManager, name string) error {
	if manager == managerOpenRC {
		out, err := exec.Command("rc-service", name, "start").CombinedOutput()
		if err != nil {
			return fmt.Errorf("rc-service %s start 失败: %v, %s", name, err, string(out))
		}
		return nil
	}
	out, err := exec.Command("/etc/init.d/"+name, "start").CombinedOutput()
	if err != nil {
		return fmt.Errorf("/etc/init.d/%s start 失败: %v, %s", name, err, string(out))
	}
	return nil
}

func stopNativeService(manager serviceManager, name string) error {
	if manager == managerOpenRC {
		_, err := exec.Command("rc-service", name, "stop").CombinedOutput()
		return err
	}
	_, err := exec.Command("/etc/init.d/"+name, "stop").CombinedOutput()
	return err
}

func disableNativeService(manager serviceManager, name string) error {
	if manager == managerOpenRC {
		_, err := exec.Command("rc-update", "del", name, "default").CombinedOutput()
		return err
	}
	if _, err := exec.LookPath("update-rc.d"); err == nil {
		_, err = exec.Command("update-rc.d", "-f", name, "remove").CombinedOutput()
		return err
	}
	_, err := exec.Command("chkconfig", "--del", name).CombinedOutput()
	return err
}

func ensureNativeSingboxService(manager serviceManager) error {
	if manager != managerOpenRC && manager != managerSysV {
		return fmt.Errorf("不支持的非 systemd 服务管理器: %s", manager)
	}
	if err := ensureProcessSupervisor(); err != nil {
		return err
	}
	if err := os.WriteFile(singboxInitScript, []byte(nativeSingboxScript(manager)), 0o755); err != nil {
		return fmt.Errorf("写 sing-box init 脚本失败: %v", err)
	}
	return enableNativeService(manager, "sing-box")
}

func singboxProcessRunning() bool {
	data, err := os.ReadFile(singboxPidFile)
	if err != nil {
		return false
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil || pid <= 0 {
		return false
	}
	proc, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	return proc.Signal(syscall.Signal(0)) == nil
}

func stopSingboxFallback() error {
	data, err := os.ReadFile(singboxPidFile)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err == nil && pid > 0 {
		if proc, findErr := os.FindProcess(pid); findErr == nil {
			_ = proc.Signal(syscall.SIGTERM)
			for i := 0; i < 20; i++ {
				if !singboxProcessRunning() {
					break
				}
				time.Sleep(100 * time.Millisecond)
			}
			if singboxProcessRunning() {
				_ = proc.Signal(syscall.SIGKILL)
			}
		}
	}
	return os.Remove(singboxPidFile)
}

func startSingboxFallback() error {
	_ = stopSingboxFallback()
	logFile, err := os.OpenFile(singboxLogFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return fmt.Errorf("打开 sing-box 日志失败: %v", err)
	}
	cmd := exec.Command(singboxBinPath(), "run", "-c", singboxConfigPath())
	cmd.Dir = installDir
	cmd.Stdout = logFile
	cmd.Stderr = logFile
	if err := cmd.Start(); err != nil {
		_ = logFile.Close()
		return fmt.Errorf("启动 sing-box 失败: %v", err)
	}
	if err := os.WriteFile(singboxPidFile, []byte(strconv.Itoa(cmd.Process.Pid)), 0o644); err != nil {
		_ = cmd.Process.Kill()
		_ = logFile.Close()
		return fmt.Errorf("写 sing-box pid 文件失败: %v", err)
	}

	done := make(chan error, 1)
	go func() {
		done <- cmd.Wait()
		_ = logFile.Close()
		_ = os.Remove(singboxPidFile)
	}()
	select {
	case err := <-done:
		return fmt.Errorf("sing-box 启动后立即退出: %v,详见 %s", err, singboxLogFile)
	case <-time.After(500 * time.Millisecond):
		return nil
	}
}

// reloadSingbox 确保 sing-box 服务存在并(热)重启
func reloadSingbox() error {
	manager := currentServiceManager()
	switch manager {
	case managerSystemd:
		if err := ensureSingboxService(); err != nil {
			return err
		}
		if out, err := exec.Command("systemctl", "enable", "sing-box").CombinedOutput(); err != nil {
			return fmt.Errorf("systemctl enable sing-box 失败: %v, %s", err, string(out))
		}
		if out, err := exec.Command("systemctl", "restart", "sing-box").CombinedOutput(); err != nil {
			return fmt.Errorf("重启 sing-box 失败: %v, %s", err, string(out))
		}
		return nil
	case managerOpenRC, managerSysV:
		if err := ensureNativeSingboxService(manager); err != nil {
			return err
		}
		return startNativeService(manager, "sing-box")
	default:
		fmt.Println("⚠️ 未检测到服务管理器，sing-box 仅在当前启动周期运行；容器请使用节点 entrypoint/supervisor。")
		return startSingboxFallback()
	}
}

func stopSingbox() error {
	manager := currentServiceManager()
	switch manager {
	case managerSystemd:
		_ = exec.Command("systemctl", "stop", "sing-box").Run()
		_ = exec.Command("systemctl", "disable", "sing-box").Run()
		return nil
	case managerOpenRC, managerSysV:
		_ = stopNativeService(manager, "sing-box")
		_ = disableNativeService(manager, "sing-box")
		return nil
	default:
		return stopSingboxFallback()
	}
}

// ensureSingboxService 写入/更新 systemd 单元(带 Restart=on-failure 自愈)
func ensureSingboxService() error {
	unit := fmt.Sprintf(`[Unit]
Description=sing-box (flux hybrid)
Wants=network-online.target
After=network-online.target
StartLimitIntervalSec=0

[Service]
WorkingDirectory=%s
ExecStart=%s run -c %s
Restart=on-failure
RestartSec=3
LimitNOFILE=1048576

[Install]
WantedBy=multi-user.target
`, installDir, singboxBinPath(), singboxConfigPath())

	if existing, err := os.ReadFile(singboxServiceUnit); err != nil || string(existing) != unit {
		if err := os.WriteFile(singboxServiceUnit, []byte(unit), 0o644); err != nil {
			return fmt.Errorf("写 sing-box.service 失败: %v", err)
		}
	}
	if out, err := exec.Command("systemctl", "daemon-reload").CombinedOutput(); err != nil {
		return fmt.Errorf("systemctl daemon-reload 失败: %v, %s", err, string(out))
	}
	return nil
}

// ---- 下载 / 解压工具 ----

func downloadFile(url, dest string) error {
	// 总超时给足(二进制十几兆,慢线路也得下完),但连不上/服务端不吭声要快速失败,
	// 否则国内机会在 GitHub 那一个源上干等十分钟,轮不到后面的镜像
	client := &http.Client{
		Timeout: 10 * time.Minute,
		Transport: &http.Transport{
			DialContext:           (&net.Dialer{Timeout: 15 * time.Second}).DialContext,
			TLSHandshakeTimeout:   15 * time.Second,
			ResponseHeaderTimeout: 30 * time.Second,
		},
	}
	resp, err := client.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	out, err := os.Create(dest)
	if err != nil {
		return err
	}
	defer out.Close()
	if _, err := io.Copy(out, resp.Body); err != nil {
		return err
	}
	return nil
}

// extractSingboxBinary 从 sing-box release 的 tar.gz 里抽出 sing-box 二进制
// 归档结构形如 sing-box-1.13.12-linux-amd64/sing-box
func extractSingboxBinary(tarGzPath, dest string) error {
	f, err := os.Open(tarGzPath)
	if err != nil {
		return err
	}
	defer f.Close()

	gz, err := gzip.NewReader(f)
	if err != nil {
		return err
	}
	defer gz.Close()

	tr := tar.NewReader(gz)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		if hdr.Typeflag == tar.TypeReg && filepath.Base(hdr.Name) == "sing-box" {
			out, err := os.Create(dest)
			if err != nil {
				return err
			}
			defer out.Close()
			if _, err := io.Copy(out, tr); err != nil {
				return err
			}
			return nil
		}
	}
	return fmt.Errorf("归档里没找到 sing-box 二进制")
}
