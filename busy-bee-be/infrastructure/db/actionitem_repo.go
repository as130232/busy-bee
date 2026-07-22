package db

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	domainactionitem "github.com/as130232/busy-bee/busy-bee-be/domain/actionitem"
	"github.com/as130232/busy-bee/busy-bee-be/infrastructure/db/sqlcgen"
)

// ActionItemRepo 以 sqlc 實作 domain/actionitem.Repository。
type ActionItemRepo struct {
	q *sqlcgen.Queries
}

var (
	_ domainactionitem.Repository         = (*ActionItemRepo)(nil)
	_ domainactionitem.ReminderRepository = (*ActionItemRepo)(nil)
)

func NewActionItemRepo(pool *pgxpool.Pool) *ActionItemRepo {
	return &ActionItemRepo{q: sqlcgen.New(pool)}
}

func (r *ActionItemRepo) Insert(ctx context.Context, meetingID, userID uuid.UUID, item domainactionitem.Extracted, sortOrder int) (domainactionitem.ActionItem, error) {
	row, err := r.q.InsertActionItem(ctx, sqlcgen.InsertActionItemParams{
		MeetingID:   meetingID,
		UserID:      userID,
		Description: item.Description,
		Assignee:    item.Assignee,
		DueText:     item.DueText,
		DueAt:       item.DueDate(), // 由 dueISO 解析；無則 nil
		Source:      "llm",
		SortOrder:   int32(sortOrder),
	})
	if err != nil {
		return domainactionitem.ActionItem{}, fmt.Errorf("db.InsertActionItem: %w", err)
	}
	return toDomainActionItem(row), nil
}

// InsertManual 使用者手動新增一筆待辦（source='manual'）。
func (r *ActionItemRepo) InsertManual(ctx context.Context, meetingID, userID uuid.UUID, description, assignee string) (domainactionitem.ActionItem, error) {
	row, err := r.q.InsertActionItem(ctx, sqlcgen.InsertActionItemParams{
		MeetingID:   meetingID,
		UserID:      userID,
		Description: description,
		Assignee:    assignee,
		Source:      "manual",
	})
	if err != nil {
		return domainactionitem.ActionItem{}, fmt.Errorf("db.InsertActionItem manual: %w", err)
	}
	return toDomainActionItem(row), nil
}

func (r *ActionItemRepo) DeleteForMeeting(ctx context.Context, meetingID uuid.UUID) error {
	if err := r.q.DeleteActionItemsForMeeting(ctx, meetingID); err != nil {
		return fmt.Errorf("db.DeleteActionItemsForMeeting: %w", err)
	}
	return nil
}

func (r *ActionItemRepo) ListByMeeting(ctx context.Context, meetingID uuid.UUID) ([]domainactionitem.ActionItem, error) {
	rows, err := r.q.ListActionItemsByMeeting(ctx, meetingID)
	if err != nil {
		return nil, fmt.Errorf("db.ListActionItemsByMeeting: %w", err)
	}
	out := make([]domainactionitem.ActionItem, len(rows))
	for i, row := range rows {
		out[i] = toDomainActionItem(row)
	}
	return out, nil
}

func (r *ActionItemRepo) ListPendingForUser(ctx context.Context, userID uuid.UUID) ([]domainactionitem.PendingItem, error) {
	rows, err := r.q.ListPendingActionItemsForUser(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("db.ListPendingActionItemsForUser: %w", err)
	}
	out := make([]domainactionitem.PendingItem, len(rows))
	for i, row := range rows {
		out[i] = domainactionitem.PendingItem{
			ActionItem: domainactionitem.ActionItem{
				ID:          row.ID,
				MeetingID:   row.MeetingID,
				UserID:      row.UserID,
				Description: row.Description,
				Assignee:    row.Assignee,
				DueText:     row.DueText,
				DueAt:       row.DueAt,
				RemindedAt:  row.RemindedAt,
				Done:        row.Done,
				CreatedAt:   row.CreatedAt,
			},
			MeetingTitle: row.MeetingTitle,
		}
	}
	return out, nil
}

// ListDueReminders 到期未提醒的行動項（實作 domainactionitem.ReminderRepository）。
func (r *ActionItemRepo) ListDueReminders(ctx context.Context) ([]domainactionitem.PendingItem, error) {
	rows, err := r.q.ListDueActionItemReminders(ctx)
	if err != nil {
		return nil, fmt.Errorf("db.ListDueActionItemReminders: %w", err)
	}
	out := make([]domainactionitem.PendingItem, len(rows))
	for i, row := range rows {
		out[i] = domainactionitem.PendingItem{
			ActionItem: domainactionitem.ActionItem{
				ID:          row.ID,
				MeetingID:   row.MeetingID,
				UserID:      row.UserID,
				Description: row.Description,
				Assignee:    row.Assignee,
				DueText:     row.DueText,
				DueAt:       row.DueAt,
				RemindedAt:  row.RemindedAt,
				Done:        row.Done,
				CreatedAt:   row.CreatedAt,
			},
			MeetingTitle: row.MeetingTitle,
		}
	}
	return out, nil
}

// MarkReminded 標記行動項已推播到期提醒（防重複）。
func (r *ActionItemRepo) MarkReminded(ctx context.Context, id uuid.UUID) error {
	if err := r.q.MarkActionItemReminded(ctx, id); err != nil {
		return fmt.Errorf("db.MarkActionItemReminded: %w", err)
	}
	return nil
}

func (r *ActionItemRepo) SetDone(ctx context.Context, id, userID uuid.UUID, done bool) (domainactionitem.ActionItem, error) {
	row, err := r.q.SetActionItemDone(ctx, sqlcgen.SetActionItemDoneParams{
		ID:     id,
		UserID: userID,
		Done:   done,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domainactionitem.ActionItem{}, domainactionitem.ErrNotFound
		}
		return domainactionitem.ActionItem{}, fmt.Errorf("db.SetActionItemDone: %w", err)
	}
	return toDomainActionItem(row), nil
}

// UpdateDescription 修改待辦內容（owner-only：以 user_id 過濾）。
func (r *ActionItemRepo) UpdateDescription(ctx context.Context, id, userID uuid.UUID, description string) (domainactionitem.ActionItem, error) {
	row, err := r.q.UpdateActionItemDescription(ctx, sqlcgen.UpdateActionItemDescriptionParams{
		ID:          id,
		UserID:      userID,
		Description: description,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domainactionitem.ActionItem{}, domainactionitem.ErrNotFound
		}
		return domainactionitem.ActionItem{}, fmt.Errorf("db.UpdateActionItemDescription: %w", err)
	}
	return toDomainActionItem(row), nil
}

func toDomainActionItem(row sqlcgen.ActionItem) domainactionitem.ActionItem {
	return domainactionitem.ActionItem{
		ID:          row.ID,
		MeetingID:   row.MeetingID,
		UserID:      row.UserID,
		Description: row.Description,
		Assignee:    row.Assignee,
		DueText:     row.DueText,
		DueAt:       row.DueAt,
		RemindedAt:  row.RemindedAt,
		Done:        row.Done,
		CreatedAt:   row.CreatedAt,
	}
}
