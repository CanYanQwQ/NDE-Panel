package socket

import (
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"time"

	"golang.org/x/net/proxy"
)

// testOutboundRequest 测试一条落地(目前支持 socks5)从本节点能不能通、出口 IP 是啥。
type testOutboundRequest struct {
	Type     string `json:"type"`
	Server   string `json:"server"`
	Port     int    `json:"port"`
	Username string `json:"username,omitempty"`
	Password string `json:"password,omitempty"`
}

// handleTestOutbound 经本节点用给定的 socks 落地拨号,抓一个查 IP 接口,回显出口 IP + 延迟。
// 中转的落地常绑前置机 IP 白名单,所以测试必须在前置机上做(而不是面板直连)。
func (w *WebSocketReporter) handleTestOutbound(data interface{}) (map[string]interface{}, error) {
	var req testOutboundRequest
	if data != nil {
		if b, err := json.Marshal(data); err == nil {
			_ = json.Unmarshal(b, &req)
		}
	}
	if req.Server == "" || req.Port == 0 {
		return nil, fmt.Errorf("落地地址或端口为空")
	}
	t := strings.ToLower(req.Type)
	if t != "socks" && t != "socks5" {
		return nil, fmt.Errorf("暂只支持 socks 落地在线测试(%s 落地已校验格式、未在线验证)", req.Type)
	}

	addr := net.JoinHostPort(req.Server, fmt.Sprintf("%d", req.Port))
	var auth *proxy.Auth
	if req.Username != "" {
		auth = &proxy.Auth{User: req.Username, Password: req.Password}
	}
	dialer, err := proxy.SOCKS5("tcp", addr, auth, &net.Dialer{Timeout: 8 * time.Second})
	if err != nil {
		return nil, fmt.Errorf("建 socks 拨号器失败: %v", err)
	}

	transport := &http.Transport{}
	if cd, ok := dialer.(proxy.ContextDialer); ok {
		transport.DialContext = cd.DialContext
	} else {
		transport.Dial = dialer.Dial
	}
	client := &http.Client{Transport: transport, Timeout: 10 * time.Second}

	// 依次试几个纯文本查 IP 接口(HTTP,避开 TLS)
	urls := []string{"http://api.ipify.org", "http://ip-api.com/line/?fields=query", "http://ifconfig.me/ip"}
	start := time.Now()
	var lastErr error
	for _, u := range urls {
		resp, e := client.Get(u)
		if e != nil {
			lastErr = e
			continue
		}
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 128))
		resp.Body.Close()
		ip := strings.TrimSpace(string(body))
		if ip != "" {
			return map[string]interface{}{
				"ok":        true,
				"exitIp":    ip,
				"latencyMs": time.Since(start).Milliseconds(),
			}, nil
		}
	}
	if lastErr == nil {
		lastErr = fmt.Errorf("查 IP 接口无返回")
	}
	return nil, fmt.Errorf("经落地连不通: %v", lastErr)
}
