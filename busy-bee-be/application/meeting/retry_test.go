package meeting

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"

	domainmeeting "github.com/as130232/busy-bee/busy-bee-be/domain/meeting"
	"github.com/as130232/busy-bee/busy-bee-be/pkg/apperr"
	"github.com/as130232/busy-bee/busy-bee-be/pkg/consts/errcode"
)

func TestRetry_FailedGoesBackToPendingAndEnqueues(t *testing.T) {
	id, userID := uuid.New(), uuid.New()
	repo := &fakeRepo{
		getResult:    domainmeeting.Meeting{ID: id, UserID: userID, Status: domainmeeting.StatusFailed},
		updateResult: domainmeeting.Meeting{ID: id, Status: domainmeeting.StatusPending},
	}
	q := &fakeQueue{}
	uc := NewRetryUC(repo, q)

	m, err := uc.Execute(context.Background(), userID, id)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if m.Status != domainmeeting.StatusPending {
		t.Errorf("Status = %s, want pending", m.Status)
	}
	if repo.updatedFrom != domainmeeting.StatusFailed || repo.updatedTo != domainmeeting.StatusPending {
		t.Errorf("transition %s->%s, want failed->pending", repo.updatedFrom, repo.updatedTo)
	}
	if q.enqueued != id {
		t.Errorf("enqueued = %v, want %v", q.enqueued, id)
	}
}

func TestRetry_NonFailedConflict(t *testing.T) {
	id, userID := uuid.New(), uuid.New()
	repo := &fakeRepo{getResult: domainmeeting.Meeting{ID: id, Status: domainmeeting.StatusCompleted}}
	uc := NewRetryUC(repo, &fakeQueue{})

	_, err := uc.Execute(context.Background(), userID, id)

	var ae *apperr.Error
	if !errors.As(err, &ae) || ae.Code != errcode.Conflict {
		t.Fatalf("err = %v, want Conflict (only failed meetings can retry)", err)
	}
}

func TestRetry_NotFoundMapped(t *testing.T) {
	repo := &fakeRepo{getErr: domainmeeting.ErrNotFound}
	uc := NewRetryUC(repo, &fakeQueue{})

	_, err := uc.Execute(context.Background(), uuid.New(), uuid.New())

	var ae *apperr.Error
	if !errors.As(err, &ae) || ae.Code != errcode.NotFound {
		t.Fatalf("err = %v, want NotFound", err)
	}
}
