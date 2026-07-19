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

var _ domainactionitem.Repository = (*ActionItemRepo)(nil)

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
		SortOrder:   int32(sortOrder),
	})
	if err != nil {
		return domainactionitem.ActionItem{}, fmt.Errorf("db.InsertActionItem: %w", err)
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
				Done:        row.Done,
				CreatedAt:   row.CreatedAt,
			},
			MeetingTitle: row.MeetingTitle,
		}
	}
	return out, nil
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

func toDomainActionItem(row sqlcgen.ActionItem) domainactionitem.ActionItem {
	return domainactionitem.ActionItem{
		ID:          row.ID,
		MeetingID:   row.MeetingID,
		UserID:      row.UserID,
		Description: row.Description,
		Assignee:    row.Assignee,
		DueText:     row.DueText,
		Done:        row.Done,
		CreatedAt:   row.CreatedAt,
	}
}
