package stt

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	domainlive "github.com/as130232/busy-bee/busy-bee-be/domain/live"
)

const (
	deepgramGrantURL = "https://api.deepgram.com/v1/auth/grant"
	// grantTTLSeconds 短效金鑰只用來開串流連線（連上後不再驗證），30 秒足夠且外洩風險低。
	grantTTLSeconds = 30
)

// DeepgramGrant 以主金鑰向 Deepgram 換取短效 JWT（usage::write），供瀏覽器直連串流辨識（F-LIVE）。
// 主金鑰需為 Member 以上權限。
type DeepgramGrant struct {
	apiKey string
	url    string
	http   *http.Client
}

var _ domainlive.TokenIssuer = (*DeepgramGrant)(nil)

type GrantOption func(*DeepgramGrant)

// WithGrantURL 測試用：改打 httptest server。
func WithGrantURL(u string) GrantOption { return func(g *DeepgramGrant) { g.url = u } }

func NewDeepgramGrant(apiKey string, opts ...GrantOption) *DeepgramGrant {
	g := &DeepgramGrant{apiKey: apiKey, url: deepgramGrantURL, http: &http.Client{Timeout: 10 * time.Second}}
	for _, o := range opts {
		o(g)
	}
	return g
}

func (g *DeepgramGrant) Issue(ctx context.Context) (domainlive.Token, error) {
	body, _ := json.Marshal(map[string]any{"ttl_seconds": grantTTLSeconds})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, g.url, bytes.NewReader(body))
	if err != nil {
		return domainlive.Token{}, fmt.Errorf("deepgram grant: %w", err)
	}
	req.Header.Set("Authorization", "Token "+g.apiKey)
	req.Header.Set("Content-Type", "application/json")
	resp, err := g.http.Do(req)
	if err != nil {
		return domainlive.Token{}, fmt.Errorf("deepgram grant: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		_, _ = io.Copy(io.Discard, resp.Body)
		return domainlive.Token{}, fmt.Errorf("deepgram grant: status %d", resp.StatusCode)
	}
	var out struct {
		AccessToken string  `json:"access_token"`
		ExpiresIn   float64 `json:"expires_in"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return domainlive.Token{}, fmt.Errorf("deepgram grant decode: %w", err)
	}
	if out.AccessToken == "" {
		return domainlive.Token{}, fmt.Errorf("deepgram grant: empty access_token")
	}
	return domainlive.Token{AccessToken: out.AccessToken, ExpiresIn: int(out.ExpiresIn)}, nil
}
