// Package live 定義即時會議（F-LIVE）的 port interface（零外部依賴）。
// 會議中的狀態（即時逐字稿、目前重點）放在瀏覽器，後端無狀態，只負責 Gemini 呼叫、用量與狀態轉換。
package live

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/as130232/busy-bee/busy-bee-be/domain/meeting"
)

// Repository 即時會議的資料存取（db.MeetingRepo 實作）。皆以 (id, userID) 落庫。
type Repository interface {
	CreateLive(ctx context.Context, m meeting.Meeting) (meeting.Meeting, error)
	// StartLiveFromScheduled scheduled → live；狀態不符回 meeting.ErrStatusConflict。
	StartLiveFromScheduled(ctx context.Context, id, userID uuid.UUID, audioPath string) (meeting.Meeting, error)
	// AddLiveSeconds 累加即時辨識秒數；非 live 回 meeting.ErrStatusConflict。
	AddLiveSeconds(ctx context.Context, id, userID uuid.UUID, seconds int) (meeting.Meeting, error)
	SumLiveSecondsSince(ctx context.Context, userID uuid.UUID, since time.Time) (int64, error)
	// FinishLive 存會議中筆記並 live → scheduled；非 live 回 meeting.ErrStatusConflict。
	FinishLive(ctx context.Context, id, userID uuid.UUID, notes meeting.LiveNotes) (meeting.Meeting, error)
}

// Token 瀏覽器直連 Deepgram 串流用的短效金鑰。
type Token struct {
	AccessToken string
	ExpiresIn   int
}

// TokenIssuer 簽發短效 STT 金鑰（Deepgram /v1/auth/grant）。
type TokenIssuer interface {
	Issue(ctx context.Context) (Token, error)
}

// NotesInput 更新即時重點的輸入。
type NotesInput struct {
	Background    string
	Current       meeting.LiveSummary
	NewTranscript string
}

// AskInput 會議中提問的輸入。Transcript 為目前完整即時逐字稿（每行含時間與來源）。
type AskInput struct {
	Background string
	Transcript string
	Question   string
	History    []meeting.LiveQA
}

// Answer 提問回答與依據來源。
type Answer struct {
	Text    string
	Sources []meeting.LiveSource
}

// Assistant 會議中 AI（Gemini 實作在 infrastructure/llm）。
type Assistant interface {
	UpdateNotes(ctx context.Context, in NotesInput) (meeting.LiveSummary, error)
	Ask(ctx context.Context, in AskInput) (Answer, error)
}
