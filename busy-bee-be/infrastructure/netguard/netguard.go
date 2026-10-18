// Package netguard 防止伺服器依使用者提供的網址連到內網（SSRF）：
// 連線前檢查實際要連的 IP，擋 loopback、私有網段、link-local（含 GCP metadata 169.254.169.254）等。
package netguard

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/netip"
	"syscall"
	"time"
)

// ErrBlocked 目標位址不是公開網路位址。
var ErrBlocked = errors.New("netguard: 目標位址不允許連線")

// cgnat 100.64.0.0/10（電信級 NAT，雲端常拿來當內部網段）；netip 沒有內建判斷。
var cgnat = netip.MustParsePrefix("100.64.0.0/10")

// IsPublic 判斷 IP 是否為可連線的公開位址。IPv4-mapped IPv6 先還原成 IPv4 再判斷。
func IsPublic(ip netip.Addr) bool {
	ip = ip.Unmap()
	return ip.IsValid() &&
		!ip.IsUnspecified() &&
		!ip.IsLoopback() &&
		!ip.IsPrivate() &&
		!ip.IsLinkLocalUnicast() &&
		!ip.IsLinkLocalMulticast() &&
		!ip.IsInterfaceLocalMulticast() &&
		!ip.IsMulticast() &&
		!(ip.Is4() && (cgnat.Contains(ip) || ip == netip.AddrFrom4([4]byte{255, 255, 255, 255})))
}

// control 在 DNS 解析之後、真正建立連線之前執行，檢查的是實際要連的 IP；
// 因此轉址後的新連線、DNS rebinding 都會被檢查到。
func control(_, address string, _ syscall.RawConn) error {
	host, _, err := net.SplitHostPort(address)
	if err != nil {
		return fmt.Errorf("%w: %s", ErrBlocked, address)
	}
	ip, err := netip.ParseAddr(host)
	if err != nil || !IsPublic(ip) {
		return fmt.Errorf("%w: %s", ErrBlocked, host)
	}
	return nil
}

// NewClient 回傳只能連公開位址的 http.Client；timeout 為 0 表示不設整體逾時（交給 ctx）。
// 不讀 HTTP_PROXY 等環境變數，避免經 proxy 繞過檢查。
func NewClient(timeout time.Duration) *http.Client {
	dialer := &net.Dialer{Timeout: 30 * time.Second, KeepAlive: 30 * time.Second, Control: control}
	return &http.Client{
		Timeout: timeout,
		Transport: &http.Transport{
			Proxy:                 nil,
			DialContext:           dialer.DialContext,
			ForceAttemptHTTP2:     true,
			MaxIdleConns:          10,
			IdleConnTimeout:       90 * time.Second,
			TLSHandshakeTimeout:   10 * time.Second,
			ExpectContinueTimeout: 1 * time.Second,
		},
	}
}

// CheckHost 解析 host，任一 IP 不是公開位址就回錯。給無法掛 dialer 的外部程式（yt-dlp）事前檢查用；
// 解析與實際連線之間仍有 DNS rebinding 的時間差，只能降低風險。
func CheckHost(ctx context.Context, host string) error {
	if h, _, err := net.SplitHostPort(host); err == nil {
		host = h
	}
	if len(host) > 1 && host[0] == '[' && host[len(host)-1] == ']' {
		host = host[1 : len(host)-1]
	}
	if ip, err := netip.ParseAddr(host); err == nil {
		if !IsPublic(ip) {
			return fmt.Errorf("%w: %s", ErrBlocked, host)
		}
		return nil
	}
	addrs, err := net.DefaultResolver.LookupNetIP(ctx, "ip", host)
	if err != nil {
		return fmt.Errorf("%w: 無法解析 %s", ErrBlocked, host)
	}
	for _, ip := range addrs {
		if !IsPublic(ip) {
			return fmt.Errorf("%w: %s", ErrBlocked, host)
		}
	}
	return nil
}
