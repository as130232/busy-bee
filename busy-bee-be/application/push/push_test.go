package push

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"

	domainpush "github.com/as130232/busy-bee/busy-bee-be/domain/push"
	"github.com/as130232/busy-bee/busy-bee-be/pkg/apperr"
	"github.com/as130232/busy-bee/busy-bee-be/pkg/consts/errcode"
)

// codeOf 取出 apperr.Error 的業務錯誤碼，供斷言使用；非 apperr 回 -1。
func codeOf(err error) errcode.ErrCode {
	var ae *apperr.Error
	if errors.As(err, &ae) {
		return ae.Code
	}
	return errcode.ErrCode(-1)
}

// fakePushRepo 記錄呼叫並可注入錯誤/預設訂閱。
type fakePushRepo struct {
	subs      []domainpush.Subscription
	upserted  []domainpush.Subscription
	deleted   []string
	upsertErr error
	deleteErr error
	listErr   error
}

func (f *fakePushRepo) Upsert(_ context.Context, sub domainpush.Subscription) (domainpush.Subscription, error) {
	if f.upsertErr != nil {
		return domainpush.Subscription{}, f.upsertErr
	}
	f.upserted = append(f.upserted, sub)
	return sub, nil
}

func (f *fakePushRepo) DeleteByEndpoint(_ context.Context, endpoint string) error {
	if f.deleteErr != nil {
		return f.deleteErr
	}
	f.deleted = append(f.deleted, endpoint)
	return nil
}

func (f *fakePushRepo) ListByUser(_ context.Context, _ uuid.UUID) ([]domainpush.Subscription, error) {
	if f.listErr != nil {
		return nil, f.listErr
	}
	return f.subs, nil
}

// fakeSender 依 endpoint 回傳預設結果（可模擬 Gone / 一般錯誤）。
type fakeSender struct {
	byEndpoint map[string]error
	sent       []string
}

func (f *fakeSender) Send(_ context.Context, sub domainpush.Subscription, _ domainpush.Message) error {
	f.sent = append(f.sent, sub.Endpoint)
	if f.byEndpoint != nil {
		return f.byEndpoint[sub.Endpoint]
	}
	return nil
}

func TestSubscribe_UpsertsSubscription(t *testing.T) {
	repo := &fakePushRepo{}
	uc := NewSubscribeUC(repo)

	if err := uc.Subscribe(context.Background(), uuid.New(), "https://ep", "key", "auth"); err != nil {
		t.Fatalf("Subscribe() error = %v", err)
	}
	if len(repo.upserted) != 1 || repo.upserted[0].Endpoint != "https://ep" {
		t.Errorf("upserted = %v, want one subscription with endpoint", repo.upserted)
	}
}

func TestSubscribe_MissingFieldsParamError(t *testing.T) {
	uc := NewSubscribeUC(&fakePushRepo{})
	cases := []struct{ ep, p, a string }{
		{"", "p", "a"}, {"ep", "", "a"}, {"ep", "p", ""},
	}
	for _, c := range cases {
		err := uc.Subscribe(context.Background(), uuid.New(), c.ep, c.p, c.a)
		if code := codeOf(err); code != errcode.Param {
			t.Errorf("Subscribe(%q,%q,%q) code = %v, want Param", c.ep, c.p, c.a, code)
		}
	}
}

func TestUnsubscribe_DeletesByEndpoint(t *testing.T) {
	repo := &fakePushRepo{}
	uc := NewSubscribeUC(repo)

	if err := uc.Unsubscribe(context.Background(), "https://ep"); err != nil {
		t.Fatalf("Unsubscribe() error = %v", err)
	}
	if len(repo.deleted) != 1 || repo.deleted[0] != "https://ep" {
		t.Errorf("deleted = %v, want [https://ep]", repo.deleted)
	}
}

func TestUnsubscribe_EmptyEndpointParamError(t *testing.T) {
	uc := NewSubscribeUC(&fakePushRepo{})
	if code := codeOf(uc.Unsubscribe(context.Background(), "")); code != errcode.Param {
		t.Errorf("code = %v, want Param", code)
	}
}

func TestSendTest_NoSubscriptionsNotFound(t *testing.T) {
	uc := NewTestUC(&fakePushRepo{}, &fakeSender{})
	_, err := uc.SendTest(context.Background(), uuid.New())
	if code := codeOf(err); code != errcode.NotFound {
		t.Errorf("code = %v, want NotFound", code)
	}
}

func TestSendTest_DeliversAndCountsSuccesses(t *testing.T) {
	repo := &fakePushRepo{subs: []domainpush.Subscription{
		{Endpoint: "ok1"}, {Endpoint: "ok2"},
	}}
	uc := NewTestUC(repo, &fakeSender{})

	delivered, err := uc.SendTest(context.Background(), uuid.New())
	if err != nil {
		t.Fatalf("SendTest() error = %v", err)
	}
	if delivered != 2 {
		t.Errorf("delivered = %d, want 2", delivered)
	}
}

// Gone 端點應被刪除且不計入送達數；一般錯誤只記 log、保留訂閱。
func TestSendTest_GoneSubscriptionCleanedUp(t *testing.T) {
	repo := &fakePushRepo{subs: []domainpush.Subscription{
		{Endpoint: "ok"}, {Endpoint: "gone"}, {Endpoint: "transient"},
	}}
	sender := &fakeSender{byEndpoint: map[string]error{
		"gone":      domainpush.ErrSubscriptionGone{Endpoint: "gone"},
		"transient": errors.New("429 rate limited"),
	}}
	uc := NewTestUC(repo, sender)

	delivered, err := uc.SendTest(context.Background(), uuid.New())
	if err != nil {
		t.Fatalf("SendTest() error = %v", err)
	}
	if delivered != 1 {
		t.Errorf("delivered = %d, want 1 (only 'ok')", delivered)
	}
	if len(repo.deleted) != 1 || repo.deleted[0] != "gone" {
		t.Errorf("deleted = %v, want only [gone] (transient retained)", repo.deleted)
	}
}
