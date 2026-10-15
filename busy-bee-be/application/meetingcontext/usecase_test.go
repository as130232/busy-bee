package meetingcontext

import (
	"context"
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/as130232/busy-bee/busy-bee-be/domain/meeting"
	domainmc "github.com/as130232/busy-bee/busy-bee-be/domain/meetingcontext"
	"github.com/as130232/busy-bee/busy-bee-be/pkg/apperr"
	"github.com/as130232/busy-bee/busy-bee-be/pkg/consts/errcode"
)

type fakeMeetings struct{ m meeting.Meeting }

func (f *fakeMeetings) GetForUser(_ context.Context, id, userID uuid.UUID) (meeting.Meeting, error) {
	if id != f.m.ID || userID != f.m.UserID {
		return meeting.Meeting{}, meeting.ErrNotFound
	}
	return f.m, nil
}

type fakeRepo struct {
	items []domainmc.Context
	sum   int64
}

func (r *fakeRepo) Create(_ context.Context, c domainmc.Context) (domainmc.Context, error) {
	c.ID = uuid.New()
	r.items = append(r.items, c)
	return c, nil
}
func (r *fakeRepo) List(context.Context, uuid.UUID, uuid.UUID) ([]domainmc.Context, error) {
	return r.items, nil
}
func (r *fakeRepo) Delete(_ context.Context, id, _, _ uuid.UUID) (domainmc.Context, error) {
	for i, c := range r.items {
		if c.ID == id {
			r.items = slices.Delete(r.items, i, i+1)
			return c, nil
		}
	}
	return domainmc.Context{}, domainmc.ErrNotFound
}
func (r *fakeRepo) SumChars(context.Context, uuid.UUID, uuid.UUID) (int64, error) { return r.sum, nil }

type fakeExtractor struct {
	mime string
	out  domainmc.Extraction
	err  error
}

func (f *fakeExtractor) ExtractContext(_ context.Context, _ []byte, mimeType string) (domainmc.Extraction, error) {
	f.mime = mimeType
	return f.out, f.err
}

func setup() (*UseCases, *fakeRepo, *fakeExtractor, meeting.Meeting) {
	m := meeting.Meeting{ID: uuid.New(), UserID: uuid.New()}
	repo, ex := &fakeRepo{}, &fakeExtractor{out: domainmc.Extraction{Keyterms: []string{"Stage"}}}
	return New(&fakeMeetings{m: m}, repo, ex, Config{MaxChars: 10, MaxFileBytes: 100}), repo, ex, m
}

func wantCode(t *testing.T, err error, code errcode.ErrCode) {
	t.Helper()
	var ae *apperr.Error
	if !errors.As(err, &ae) || ae.Code != code {
		t.Fatalf("err = %v, want code %d", err, code)
	}
}

func TestAdd_Text(t *testing.T) {
	uc, repo, ex, m := setup()
	c, err := uc.Add(context.Background(), m.UserID, m.ID, AddInput{Text: "  上線流程  "})
	if err != nil {
		t.Fatal(err)
	}
	if c.Title != defaultTextTitle || c.Content != "上線流程" || c.Source != domainmc.SourceText || !slices.Equal(c.Keyterms, []string{"Stage"}) {
		t.Errorf("c = %+v", c)
	}
	if ex.mime != "text/plain" {
		t.Errorf("text 只抽術語，mime = %q", ex.mime)
	}
	if len(repo.items) != 1 {
		t.Fatal("not stored")
	}
}

func TestAdd_TextKeytermFailureTolerated(t *testing.T) {
	uc, _, ex, m := setup()
	ex.err = errors.New("gemini down")
	c, err := uc.Add(context.Background(), m.UserID, m.ID, AddInput{Title: "筆記", Text: "內容"})
	if err != nil || c.Title != "筆記" || len(c.Keyterms) != 0 {
		t.Fatalf("c = %+v err %v", c, err)
	}
}

func TestAdd_Files(t *testing.T) {
	uc, _, ex, m := setup()
	c, err := uc.Add(context.Background(), m.UserID, m.ID, AddInput{Filename: "Notes.MD", Data: []byte("# 標題")})
	if err != nil || c.Content != "# 標題" || c.Source != domainmc.SourceFile || c.Title != "Notes.MD" {
		t.Fatalf("md: c = %+v err %v", c, err)
	}

	ex.out = domainmc.Extraction{Content: "PDF 內文", Keyterms: []string{"QA"}}
	c, err = uc.Add(context.Background(), m.UserID, m.ID, AddInput{Filename: "spec.pdf", Data: []byte("%PDF")})
	if err != nil || c.Content != "PDF 內文" || ex.mime != "application/pdf" {
		t.Fatalf("pdf: c = %+v err %v mime %q", c, err, ex.mime)
	}
}

func TestAdd_Rejections(t *testing.T) {
	uc, repo, ex, m := setup()
	ctx := context.Background()

	_, err := uc.Add(ctx, uuid.New(), m.ID, AddInput{Text: "x"})
	wantCode(t, err, errcode.NotFound)
	_, err = uc.Add(ctx, m.UserID, m.ID, AddInput{Text: "  "})
	wantCode(t, err, errcode.Param)
	_, err = uc.Add(ctx, m.UserID, m.ID, AddInput{Filename: "a.docx", Data: []byte("x")})
	wantCode(t, err, errcode.ContextUnsupportedType)
	_, err = uc.Add(ctx, m.UserID, m.ID, AddInput{Filename: "a.txt", Data: []byte(strings.Repeat("x", 101))})
	wantCode(t, err, errcode.ContextTooLarge)
	_, err = uc.Add(ctx, m.UserID, m.ID, AddInput{Filename: "a.txt", Data: []byte{0xff, 0xfe}})
	wantCode(t, err, errcode.ContextExtractFailed)

	repo.sum = 8
	_, err = uc.Add(ctx, m.UserID, m.ID, AddInput{Text: "三個字"})
	wantCode(t, err, errcode.ContextTooLarge) // 8 + 3 > 10（以字數計）

	repo.sum = 0
	ex.err = errors.New("bad pdf")
	_, err = uc.Add(ctx, m.UserID, m.ID, AddInput{Filename: "a.pdf", Data: []byte("x")})
	wantCode(t, err, errcode.ContextExtractFailed)
	ex.err, ex.out = nil, domainmc.Extraction{Content: "   "}
	_, err = uc.Add(ctx, m.UserID, m.ID, AddInput{Filename: "a.pdf", Data: []byte("x")})
	wantCode(t, err, errcode.ContextExtractFailed) // 轉出空白視為失敗
	if len(repo.items) != 0 {
		t.Errorf("rejected inputs must not be stored, got %d", len(repo.items))
	}
}

func TestListAndDelete(t *testing.T) {
	uc, _, _, m := setup()
	ctx := context.Background()
	c, _ := uc.Add(ctx, m.UserID, m.ID, AddInput{Text: "x"})
	list, err := uc.List(ctx, m.UserID, m.ID)
	if err != nil || len(list) != 1 {
		t.Fatalf("list = %v err %v", list, err)
	}
	_, err = uc.List(ctx, uuid.New(), m.ID)
	wantCode(t, err, errcode.NotFound)
	if err := uc.Delete(ctx, m.UserID, m.ID, c.ID); err != nil {
		t.Fatal(err)
	}
	wantCode(t, uc.Delete(ctx, m.UserID, m.ID, c.ID), errcode.NotFound)
}
