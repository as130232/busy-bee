package meeting

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	domainactionitem "github.com/as130232/busy-bee/busy-bee-be/domain/actionitem"
	domainmeeting "github.com/as130232/busy-bee/busy-bee-be/domain/meeting"
	domainpush "github.com/as130232/busy-bee/busy-bee-be/domain/push"
)

type fakeActionReminderRepo struct {
	due      []domainactionitem.PendingItem
	reminded []uuid.UUID
}

func (f *fakeActionReminderRepo) ListDueReminders(_ context.Context) ([]domainactionitem.PendingItem, error) {
	return f.due, nil
}

func (f *fakeActionReminderRepo) MarkReminded(_ context.Context, id uuid.UUID) error {
	f.reminded = append(f.reminded, id)
	return nil
}

type fakeReminderRepo struct {
	due      []domainmeeting.Meeting
	reminded []uuid.UUID
}

func (f *fakeReminderRepo) ListDueReminders(_ context.Context) ([]domainmeeting.Meeting, error) {
	return f.due, nil
}

func (f *fakeReminderRepo) MarkReminded(_ context.Context, id uuid.UUID) error {
	f.reminded = append(f.reminded, id)
	return nil
}

type fakePushRepo struct {
	subs    map[uuid.UUID][]domainpush.Subscription
	deleted []string
}

func (f *fakePushRepo) Upsert(_ context.Context, s domainpush.Subscription) (domainpush.Subscription, error) {
	return s, nil
}

func (f *fakePushRepo) DeleteByEndpoint(_ context.Context, endpoint string) error {
	f.deleted = append(f.deleted, endpoint)
	return nil
}

func (f *fakePushRepo) ListByUser(_ context.Context, userID uuid.UUID) ([]domainpush.Subscription, error) {
	return f.subs[userID], nil
}

type fakeSender struct {
	sent    []string // endpoints
	failOn  map[string]error
	lastMsg domainpush.Message
}

func (f *fakeSender) Send(_ context.Context, sub domainpush.Subscription, msg domainpush.Message) error {
	if err, ok := f.failOn[sub.Endpoint]; ok {
		return err
	}
	f.sent = append(f.sent, sub.Endpoint)
	f.lastMsg = msg
	return nil
}

func dueMeeting(userID uuid.UUID) domainmeeting.Meeting {
	at := time.Now().Add(10 * time.Minute)
	return domainmeeting.Meeting{
		ID: uuid.New(), UserID: userID, Title: "架構週會",
		Status: domainmeeting.StatusScheduled, ScheduledAt: &at, RemindBeforeMin: 15,
	}
}

func TestReminder_SendsToAllUserSubsAndMarks(t *testing.T) {
	userID := uuid.New()
	m := dueMeeting(userID)
	repo := &fakeReminderRepo{due: []domainmeeting.Meeting{m}}
	pushRepo := &fakePushRepo{subs: map[uuid.UUID][]domainpush.Subscription{
		userID: {{Endpoint: "ep1", UserID: userID}, {Endpoint: "ep2", UserID: userID}},
	}}
	sender := &fakeSender{}
	uc := NewReminderUC(repo, pushRepo, sender, nil)

	uc.SweepOnce(context.Background())

	if len(sender.sent) != 2 {
		t.Errorf("sent = %v, want both endpoints", sender.sent)
	}
	if len(repo.reminded) != 1 || repo.reminded[0] != m.ID {
		t.Errorf("reminded = %v, want [%v]", repo.reminded, m.ID)
	}
	// 深連結：點通知進 App 並帶 record 參數（前端據此高亮錄音鈕）
	if sender.lastMsg.URL != "/?record=1" {
		t.Errorf("reminder URL = %q, want /?record=1", sender.lastMsg.URL)
	}
}

func TestReminder_GoneSubscriptionDeleted(t *testing.T) {
	userID := uuid.New()
	m := dueMeeting(userID)
	repo := &fakeReminderRepo{due: []domainmeeting.Meeting{m}}
	pushRepo := &fakePushRepo{subs: map[uuid.UUID][]domainpush.Subscription{
		userID: {{Endpoint: "dead", UserID: userID}, {Endpoint: "alive", UserID: userID}},
	}}
	sender := &fakeSender{failOn: map[string]error{
		"dead": domainpush.ErrSubscriptionGone{Endpoint: "dead"},
	}}
	uc := NewReminderUC(repo, pushRepo, sender, nil)

	uc.SweepOnce(context.Background())

	if len(pushRepo.deleted) != 1 || pushRepo.deleted[0] != "dead" {
		t.Errorf("deleted = %v, want [dead]", pushRepo.deleted)
	}
	if len(sender.sent) != 1 || sender.sent[0] != "alive" {
		t.Errorf("sent = %v, want [alive]", sender.sent)
	}
	if len(repo.reminded) != 1 {
		t.Error("meeting should still be marked reminded")
	}
}

func TestReminder_NoSubsStillMarks(t *testing.T) {
	// 用戶沒訂閱也要標記，避免每分鐘重掃同一場會議
	m := dueMeeting(uuid.New())
	repo := &fakeReminderRepo{due: []domainmeeting.Meeting{m}}
	uc := NewReminderUC(repo, &fakePushRepo{}, &fakeSender{}, nil)

	uc.SweepOnce(context.Background())

	if len(repo.reminded) != 1 {
		t.Error("should mark reminded even without subscriptions")
	}
}

func TestReminder_ActionItemDueSendsAndMarks(t *testing.T) {
	// 到期行動項推播「待辦提醒」並標記已提醒，深連結指向所屬會議。
	userID := uuid.New()
	meetingID := uuid.New()
	item := domainactionitem.PendingItem{
		ActionItem: domainactionitem.ActionItem{
			ID: uuid.New(), MeetingID: meetingID, UserID: userID, Description: "交付登入規格",
		},
		MeetingTitle: "架構週會",
	}
	arepo := &fakeActionReminderRepo{due: []domainactionitem.PendingItem{item}}
	pushRepo := &fakePushRepo{subs: map[uuid.UUID][]domainpush.Subscription{
		userID: {{Endpoint: "ep1", UserID: userID}},
	}}
	sender := &fakeSender{}
	uc := NewReminderUC(&fakeReminderRepo{}, pushRepo, sender, arepo)

	uc.SweepOnce(context.Background())

	if len(sender.sent) != 1 {
		t.Errorf("sent = %v, want 1", sender.sent)
	}
	if len(arepo.reminded) != 1 || arepo.reminded[0] != item.ID {
		t.Errorf("reminded = %v, want [%v]", arepo.reminded, item.ID)
	}
	if sender.lastMsg.Title != "待辦提醒" {
		t.Errorf("title = %q, want 待辦提醒", sender.lastMsg.Title)
	}
	if sender.lastMsg.URL != "/meetings/"+meetingID.String() {
		t.Errorf("URL = %q, want /meetings/%s", sender.lastMsg.URL, meetingID)
	}
}

func TestReminder_ActionItemTransientDoesNotMark(t *testing.T) {
	// 全部暫時性失敗 → 不標記，下一輪重試（與會議提醒一致）。
	userID := uuid.New()
	item := domainactionitem.PendingItem{
		ActionItem: domainactionitem.ActionItem{ID: uuid.New(), MeetingID: uuid.New(), UserID: userID, Description: "x"},
	}
	arepo := &fakeActionReminderRepo{due: []domainactionitem.PendingItem{item}}
	pushRepo := &fakePushRepo{subs: map[uuid.UUID][]domainpush.Subscription{
		userID: {{Endpoint: "ep1", UserID: userID}},
	}}
	sender := &fakeSender{failOn: map[string]error{"ep1": errors.New("503")}}
	uc := NewReminderUC(&fakeReminderRepo{}, pushRepo, sender, arepo)

	uc.SweepOnce(context.Background())

	if len(arepo.reminded) != 0 {
		t.Error("transient failure should leave action item unmarked for retry")
	}
}

func TestReminder_TransientSendErrorDoesNotMark(t *testing.T) {
	// 全部發送失敗（暫時性）→ 不標記，下一輪掃描重試
	userID := uuid.New()
	m := dueMeeting(userID)
	repo := &fakeReminderRepo{due: []domainmeeting.Meeting{m}}
	pushRepo := &fakePushRepo{subs: map[uuid.UUID][]domainpush.Subscription{
		userID: {{Endpoint: "ep1", UserID: userID}},
	}}
	sender := &fakeSender{failOn: map[string]error{"ep1": errors.New("503 from push service")}}
	uc := NewReminderUC(repo, pushRepo, sender, nil)

	uc.SweepOnce(context.Background())

	if len(repo.reminded) != 0 {
		t.Error("transient failure should leave meeting unmarked for retry")
	}
}
