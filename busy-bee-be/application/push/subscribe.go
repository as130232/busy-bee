// Package push 提供推播訂閱 use cases。
package push

import (
	"context"
	"net/url"
	"strings"

	"github.com/google/uuid"

	domainpush "github.com/as130232/busy-bee/busy-bee-be/domain/push"
	"github.com/as130232/busy-bee/busy-bee-be/pkg/apperr"
	"github.com/as130232/busy-bee/busy-bee-be/pkg/consts/errcode"
)

type SubscribeUC struct {
	repo domainpush.Repository
}

func NewSubscribeUC(repo domainpush.Repository) *SubscribeUC {
	return &SubscribeUC{repo: repo}
}

// pushHosts 允許的推播服務網域（完全相符）；pushHostSuffixes 允許其子網域。
// 伺服器之後會對 endpoint 發請求，只收已知推播服務可避免被當成跳板打內網或任意網站（SSRF）。
var (
	pushHosts        = map[string]bool{"fcm.googleapis.com": true, "updates.push.services.mozilla.com": true}
	pushHostSuffixes = []string{".push.apple.com", ".notify.windows.com"}
)

func allowedEndpoint(endpoint string) bool {
	u, err := url.Parse(endpoint)
	if err != nil || u.Scheme != "https" || u.User != nil || u.Port() != "" {
		return false
	}
	host := strings.ToLower(u.Hostname())
	if pushHosts[host] {
		return true
	}
	for _, suffix := range pushHostSuffixes {
		if strings.HasSuffix(host, suffix) {
			return true
		}
	}
	return false
}

func (uc *SubscribeUC) Subscribe(ctx context.Context, userID uuid.UUID, endpoint, p256dh, auth string) error {
	if endpoint == "" || p256dh == "" || auth == "" {
		return apperr.New(errcode.Param, "subscription")
	}
	if !allowedEndpoint(endpoint) {
		return apperr.New(errcode.Param, "endpoint")
	}
	if _, err := uc.repo.Upsert(ctx, domainpush.Subscription{
		UserID: userID, Endpoint: endpoint, P256dh: p256dh, Auth: auth,
	}); err != nil {
		return apperr.Wrap(err, errcode.Internal)
	}
	return nil
}

func (uc *SubscribeUC) Unsubscribe(ctx context.Context, endpoint string) error {
	if endpoint == "" {
		return apperr.New(errcode.Param, "endpoint")
	}
	if err := uc.repo.DeleteByEndpoint(ctx, endpoint); err != nil {
		return apperr.Wrap(err, errcode.Internal)
	}
	return nil
}
