package meeting

import (
	"context"
	"strings"
	"testing"

	"github.com/google/uuid"

	domainmeeting "github.com/as130232/busy-bee/busy-bee-be/domain/meeting"
)

func TestImport_CreatesPendingMeetingAndEnqueues(t *testing.T) {
	repo, q := &processFakeRepo{}, &fakeQueue{}
	uc := NewImportUC(repo, q)

	m, err := uc.Execute(context.Background(), uuid.New(), ImportInput{
		URL: "https://youtu.be/abc123", Scenario: "meeting",
	})
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if m.Status != domainmeeting.StatusPending {
		t.Errorf("status = %s, want pending", m.Status)
	}
	if m.SourceURL != "https://youtu.be/abc123" {
		t.Errorf("sourceURL = %q", m.SourceURL)
	}
	if m.Title != importPlaceholderTitle {
		t.Errorf("title = %q, want placeholder (auto from source)", m.Title)
	}
	if !strings.HasSuffix(m.AudioGCSPath, ".m4a") {
		t.Errorf("audioGCSPath = %q, want .m4a", m.AudioGCSPath)
	}
	if q.enqueued != m.ID {
		t.Errorf("enqueued = %v, want %v", q.enqueued, m.ID)
	}
}

func TestImport_CustomTitleKept(t *testing.T) {
	repo, q := &processFakeRepo{}, &fakeQueue{}
	uc := NewImportUC(repo, q)
	m, err := uc.Execute(context.Background(), uuid.New(), ImportInput{
		URL: "https://example.com/ep.mp3", Title: "我的單集",
	})
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if m.Title != "我的單集" {
		t.Errorf("title = %q, want 我的單集", m.Title)
	}
}

func TestImport_InvalidURL(t *testing.T) {
	for _, bad := range []string{"", "   ", "not-a-url", "ftp://x.com/a", "javascript:alert(1)"} {
		repo, q := &processFakeRepo{}, &fakeQueue{}
		uc := NewImportUC(repo, q)
		if _, err := uc.Execute(context.Background(), uuid.New(), ImportInput{URL: bad}); err == nil {
			t.Errorf("expected error for %q", bad)
		}
		if q.enqueued != uuid.Nil {
			t.Errorf("should not enqueue on invalid url %q", bad)
		}
	}
}
