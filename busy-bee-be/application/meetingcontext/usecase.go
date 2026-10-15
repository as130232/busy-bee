// Package meetingcontext 會前資料（F-LIVE）use cases：新增（貼上文字或上傳 .txt/.md/.pdf 轉純文字）、列出、刪除。
// 原始檔不保存：轉出的純文字即為後續 prompt 使用的內容（省 GCS 與刪除同步成本）。
package meetingcontext

import (
	"context"
	"errors"
	"log/slog"
	"path"
	"strings"
	"unicode/utf8"

	"github.com/google/uuid"

	"github.com/as130232/busy-bee/busy-bee-be/domain/meeting"
	domainmc "github.com/as130232/busy-bee/busy-bee-be/domain/meetingcontext"
	"github.com/as130232/busy-bee/busy-bee-be/pkg/apperr"
	"github.com/as130232/busy-bee/busy-bee-be/pkg/consts/errcode"
)

const defaultTextTitle = "貼上的文字"

// MeetingReader 以 owner 條件讀取紀錄。
type MeetingReader interface {
	GetForUser(ctx context.Context, id, userID uuid.UUID) (meeting.Meeting, error)
}

type Config struct {
	// MaxChars 單場會前資料合計字數上限。
	MaxChars int64
	// MaxFileBytes 單一檔案大小上限。
	MaxFileBytes int64
}

type UseCases struct {
	meetings  MeetingReader
	repo      domainmc.Repository
	extractor domainmc.Extractor
	cfg       Config
}

func New(meetings MeetingReader, repo domainmc.Repository, extractor domainmc.Extractor, cfg Config) *UseCases {
	return &UseCases{meetings: meetings, repo: repo, extractor: extractor, cfg: cfg}
}

// AddInput 貼上文字（Text）或上傳檔案（Filename + Data）擇一。
type AddInput struct {
	Title    string
	Text     string
	Filename string
	Data     []byte
}

func (uc *UseCases) Add(ctx context.Context, userID, meetingID uuid.UUID, in AddInput) (domainmc.Context, error) {
	if err := uc.checkMeeting(ctx, userID, meetingID); err != nil {
		return domainmc.Context{}, err
	}
	c := domainmc.Context{MeetingID: meetingID, UserID: userID, Title: strings.TrimSpace(in.Title)}
	var err error
	if in.Filename == "" {
		c.Source, c.Content = domainmc.SourceText, strings.TrimSpace(in.Text)
		if c.Content == "" {
			return domainmc.Context{}, apperr.New(errcode.Param, "text")
		}
		if c.Title == "" {
			c.Title = defaultTextTitle
		}
	} else {
		c.Source = domainmc.SourceFile
		if c.Title == "" {
			c.Title = in.Filename
		}
		if c.Content, c.Keyterms, err = uc.readFile(ctx, in.Filename, in.Data); err != nil {
			return domainmc.Context{}, err
		}
	}

	used, err := uc.repo.SumChars(ctx, meetingID, userID)
	if err != nil {
		return domainmc.Context{}, apperr.Wrap(err, errcode.Internal)
	}
	if used+int64(utf8.RuneCountInString(c.Content)) > uc.cfg.MaxChars {
		return domainmc.Context{}, apperr.New(errcode.ContextTooLarge)
	}
	if c.Keyterms == nil {
		c.Keyterms = uc.textKeyterms(ctx, c.Content)
	}
	out, err := uc.repo.Create(ctx, c)
	if err != nil {
		return domainmc.Context{}, apperr.Wrap(err, errcode.Internal)
	}
	slog.InfoContext(ctx, "meetingcontext.added", "meeting_id", meetingID, "context_id", out.ID, "chars", utf8.RuneCountInString(out.Content))
	return out, nil
}

// readFile 依副檔名轉純文字：txt/md 直接讀（須為 UTF-8），pdf 交給 Gemini 轉換並同時抽術語。
// 回傳的 keyterms 為 nil 時由呼叫端另以純文字抽術語。
func (uc *UseCases) readFile(ctx context.Context, filename string, data []byte) (string, []string, error) {
	if int64(len(data)) > uc.cfg.MaxFileBytes {
		return "", nil, apperr.New(errcode.ContextTooLarge)
	}
	switch strings.ToLower(path.Ext(filename)) {
	case ".txt", ".md":
		if !utf8.Valid(data) {
			return "", nil, apperr.New(errcode.ContextExtractFailed)
		}
		content := strings.TrimSpace(string(data))
		if content == "" {
			return "", nil, apperr.New(errcode.ContextExtractFailed)
		}
		return content, nil, nil
	case ".pdf":
		ext, err := uc.extractor.ExtractContext(ctx, data, "application/pdf")
		if err != nil {
			return "", nil, apperr.Wrap(err, errcode.ContextExtractFailed)
		}
		content := strings.TrimSpace(ext.Content)
		if content == "" {
			return "", nil, apperr.New(errcode.ContextExtractFailed)
		}
		keyterms := ext.Keyterms
		if keyterms == nil {
			keyterms = []string{}
		}
		return content, keyterms, nil
	default:
		return "", nil, apperr.New(errcode.ContextUnsupportedType)
	}
}

// textKeyterms 從純文字抽專有名詞；失敗不阻擋新增（只是少了術語加權）。
func (uc *UseCases) textKeyterms(ctx context.Context, content string) []string {
	ext, err := uc.extractor.ExtractContext(ctx, []byte(content), "text/plain")
	if err != nil {
		slog.WarnContext(ctx, "meetingcontext.keyterms_failed", "err", err)
		return []string{}
	}
	if ext.Keyterms == nil {
		return []string{}
	}
	return ext.Keyterms
}

func (uc *UseCases) List(ctx context.Context, userID, meetingID uuid.UUID) ([]domainmc.Context, error) {
	if err := uc.checkMeeting(ctx, userID, meetingID); err != nil {
		return nil, err
	}
	cs, err := uc.repo.List(ctx, meetingID, userID)
	if err != nil {
		return nil, apperr.Wrap(err, errcode.Internal)
	}
	return cs, nil
}

func (uc *UseCases) Delete(ctx context.Context, userID, meetingID, contextID uuid.UUID) error {
	if _, err := uc.repo.Delete(ctx, contextID, meetingID, userID); err != nil {
		if errors.Is(err, domainmc.ErrNotFound) {
			return apperr.New(errcode.NotFound)
		}
		return apperr.Wrap(err, errcode.Internal)
	}
	return nil
}

func (uc *UseCases) checkMeeting(ctx context.Context, userID, meetingID uuid.UUID) error {
	if _, err := uc.meetings.GetForUser(ctx, meetingID, userID); err != nil {
		if errors.Is(err, meeting.ErrNotFound) {
			return apperr.New(errcode.NotFound)
		}
		return apperr.Wrap(err, errcode.Internal)
	}
	return nil
}
