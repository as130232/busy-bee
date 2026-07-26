// Package webpush 以 VAPID Web Push 實作 domain/push.Sender。
package webpush

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	wp "github.com/SherClockHolmes/webpush-go"

	domainpush "github.com/as130232/busy-bee/busy-bee-be/domain/push"
)

// sendTimeout 單則推播的 HTTP 逾時上界。提醒掃描不經 worker 佇列（無 per-task deadline），
// 若推播服務 hang 住會拖垮整輪掃描，故在此設 per-call 上界。
const sendTimeout = 15 * time.Second

type Sender struct {
	publicKey  string
	privateKey string
	// subscriber 為原始 email（或 https URL）。webpush-go 內部會自動補 "mailto:" 前綴，
	// 故此處「絕對不可」自行加 mailto:，否則 JWT sub claim 變成 "mailto:mailto:..."，
	// Chrome/Firefox 容忍但 Apple 嚴格檢查會回 403 BadJwtToken（iOS 推播全數失敗）。
	subscriber string
	httpClient *http.Client
}

var _ domainpush.Sender = (*Sender)(nil)

func New(publicKey, privateKey, subscriberEmail string) *Sender {
	return &Sender{
		publicKey:  publicKey,
		privateKey: privateKey,
		subscriber: subscriberEmail,
		httpClient: &http.Client{Timeout: sendTimeout},
	}
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
		HTTPClient:      s.httpClient,
		VAPIDPublicKey:  s.publicKey,
		VAPIDPrivateKey: s.privateKey,
		Subscriber:      s.subscriber,
		TTL:             3600,
	})
	if err != nil {
		return fmt.Errorf("webpush send: %w", err)
	}
	defer resp.Body.Close()

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
