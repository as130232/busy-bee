// Package meetingcontext 定義會前資料（F-LIVE）entity 與 port：使用者會前貼上或上傳的背景資料，
// 轉成純文字後供即時重點、會議中提問、正式摘要與 STT 術語加權使用（零外部依賴）。
package meetingcontext

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

type Source string

const (
	SourceText Source = "text"
	SourceFile Source = "file"
)

var ErrNotFound = errors.New("meeting context not found")

type Context struct {
	ID        uuid.UUID
	MeetingID uuid.UUID
	UserID    uuid.UUID
	Title     string
	Source    Source
	GCSPath   string
	Content   string
	Keyterms  []string
	CreatedAt time.Time
}

// Repository 會前資料存取；所有查詢以 (meetingID, userID) 過濾。
type Repository interface {
	Create(ctx context.Context, c Context) (Context, error)
	List(ctx context.Context, meetingID, userID uuid.UUID) ([]Context, error)
	// Delete 刪除並回傳被刪的資料（供清 GCS 檔）；不存在回 ErrNotFound。
	Delete(ctx context.Context, id, meetingID, userID uuid.UUID) (Context, error)
	SumChars(ctx context.Context, meetingID, userID uuid.UUID) (int64, error)
}

// Extraction 檔案轉文字結果：Content 為 markdown 純文字（純文字來源時為空，沿用原文）；Keyterms 為專有名詞。
type Extraction struct {
	Content  string
	Keyterms []string
}

// Extractor 將檔案轉為純文字並抽專有名詞（Gemini 實作）。MIMEType 為 text/plain 時只抽術語。
type Extractor interface {
	Extract(ctx context.Context, data []byte, mimeType string) (Extraction, error)
}

// PromptBlock 組 prompt 用的背景資料段落；無資料回空字串（prompt 與無會前資料時相同）。
func PromptBlock(cs []Context) string {
	if len(cs) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString("## 背景資料（會前提供，非會議內容）\n")
	for _, c := range cs {
		fmt.Fprintf(&b, "\n### %s\n%s\n", c.Title, strings.TrimSpace(c.Content))
	}
	return b.String()
}

// MergeKeyterms 合併全域術語與會前資料術語：去空白、去重（不分大小寫），全域優先，總數上限 limit。
func MergeKeyterms(global []string, cs []Context, limit int) []string {
	seen := map[string]bool{}
	out := []string{}
	add := func(t string) {
		t = strings.TrimSpace(t)
		k := strings.ToLower(t)
		if t == "" || seen[k] || len(out) >= limit {
			return
		}
		seen[k] = true
		out = append(out, t)
	}
	for _, t := range global {
		add(t)
	}
	for _, c := range cs {
		for _, t := range c.Keyterms {
			add(t)
		}
	}
	return out
}
