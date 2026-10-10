package db

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	domaincorrection "github.com/as130232/busy-bee/busy-bee-be/domain/correction"
	domainmeeting "github.com/as130232/busy-bee/busy-bee-be/domain/meeting"
	"github.com/as130232/busy-bee/busy-bee-be/infrastructure/db/sqlcgen"
)

// CorrectionRepo 實作 domain/correction.Repository。
// 跨表原子操作沿用 ProcessRepo 作法（pgx.BeginFunc），確保中途失敗整批 rollback。
type CorrectionRepo struct {
	pool *pgxpool.Pool
	q    *sqlcgen.Queries
}

var _ domaincorrection.Repository = (*CorrectionRepo)(nil)

func NewCorrectionRepo(pool *pgxpool.Pool) *CorrectionRepo {
	return &CorrectionRepo{pool: pool, q: sqlcgen.New(pool)}
}

// pgUniqueViolation PostgreSQL unique_violation 錯誤碼。
const pgUniqueViolation = "23505"

func (r *CorrectionRepo) CreateWithSnapshot(ctx context.Context, meetingID, userID uuid.UUID, snap domaincorrection.Snapshot) (domaincorrection.Correction, error) {
	segJSON, err := json.Marshal(snap.Segments)
	if err != nil {
		return domaincorrection.Correction{}, fmt.Errorf("db.CreateCorrection marshal segments: %w", err)
	}
	secJSON, err := json.Marshal(snap.SummarySections)
	if err != nil {
		return domaincorrection.Correction{}, fmt.Errorf("db.CreateCorrection marshal sections: %w", err)
	}
	var out domaincorrection.Correction
	err = pgx.BeginFunc(ctx, r.pool, func(tx pgx.Tx) error {
		q := sqlcgen.New(tx)
		n, err := q.CountCorrectionsForMeeting(ctx, sqlcgen.CountCorrectionsForMeetingParams{MeetingID: meetingID, UserID: userID})
		if err != nil {
			return fmt.Errorf("db.CreateCorrection count: %w", err)
		}
		if n >= domaincorrection.MaxAttempts {
			return domaincorrection.ErrStatusConflict
		}
		row, err := q.InsertCorrection(ctx, sqlcgen.InsertCorrectionParams{
			MeetingID: meetingID, UserID: userID, Attempt: int32(n) + 1,
			OriginalSegments: segJSON, OriginalTranscript: snap.Transcript,
			OriginalSummarySections: secJSON, OriginalSummary: snap.Summary,
		})
		if err != nil {
			var pgErr *pgconn.PgError
			if errors.As(err, &pgErr) && pgErr.Code == pgUniqueViolation {
				return domaincorrection.ErrActiveExists // 進行中部分唯一索引，或 attempt 併發衝突
			}
			return fmt.Errorf("db.CreateCorrection insert: %w", err)
		}
		out, err = toDomainCorrection(row)
		return err
	})
	return out, err
}

func (r *CorrectionRepo) Get(ctx context.Context, id uuid.UUID) (domaincorrection.Correction, error) {
	row, err := r.q.GetCorrection(ctx, id)
	if err != nil {
		return domaincorrection.Correction{}, mapNoRows(err, domaincorrection.ErrNotFound, "db.GetCorrection")
	}
	return toDomainCorrection(row)
}

func (r *CorrectionRepo) GetLatestForMeeting(ctx context.Context, meetingID, userID uuid.UUID) (domaincorrection.Correction, error) {
	row, err := r.q.GetLatestCorrectionForMeeting(ctx, sqlcgen.GetLatestCorrectionForMeetingParams{MeetingID: meetingID, UserID: userID})
	if err != nil {
		return domaincorrection.Correction{}, mapNoRows(err, domaincorrection.ErrNotFound, "db.GetLatestCorrection")
	}
	return toDomainCorrection(row)
}

func (r *CorrectionRepo) CountForMeeting(ctx context.Context, meetingID, userID uuid.UUID) (int, error) {
	n, err := r.q.CountCorrectionsForMeeting(ctx, sqlcgen.CountCorrectionsForMeetingParams{MeetingID: meetingID, UserID: userID})
	if err != nil {
		return 0, fmt.Errorf("db.CountCorrections: %w", err)
	}
	return int(n), nil
}

func (r *CorrectionRepo) UpdateStatus(ctx context.Context, id uuid.UUID, from, to domaincorrection.Status, errorMessage string) (domaincorrection.Correction, error) {
	row, err := r.q.UpdateCorrectionStatus(ctx, sqlcgen.UpdateCorrectionStatusParams{
		ID: id, FromStatus: string(from), ToStatus: string(to), ErrorMessage: errorMessage,
	})
	if err != nil {
		return domaincorrection.Correction{}, mapNoRows(err, domaincorrection.ErrStatusConflict, "db.UpdateCorrectionStatus")
	}
	return toDomainCorrection(row)
}

func (r *CorrectionRepo) SaveProgress(ctx context.Context, id uuid.UUID, p domaincorrection.Progress) error {
	raw, err := json.Marshal(p)
	if err != nil {
		return fmt.Errorf("db.SaveCorrectionProgress marshal: %w", err)
	}
	if err := r.q.UpdateCorrectionProgress(ctx, sqlcgen.UpdateCorrectionProgressParams{ID: id, Progress: raw}); err != nil {
		return fmt.Errorf("db.SaveCorrectionProgress: %w", err)
	}
	return nil
}

func (r *CorrectionRepo) ListUnfinishedIDs(ctx context.Context) ([]uuid.UUID, error) {
	ids, err := r.q.ListUnfinishedCorrectionIDs(ctx)
	if err != nil {
		return nil, fmt.Errorf("db.ListUnfinishedCorrectionIDs: %w", err)
	}
	return ids, nil
}

// ApplyResult 單一 transaction：逐字稿 → 摘要區塊 → 一句話摘要 → 行動項替換 → status running→completed。
func (r *CorrectionRepo) ApplyResult(ctx context.Context, c domaincorrection.Correction, res domaincorrection.Result) error {
	return pgx.BeginFunc(ctx, r.pool, func(tx pgx.Tx) error {
		q := sqlcgen.New(tx)
		meetings := &MeetingRepo{q: q}
		items := &ActionItemRepo{q: q}
		current, err := lockSegments(ctx, q, c)
		if err != nil {
			return fmt.Errorf("db.ApplyCorrection lock: %w", err)
		}
		segs := domaincorrection.ApplyPatches(current, res.Patches)
		if _, err := meetings.UpdateTranscriptSegments(ctx, c.MeetingID, c.UserID, segs, domainmeeting.FlattenSegments(segs)); err != nil {
			return fmt.Errorf("db.ApplyCorrection segments: %w", err)
		}
		if len(res.SummarySections) > 0 {
			if _, err := meetings.SaveSummarySections(ctx, c.MeetingID, c.UserID, res.SummarySections); err != nil {
				return fmt.Errorf("db.ApplyCorrection sections: %w", err)
			}
		}
		if res.Summary != "" {
			if _, err := meetings.SaveSummary(ctx, c.MeetingID, c.UserID, res.Summary); err != nil {
				return fmt.Errorf("db.ApplyCorrection summary: %w", err)
			}
		}
		for _, id := range res.DeleteItemIDs {
			// 期間被使用者勾選/改過/刪除的項目不刪（0 列不視為錯誤）
			if _, err := q.DeleteReplaceableActionItem(ctx, sqlcgen.DeleteReplaceableActionItemParams{ID: id, UserID: c.UserID}); err != nil {
				return fmt.Errorf("db.ApplyCorrection delete item: %w", err)
			}
		}
		for i, it := range res.InsertItems {
			if _, err := items.Insert(ctx, c.MeetingID, c.UserID, it, res.InsertSortFrom+i); err != nil {
				return fmt.Errorf("db.ApplyCorrection insert item: %w", err)
			}
		}
		if _, err := q.UpdateCorrectionStatus(ctx, sqlcgen.UpdateCorrectionStatusParams{
			ID: c.ID, FromStatus: string(domaincorrection.StatusRunning), ToStatus: string(domaincorrection.StatusCompleted),
		}); err != nil {
			return mapNoRows(err, domaincorrection.ErrStatusConflict, "db.ApplyCorrection status")
		}
		return nil
	})
}

// Revert 單一 transaction：快照寫回逐字稿/摘要區塊/一句話摘要，status completed→reverted；行動項不動。
func (r *CorrectionRepo) Revert(ctx context.Context, c domaincorrection.Correction) error {
	return pgx.BeginFunc(ctx, r.pool, func(tx pgx.Tx) error {
		q := sqlcgen.New(tx)
		meetings := &MeetingRepo{q: q}
		snap := c.Original
		current, err := lockSegments(ctx, q, c)
		if err != nil {
			return fmt.Errorf("db.RevertCorrection lock: %w", err)
		}
		segs := domaincorrection.RevertSegments(snap.Segments, current)
		if _, err := meetings.UpdateTranscriptSegments(ctx, c.MeetingID, c.UserID, segs, domainmeeting.FlattenSegments(segs)); err != nil {
			return fmt.Errorf("db.RevertCorrection segments: %w", err)
		}
		if _, err := meetings.SaveSummarySections(ctx, c.MeetingID, c.UserID, snap.SummarySections); err != nil {
			return fmt.Errorf("db.RevertCorrection sections: %w", err)
		}
		if _, err := meetings.SaveSummary(ctx, c.MeetingID, c.UserID, snap.Summary); err != nil {
			return fmt.Errorf("db.RevertCorrection summary: %w", err)
		}
		if _, err := q.UpdateCorrectionStatus(ctx, sqlcgen.UpdateCorrectionStatusParams{
			ID: c.ID, FromStatus: string(domaincorrection.StatusCompleted), ToStatus: string(domaincorrection.StatusReverted),
		}); err != nil {
			return mapNoRows(err, domaincorrection.ErrStatusConflict, "db.RevertCorrection status")
		}
		return nil
	})
}

// lockSegments 交易內以 FOR UPDATE 鎖定並讀取紀錄當下的 segments。
func lockSegments(ctx context.Context, q *sqlcgen.Queries, c domaincorrection.Correction) ([]domainmeeting.TranscriptSegment, error) {
	raw, err := q.LockMeetingSegments(ctx, sqlcgen.LockMeetingSegmentsParams{ID: c.MeetingID, UserID: c.UserID})
	if err != nil {
		return nil, mapNoRows(err, domaincorrection.ErrNotFound, "db.LockMeetingSegments")
	}
	var segs []domainmeeting.TranscriptSegment
	if len(raw) > 0 {
		if err := json.Unmarshal(raw, &segs); err != nil {
			return nil, fmt.Errorf("db.LockMeetingSegments unmarshal: %w", err)
		}
	}
	return segs, nil
}

func toDomainCorrection(row sqlcgen.TranscriptCorrection) (domaincorrection.Correction, error) {
	c := domaincorrection.Correction{
		ID: row.ID, MeetingID: row.MeetingID, UserID: row.UserID,
		Attempt: int(row.Attempt), Status: domaincorrection.Status(row.Status),
		ErrorMessage: row.ErrorMessage, CreatedAt: row.CreatedAt,
		CompletedAt: row.CompletedAt, RevertedAt: row.RevertedAt,
		Original: domaincorrection.Snapshot{Transcript: row.OriginalTranscript, Summary: row.OriginalSummary},
	}
	if err := json.Unmarshal(row.Progress, &c.Progress); err != nil {
		return domaincorrection.Correction{}, fmt.Errorf("db.toDomainCorrection progress: %w", err)
	}
	if c.Progress.Chunks == nil {
		c.Progress.Chunks = map[string][]domaincorrection.Patch{}
	}
	if err := json.Unmarshal(row.OriginalSegments, &c.Original.Segments); err != nil {
		return domaincorrection.Correction{}, fmt.Errorf("db.toDomainCorrection segments: %w", err)
	}
	if err := json.Unmarshal(row.OriginalSummarySections, &c.Original.SummarySections); err != nil {
		return domaincorrection.Correction{}, fmt.Errorf("db.toDomainCorrection sections: %w", err)
	}
	return c, nil
}
