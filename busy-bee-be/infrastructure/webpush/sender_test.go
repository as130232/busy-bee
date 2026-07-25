package webpush

import (
	"errors"
	"net/http"
	"strings"
	"testing"

	domainpush "github.com/as130232/busy-bee/busy-bee-be/domain/push"
)

// webpush-go 會自動為 subscriber 補 "mailto:"；若我們自己再補一次，sub claim 會變成
// "mailto:mailto:..."，Apple 回 403 BadJwtToken。此測試守住「不得自行加 mailto:」。
func TestNewSubscriberNotDoublePrefixed(t *testing.T) {
	s := New("pub", "priv", "as130232@gmail.com")
	if strings.HasPrefix(s.subscriber, "mailto:") {
		t.Fatalf("subscriber 不可含 mailto: 前綴（webpush-go 會自動補），got %q", s.subscriber)
	}
	if s.subscriber != "as130232@gmail.com" {
		t.Fatalf("subscriber = %q, want raw email", s.subscriber)
	}
}

func TestClassifyStatus(t *testing.T) {
	const endpoint = "https://push.example/ep-1"

	gone := []int{http.StatusForbidden, http.StatusNotFound, http.StatusGone} // 403/404/410
	for _, code := range gone {
		err := classifyStatus(code, endpoint)
		var g domainpush.ErrSubscriptionGone
		if !errors.As(err, &g) {
			t.Fatalf("status %d: want ErrSubscriptionGone, got %v", code, err)
		}
		if g.Endpoint != endpoint {
			t.Fatalf("status %d: endpoint = %q, want %q", code, g.Endpoint, endpoint)
		}
	}

	// 可重試錯誤（限流 / 伺服器錯誤 / 其他 4xx）不得視為失效訂閱，避免誤刪
	retryable := []int{http.StatusTooManyRequests, http.StatusInternalServerError, http.StatusBadRequest, http.StatusRequestEntityTooLarge}
	for _, code := range retryable {
		err := classifyStatus(code, endpoint)
		if err == nil {
			t.Fatalf("status %d: want error, got nil", code)
		}
		var g domainpush.ErrSubscriptionGone
		if errors.As(err, &g) {
			t.Fatalf("status %d: must NOT be ErrSubscriptionGone", code)
		}
	}

	for _, code := range []int{http.StatusOK, http.StatusCreated, http.StatusAccepted} {
		if err := classifyStatus(code, endpoint); err != nil {
			t.Fatalf("status %d: want nil, got %v", code, err)
		}
	}
}
