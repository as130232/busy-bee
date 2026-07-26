package db

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	domainactionitem "github.com/as130232/busy-bee/busy-bee-be/domain/actionitem"
	domainartifact "github.com/as130232/busy-bee/busy-bee-be/domain/artifact"
	"github.com/as130232/busy-bee/busy-bee-be/infrastructure/db/sqlcgen"
)

// ProcessRepo 提供處理管線中需要跨表原子性的落庫操作。
// 沿用 ChunkRepo.Upsert 的作法：以單一 transaction 包住 delete→insert，避免中途失敗留下部分/空狀態。
type ProcessRepo struct {
	pool *pgxpool.Pool
}

func NewProcessRepo(pool *pgxpool.Pool) *ProcessRepo {
	return &ProcessRepo{pool: pool}
}

// SaveExtraction 於單一 transaction 內落庫抽取階段產物：
// 摘要（可選）→ 清空舊行動項 → 依序插入新行動項 → 寫入冪等標記。
// 任一步失敗整批 rollback，確保「標記存在即代表行動項完整」的不變式，且不留下瞬間空清單。
func (r *ProcessRepo) SaveExtraction(
	ctx context.Context,
	meetingID, userID uuid.UUID,
	summary string,
	items []domainactionitem.Extracted,
	markerType domainartifact.Type,
	markerJSON string,
) error {
	return pgx.BeginFunc(ctx, r.pool, func(tx pgx.Tx) error {
		q := sqlcgen.New(tx)
		meetings := &MeetingRepo{q: q}
		actionItems := &ActionItemRepo{q: q}
		artifacts := &ArtifactRepo{q: q}

		if summary != "" {
			if _, err := meetings.SaveSummary(ctx, meetingID, summary); err != nil {
				return fmt.Errorf("db.SaveExtraction summary: %w", err)
			}
		}
		if err := actionItems.DeleteForMeeting(ctx, meetingID); err != nil {
			return fmt.Errorf("db.SaveExtraction clear: %w", err)
		}
		for i, it := range items {
			if _, err := actionItems.Insert(ctx, meetingID, userID, it, i); err != nil {
				return fmt.Errorf("db.SaveExtraction insert: %w", err)
			}
		}
		if _, err := artifacts.Upsert(ctx, meetingID, markerType, markerJSON); err != nil {
			return fmt.Errorf("db.SaveExtraction marker: %w", err)
		}
		return nil
	})
}
