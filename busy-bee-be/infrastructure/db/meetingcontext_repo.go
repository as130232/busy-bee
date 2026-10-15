package db

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	domainmc "github.com/as130232/busy-bee/busy-bee-be/domain/meetingcontext"
	"github.com/as130232/busy-bee/busy-bee-be/infrastructure/db/sqlcgen"
)

// MeetingContextRepo 以 sqlc 實作 domain/meetingcontext.Repository（會前資料，F-LIVE）。
type MeetingContextRepo struct {
	q *sqlcgen.Queries
}

var _ domainmc.Repository = (*MeetingContextRepo)(nil)

func NewMeetingContextRepo(pool *pgxpool.Pool) *MeetingContextRepo {
	return &MeetingContextRepo{q: sqlcgen.New(pool)}
}

func (r *MeetingContextRepo) Create(ctx context.Context, c domainmc.Context) (domainmc.Context, error) {
	keyterms := c.Keyterms
	if keyterms == nil {
		keyterms = []string{}
	}
	row, err := r.q.CreateMeetingContext(ctx, sqlcgen.CreateMeetingContextParams{
		MeetingID: c.MeetingID, UserID: c.UserID, Title: c.Title, Source: string(c.Source),
		GcsPath: c.GCSPath, Content: c.Content, Keyterms: keyterms,
	})
	if err != nil {
		return domainmc.Context{}, fmt.Errorf("db.CreateMeetingContext: %w", err)
	}
	return toDomainContext(row), nil
}

func (r *MeetingContextRepo) List(ctx context.Context, meetingID, userID uuid.UUID) ([]domainmc.Context, error) {
	rows, err := r.q.ListMeetingContexts(ctx, sqlcgen.ListMeetingContextsParams{MeetingID: meetingID, UserID: userID})
	if err != nil {
		return nil, fmt.Errorf("db.ListMeetingContexts: %w", err)
	}
	out := make([]domainmc.Context, len(rows))
	for i, row := range rows {
		out[i] = toDomainContext(row)
	}
	return out, nil
}

func (r *MeetingContextRepo) Delete(ctx context.Context, id, meetingID, userID uuid.UUID) (domainmc.Context, error) {
	row, err := r.q.DeleteMeetingContext(ctx, sqlcgen.DeleteMeetingContextParams{ID: id, MeetingID: meetingID, UserID: userID})
	if err != nil {
		return domainmc.Context{}, mapNoRows(err, domainmc.ErrNotFound, "db.DeleteMeetingContext")
	}
	return toDomainContext(row), nil
}

func (r *MeetingContextRepo) SumChars(ctx context.Context, meetingID, userID uuid.UUID) (int64, error) {
	total, err := r.q.SumMeetingContextChars(ctx, sqlcgen.SumMeetingContextCharsParams{MeetingID: meetingID, UserID: userID})
	if err != nil {
		return 0, fmt.Errorf("db.SumMeetingContextChars: %w", err)
	}
	return total, nil
}

func toDomainContext(row sqlcgen.MeetingContext) domainmc.Context {
	return domainmc.Context{
		ID: row.ID, MeetingID: row.MeetingID, UserID: row.UserID, Title: row.Title,
		Source: domainmc.Source(row.Source), GCSPath: row.GcsPath, Content: row.Content,
		Keyterms: row.Keyterms, CreatedAt: row.CreatedAt,
	}
}
