package netguard

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"testing"
)

func TestIsPublic(t *testing.T) {
	blocked := []string{
		"127.0.0.1", "10.1.2.3", "172.16.0.1", "192.168.1.1", "169.254.169.254",
		"100.64.0.1", "0.0.0.0", "224.0.0.1", "255.255.255.255",
		"::1", "fe80::1", "fc00::1", "::", "::ffff:127.0.0.1", "::ffff:169.254.169.254",
	}
	for _, s := range blocked {
		if IsPublic(netip.MustParseAddr(s)) {
			t.Errorf("IsPublic(%s) = true, want false", s)
		}
	}
	for _, s := range []string{"8.8.8.8", "142.250.66.78", "2001:4860:4860::8888"} {
		if !IsPublic(netip.MustParseAddr(s)) {
			t.Errorf("IsPublic(%s) = false, want true", s)
		}
	}
}

// 直接連 loopback 的 httptest server 必須被擋（模擬攻擊者讓伺服器打內網）。
func TestClient_BlocksLoopback(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {}))
	defer srv.Close()

	_, err := NewClient(0).Get(srv.URL)
	if !errors.Is(err, ErrBlocked) {
		t.Fatalf("err = %v, want ErrBlocked", err)
	}
}

func TestCheckHost(t *testing.T) {
	ctx := context.Background()
	for _, h := range []string{"127.0.0.1", "localhost", "169.254.169.254", "[::1]", "metadata.google.internal"} {
		if err := CheckHost(ctx, h); err == nil {
			t.Errorf("CheckHost(%q) = nil, want error", h)
		}
	}
	if err := CheckHost(ctx, "8.8.8.8"); err != nil {
		t.Errorf("CheckHost(8.8.8.8) = %v, want nil", err)
	}
}
