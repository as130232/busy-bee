package meeting

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	appmeeting "github.com/as130232/busy-bee/busy-bee-be/application/meeting"
	domainmeeting "github.com/as130232/busy-bee/busy-bee-be/domain/meeting"
	domainuser "github.com/as130232/busy-bee/busy-bee-be/domain/user"
)

type fakeRepo struct{}

func (f *fakeRepo) Create(_ context.Context, m domainmeeting.Meeting) (domainmeeting.Meeting, error) {
	return m, nil
}
func (f *fakeRepo) GetForUser(_ context.Context, id, _ uuid.UUID) (domainmeeting.Meeting, error) {
	return domainmeeting.Meeting{ID: id, Status: domainmeeting.StatusScheduled, AudioGCSPath: "audio/a.webm"}, nil
}
func (f *fakeRepo) UpdateStatus(_ context.Context, id uuid.UUID, _, to domainmeeting.Status) (domainmeeting.Meeting, error) {
	return domainmeeting.Meeting{ID: id, Status: to}, nil
}
func (f *fakeRepo) Get(_ context.Context, id uuid.UUID) (domainmeeting.Meeting, error) {
	return domainmeeting.Meeting{ID: id}, nil
}
func (f *fakeRepo) SaveTranscript(_ context.Context, id uuid.UUID, _ string, _ int) (domainmeeting.Meeting, error) {
	return domainmeeting.Meeting{ID: id}, nil
}
func (f *fakeRepo) SetCompleted(_ context.Context, id uuid.UUID) (domainmeeting.Meeting, error) {
	return domainmeeting.Meeting{ID: id}, nil
}
func (f *fakeRepo) SetFailed(_ context.Context, id uuid.UUID, _ string) (domainmeeting.Meeting, error) {
	return domainmeeting.Meeting{ID: id}, nil
}

type fakeStorage struct{}

func (f *fakeStorage) SignedUploadURL(_ context.Context, _, _ string, _ int64) (domainmeeting.UploadTarget, error) {
	return domainmeeting.UploadTarget{URL: "https://signed", Headers: map[string]string{"Content-Type": "audio/webm"}}, nil
}
func (f *fakeStorage) Exists(_ context.Context, _ string) (bool, error) { return true, nil }
func (f *fakeStorage) Download(_ context.Context, _ string) (io.ReadCloser, int64, error) {
	return io.NopCloser(strings.NewReader("")), 0, nil
}

type fakeQueue struct{}

func (f *fakeQueue) EnqueueProcessMeeting(_ context.Context, _ uuid.UUID) error { return nil }

func testRouter(t *testing.T) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	repo, st, q := &fakeRepo{}, &fakeStorage{}, &fakeQueue{}
	h := NewHandler(appmeeting.NewCreateUC(repo, st), appmeeting.NewCompleteUploadUC(repo, st, q))

	e := gin.New()
	injectIdentity := func(c *gin.Context) {
		ctx := domainuser.WithIdentity(c.Request.Context(), domainuser.Identity{UID: "u1"})
		c.Request = c.Request.WithContext(ctx)
	}
	// handler 依賴 ctx 內的 userID；測試用固定 user
	e.POST("/meetings", injectIdentity, withTestUserID(), h.Create)
	e.POST("/meetings/:id/complete-upload", injectIdentity, withTestUserID(), h.CompleteUpload)
	return e
}

var testUserID = uuid.New()

func withTestUserID() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Request = c.Request.WithContext(domainuser.WithID(c.Request.Context(), testUserID))
	}
}

func TestCreate_ReturnsMeetingAndUpload(t *testing.T) {
	e := testRouter(t)
	w := httptest.NewRecorder()
	body := `{"title":"架構討論","contentType":"audio/webm"}`
	req := httptest.NewRequest(http.MethodPost, "/meetings", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	e.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", w.Code, w.Body.String())
	}
	var resp struct {
		Data struct {
			Meeting struct {
				ID     string `json:"id"`
				Status string `json:"status"`
			} `json:"meeting"`
			Upload struct {
				URL     string            `json:"url"`
				Headers map[string]string `json:"headers"`
			} `json:"upload"`
		} `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if resp.Data.Meeting.Status != "scheduled" {
		t.Errorf("status = %s, want scheduled", resp.Data.Meeting.Status)
	}
	if resp.Data.Upload.URL != "https://signed" {
		t.Errorf("upload.url = %q", resp.Data.Upload.URL)
	}
}

func TestCreate_InvalidBody400(t *testing.T) {
	e := testRouter(t)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/meetings", strings.NewReader(`{"title":""}`))
	req.Header.Set("Content-Type", "application/json")
	e.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", w.Code)
	}
}

func TestCompleteUpload_ReturnsPending(t *testing.T) {
	e := testRouter(t)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/meetings/"+uuid.NewString()+"/complete-upload", nil)
	e.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), `"pending"`) {
		t.Errorf("body = %s, want pending status", w.Body.String())
	}
}

func TestCompleteUpload_BadUUID400(t *testing.T) {
	e := testRouter(t)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/meetings/not-a-uuid/complete-upload", nil)
	e.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", w.Code)
	}
}

func (f *fakeRepo) ListUnfinishedIDs(_ context.Context) ([]uuid.UUID, error) {
	return nil, nil
}
