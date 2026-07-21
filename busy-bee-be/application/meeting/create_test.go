package meeting

import (
	"context"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/google/uuid"

	domainmeeting "github.com/as130232/busy-bee/busy-bee-be/domain/meeting"
	"github.com/as130232/busy-bee/busy-bee-be/pkg/apperr"
	"github.com/as130232/busy-bee/busy-bee-be/pkg/consts/errcode"
)

type fakeRepo struct {
	created      *domainmeeting.Meeting
	getResult    domainmeeting.Meeting
	getErr       error
	updateResult domainmeeting.Meeting
	updateErr    error
	updatedFrom  domainmeeting.Status
	updatedTo    domainmeeting.Status
}

func (f *fakeRepo) Create(_ context.Context, m domainmeeting.Meeting) (domainmeeting.Meeting, error) {
	f.created = &m
	return m, nil
}

func (f *fakeRepo) GetForUser(_ context.Context, _, _ uuid.UUID) (domainmeeting.Meeting, error) {
	return f.getResult, f.getErr
}

func (f *fakeRepo) UpdateStatus(_ context.Context, _ uuid.UUID, from, to domainmeeting.Status) (domainmeeting.Meeting, error) {
	f.updatedFrom, f.updatedTo = from, to
	return f.updateResult, f.updateErr
}

func (f *fakeRepo) Get(_ context.Context, _ uuid.UUID) (domainmeeting.Meeting, error) {
	return f.getResult, f.getErr
}

func (f *fakeRepo) SaveTranscript(_ context.Context, _ uuid.UUID, _ string, _ []domainmeeting.TranscriptSegment, _ int) (domainmeeting.Meeting, error) {
	return f.getResult, nil
}

func (f *fakeRepo) SetCompleted(_ context.Context, _ uuid.UUID) (domainmeeting.Meeting, error) {
	return f.getResult, nil
}

func (f *fakeRepo) SetFailed(_ context.Context, _ uuid.UUID, _ string) (domainmeeting.Meeting, error) {
	return f.getResult, nil
}

type fakeStorage struct {
	signedPath    string
	signedType    string
	signedMax     int64
	target        domainmeeting.UploadTarget
	existsResult  bool
	existsErr     error
	existsQueried string
}

func (f *fakeStorage) SignedUploadURL(_ context.Context, path, contentType string, maxBytes int64) (domainmeeting.UploadTarget, error) {
	f.signedPath, f.signedType, f.signedMax = path, contentType, maxBytes
	return f.target, nil
}

func (f *fakeStorage) SignedDownloadURL(_ context.Context, _ string) (string, error) {
	return "https://signed-download", nil
}

func (f *fakeStorage) Exists(_ context.Context, path string) (bool, error) {
	f.existsQueried = path
	return f.existsResult, f.existsErr
}

func (f *fakeStorage) Download(_ context.Context, _ string) (io.ReadCloser, int64, error) {
	return io.NopCloser(strings.NewReader("")), 0, nil
}

func TestCreate_ReturnsMeetingAndUploadTarget(t *testing.T) {
	repo := &fakeRepo{}
	st := &fakeStorage{target: domainmeeting.UploadTarget{URL: "https://signed"}}
	uc := NewCreateUC(repo, st)
	userID := uuid.New()

	out, err := uc.Execute(context.Background(), userID, CreateInput{
		Title: "架構討論", ContentType: "audio/webm",
	})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	if out.Meeting.Status != domainmeeting.StatusScheduled {
		t.Errorf("Status = %s, want scheduled (audio not yet uploaded)", out.Meeting.Status)
	}
	if out.Upload.URL != "https://signed" {
		t.Errorf("Upload.URL = %q", out.Upload.URL)
	}
	if repo.created.AudioGCSPath == "" || !strings.HasSuffix(repo.created.AudioGCSPath, ".webm") {
		t.Errorf("AudioGCSPath = %q, want path with .webm ext", repo.created.AudioGCSPath)
	}
	if st.signedPath != repo.created.AudioGCSPath {
		t.Errorf("signed path %q != stored path %q", st.signedPath, repo.created.AudioGCSPath)
	}
	if st.signedMax != 200*1024*1024 {
		t.Errorf("maxBytes = %d, want 200MB", st.signedMax)
	}
}

func TestCreate_EmptyTitleParamError(t *testing.T) {
	uc := NewCreateUC(&fakeRepo{}, &fakeStorage{})

	_, err := uc.Execute(context.Background(), uuid.New(), CreateInput{Title: " ", ContentType: "audio/webm"})

	var ae *apperr.Error
	if !errors.As(err, &ae) || ae.Code != errcode.Param {
		t.Fatalf("err = %v, want Param", err)
	}
}

func TestCreate_UnsupportedContentTypeParamError(t *testing.T) {
	uc := NewCreateUC(&fakeRepo{}, &fakeStorage{})

	_, err := uc.Execute(context.Background(), uuid.New(), CreateInput{Title: "m", ContentType: "video/mp4"})

	var ae *apperr.Error
	if !errors.As(err, &ae) || ae.Code != errcode.Param {
		t.Fatalf("err = %v, want Param for non-audio content type", err)
	}
}

func TestCompleteUpload_TransitionsAndEnqueues(t *testing.T) {
	id, userID := uuid.New(), uuid.New()
	repo := &fakeRepo{
		getResult:    domainmeeting.Meeting{ID: id, UserID: userID, Status: domainmeeting.StatusScheduled, AudioGCSPath: "audio/x.webm"},
		updateResult: domainmeeting.Meeting{ID: id, Status: domainmeeting.StatusPending},
	}
	st := &fakeStorage{existsResult: true}
	q := &fakeQueue{}
	uc := NewCompleteUploadUC(repo, st, q)

	m, err := uc.Execute(context.Background(), userID, id)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if m.Status != domainmeeting.StatusPending {
		t.Errorf("Status = %s, want pending", m.Status)
	}
	if repo.updatedFrom != domainmeeting.StatusScheduled || repo.updatedTo != domainmeeting.StatusPending {
		t.Errorf("transition %s->%s, want scheduled->pending", repo.updatedFrom, repo.updatedTo)
	}
	if q.enqueued != id {
		t.Errorf("enqueued = %v, want %v", q.enqueued, id)
	}
}

func TestCompleteUpload_AudioMissingParamError(t *testing.T) {
	id, userID := uuid.New(), uuid.New()
	repo := &fakeRepo{getResult: domainmeeting.Meeting{ID: id, Status: domainmeeting.StatusScheduled, AudioGCSPath: "audio/x.webm"}}
	uc := NewCompleteUploadUC(repo, &fakeStorage{existsResult: false}, &fakeQueue{})

	_, err := uc.Execute(context.Background(), userID, id)

	var ae *apperr.Error
	if !errors.As(err, &ae) || ae.Code != errcode.Param {
		t.Fatalf("err = %v, want Param (audio not uploaded)", err)
	}
}

func TestCompleteUpload_AlreadyProcessingIsIdempotent(t *testing.T) {
	id, userID := uuid.New(), uuid.New()
	repo := &fakeRepo{getResult: domainmeeting.Meeting{ID: id, Status: domainmeeting.StatusTranscribing}}
	q := &fakeQueue{}
	uc := NewCompleteUploadUC(repo, &fakeStorage{existsResult: true}, q)

	m, err := uc.Execute(context.Background(), userID, id)
	if err != nil {
		t.Fatalf("Execute() error = %v, want idempotent success", err)
	}
	if m.Status != domainmeeting.StatusTranscribing {
		t.Errorf("Status = %s, want unchanged", m.Status)
	}
	if q.enqueued != uuid.Nil {
		t.Error("should not re-enqueue an already-processing meeting")
	}
}

func TestCompleteUpload_NotFoundMapped(t *testing.T) {
	repo := &fakeRepo{getErr: domainmeeting.ErrNotFound}
	uc := NewCompleteUploadUC(repo, &fakeStorage{}, &fakeQueue{})

	_, err := uc.Execute(context.Background(), uuid.New(), uuid.New())

	var ae *apperr.Error
	if !errors.As(err, &ae) || ae.Code != errcode.NotFound {
		t.Fatalf("err = %v, want NotFound", err)
	}
}

type fakeQueue struct {
	enqueued uuid.UUID
}

func (f *fakeQueue) EnqueueProcessMeeting(_ context.Context, id uuid.UUID) error {
	f.enqueued = id
	return nil
}

func (f *fakeRepo) ListUnfinishedIDs(_ context.Context) ([]uuid.UUID, error) {
	return nil, nil
}

func (f *fakeRepo) ListForUser(_ context.Context, _ uuid.UUID, _ string) ([]domainmeeting.Meeting, error) {
	return []domainmeeting.Meeting{f.getResult}, f.getErr
}
