package meeting

import (
	"context"
	"fmt"
	"net/url"
	"strings"

	"github.com/google/uuid"

	domainmeeting "github.com/as130232/busy-bee/busy-bee-be/domain/meeting"
	"github.com/as130232/busy-bee/busy-bee-be/pkg/apperr"
	"github.com/as130232/busy-bee/busy-bee-be/pkg/consts/errcode"
)

// importPlaceholderTitle 匯入會議在抓到來源標題前的暫定名；抓取階段會以來源標題覆蓋（見 process.go）。
const importPlaceholderTitle = "匯入中…"

// ImportInput 貼連結匯入的輸入。
type ImportInput struct {
	URL      string
	Title    string // 選填；空則用來源標題（YouTube/Podcast）
	Scenario string
	Language string
}

// ImportUC 由外部連結建立會議：狀態直接 pending、帶 SourceURL，音訊由 worker 抓取（fetchStage）後接原管線。
type ImportUC struct {
	repo  domainmeeting.Repository
	queue domainmeeting.TaskQueue
}

func NewImportUC(repo domainmeeting.Repository, queue domainmeeting.TaskQueue) *ImportUC {
	return &ImportUC{repo: repo, queue: queue}
}

func (uc *ImportUC) Execute(ctx context.Context, userID uuid.UUID, in ImportInput) (domainmeeting.Meeting, error) {
	src := strings.TrimSpace(in.URL)
	if u, err := url.Parse(src); err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return domainmeeting.Meeting{}, apperr.New(errcode.Param, "url")
	}

	title := strings.TrimSpace(in.Title)
	if title == "" {
		title = importPlaceholderTitle
	}

	id := uuid.New()
	m, err := uc.repo.Create(ctx, domainmeeting.Meeting{
		ID:           id,
		UserID:       userID,
		Title:        title,
		Status:       domainmeeting.StatusPending,
		Scenario:     domainmeeting.ParseScenario(in.Scenario),
		Language:     domainmeeting.ParseLanguage(in.Language),
		AudioGCSPath: fmt.Sprintf("audio/%s/%s.m4a", userID, id),
		SourceURL:    src,
	})
	if err != nil {
		return domainmeeting.Meeting{}, apperr.Wrap(err, errcode.Internal)
	}

	if err := uc.queue.EnqueueProcessMeeting(ctx, m.ID); err != nil {
		return domainmeeting.Meeting{}, apperr.Wrap(err, errcode.Internal)
	}
	return m, nil
}
