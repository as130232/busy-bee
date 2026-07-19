// Package webpush 以 VAPID Web Push 實作 domain/push.Sender。
package webpush

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	wp "github.com/SherClockHolmes/webpush-go"

	domainpush "github.com/as130232/busy-bee/busy-bee-be/domain/push"
)

type Sender struct {
	publicKey  string
	privateKey string
	subscriber string // mailto:，推播服務要求的聯絡方式
}

var _ domainpush.Sender = (*Sender)(nil)

func New(publicKey, privateKey, subscriberEmail string) *Sender {
	return &Sender{publicKey: publicKey, privateKey: privateKey, subscriber: "mailto:" + subscriberEmail}
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

	switch {
	case resp.StatusCode == http.StatusGone || resp.StatusCode == http.StatusNotFound:
		return domainpush.ErrSubscriptionGone{Endpoint: sub.Endpoint}
	case resp.StatusCode >= 400:
		return fmt.Errorf("webpush status %d", resp.StatusCode)
	}
	return nil
}
