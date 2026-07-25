// Package webpush 以 VAPID Web Push 實作 domain/push.Sender。
package webpush

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"

	wp "github.com/SherClockHolmes/webpush-go"

	domainpush "github.com/as130232/busy-bee/busy-bee-be/domain/push"
)

type Sender struct {
	publicKey  string
	privateKey string
	// subscriber 為原始 email（或 https URL）。webpush-go 內部會自動補 "mailto:" 前綴，
	// 故此處「絕對不可」自行加 mailto:，否則 JWT sub claim 變成 "mailto:mailto:..."，
	// Chrome/Firefox 容忍但 Apple 嚴格檢查會回 403 BadJwtToken（iOS 推播全數失敗）。
	subscriber string
}

var _ domainpush.Sender = (*Sender)(nil)

func New(publicKey, privateKey, subscriberEmail string) *Sender {
	return &Sender{publicKey: publicKey, privateKey: privateKey, subscriber: subscriberEmail}
}

func (s *Sender) Send(ctx context.Context, sub domainpush.Subscription, msg domainpush.Message) error {
	payload, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("webpush marshal: %w", err)
	}

	resp, err := wp.SendNotificationWithContext(ctx, payload, &wp.Subscription{
		Endpoint: sub.Endpoint,
		Keys:     wp.Keys{P256dh: sub.P256dh, Auth: sub.Auth},
	}, &wp.Options{
		VAPIDPublicKey:  s.publicKey,
		VAPIDPrivateKey: s.privateKey,
		Subscriber:      s.subscriber,
		TTL:             3600,
	})
	if err != nil {
		return fmt.Errorf("webpush send: %w", err)
	}
	defer resp.Body.Close()

	// 診斷用：記錄每次送出的狀態碼、推播服務 host 與回應內文（推播服務常在 body 說明拒絕原因，
	// 例如 VAPID 金鑰雜湊不符）。內文只截前 300 bytes；host 非機密（不含 user token）。
	host := sub.Endpoint
	if u, perr := url.Parse(sub.Endpoint); perr == nil {
		host = u.Host
	}
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 300))
	slog.InfoContext(ctx, "webpush.result", "status", resp.StatusCode, "host", host, "body", string(body))

	return classifyStatus(resp.StatusCode, sub.Endpoint)
}

// classifyStatus 依推播服務回應狀態碼判定結果：
//   - 403 / 404 / 410 → ErrSubscriptionGone：訂閱已失效，caller 應刪除。
//     403（VAPID 金鑰不符）代表此訂閱是用不同的 application server key 建立、對本服務永久無效，
//     故一律清除，避免殭屍訂閱每輪重試把推播服務打到限流（429）。
//   - 其他 >=400（含 429 限流、5xx）→ 可重試錯誤，保留訂閱、下輪再試。
func classifyStatus(code int, endpoint string) error {
	switch {
	case code == http.StatusForbidden || code == http.StatusNotFound || code == http.StatusGone:
		return domainpush.ErrSubscriptionGone{Endpoint: endpoint}
	case code >= 400:
		return fmt.Errorf("webpush status %d", code)
	}
	return nil
}
