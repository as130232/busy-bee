package meeting

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	domainmeeting "github.com/as130232/busy-bee/busy-bee-be/domain/meeting"
	domainpush "github.com/as130232/busy-bee/busy-bee-be/domain/push"
)

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
}

func (f *fakeSender) Send(_ context.Context, sub domainpush.Subscription, _ domainpush.Message) error {
	if err, ok := f.failOn[sub.Endpoint]; ok {
		return err
	}
	f.sent = append(f.sent, sub.Endpoint)
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
	uc := NewReminderUC(repo, pushRepo, sender)

	uc.SweepOnce(context.Background())

	if len(sender.sent) != 2 {
		t.Errorf("sent = %v, want both endpoints", sender.sent)
	}
	if len(repo.reminded) != 1 || repo.reminded[0] != m.ID {
		t.Errorf("reminded = %v, want [%v]", repo.reminded, m.ID)
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
	uc := NewReminderUC(repo, pushRepo, sender)

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
	uc := NewReminderUC(repo, &fakePushRepo{}, &fakeSender{})

	uc.SweepOnce(context.Background())

	if len(repo.reminded) != 1 {
		t.Error("should mark reminded even without subscriptions")
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
	uc := NewReminderUC(repo, pushRepo, sender)

	uc.SweepOnce(context.Background())

	if len(repo.reminded) != 0 {
		t.Error("transient failure should leave meeting unmarked for retry")
	}
}
