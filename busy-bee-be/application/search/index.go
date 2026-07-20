// Package search application：索引（IndexUC）與查詢（SearchUC）use cases。
package search

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	domainmeeting "github.com/as130232/busy-bee/busy-bee-be/domain/meeting"
	domainsearch "github.com/as130232/busy-bee/busy-bee-be/domain/search"
)

const (
	chunkTargetChars = 400
	chunkOverlap     = 1
)

// meetingGetter 窄介面：只取單一會議（IndexUC 所需）。*db.MeetingRepo 滿足。
type meetingGetter interface {
	Get(ctx context.Context, id uuid.UUID) (domainmeeting.Meeting, error)
}

// IndexUC 為單一會議建立語意索引（切塊 → embed → upsert）。
type IndexUC struct {
	meetings meetingGetter
	embedder domainsearch.Embedder
	chunks   domainsearch.ChunkRepository
}

func NewIndexUC(meetings meetingGetter, embedder domainsearch.Embedder, chunks domainsearch.ChunkRepository) *IndexUC {
	return &IndexUC{meetings: meetings, embedder: embedder, chunks: chunks}
}

// Execute 切塊 → 逐塊 embed → upsert（冪等：Upsert 內部先刪後插）。空逐字稿跳過。
func (uc *IndexUC) Execute(ctx context.Context, meetingID uuid.UUID) error {
	m, err := uc.meetings.Get(ctx, meetingID)
	if err != nil {
		return fmt.Errorf("index get meeting: %w", err)
	}
	parts := domainsearch.SplitIntoChunks(m.Transcript, chunkTargetChars, chunkOverlap)
	if len(parts) == 0 {
		return nil
	}
	chunks := make([]domainsearch.Chunk, 0, len(parts))
	for i, p := range parts {
		vec, err := uc.embedder.Embed(ctx, p)
		if err != nil {
			return fmt.Errorf("index embed chunk %d: %w", i, err)
		}
		chunks = append(chunks, domainsearch.Chunk{
			MeetingID: m.ID, UserID: m.UserID, ChunkIndex: i, Content: p, Embedding: vec,
		})
	}
	if err := uc.chunks.Upsert(ctx, chunks); err != nil {
		return fmt.Errorf("index upsert: %w", err)
	}
	return nil
}
