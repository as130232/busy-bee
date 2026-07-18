package db

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	domainartifact "github.com/as130232/busy-bee/busy-bee-be/domain/artifact"
	"github.com/as130232/busy-bee/busy-bee-be/infrastructure/db/sqlcgen"
)

// ArtifactRepo 以 sqlc 實作 domain/artifact.Repository。
type ArtifactRepo struct {
	q *sqlcgen.Queries
}

var _ domainartifact.Repository = (*ArtifactRepo)(nil)

func NewArtifactRepo(pool *pgxpool.Pool) *ArtifactRepo {
	return &ArtifactRepo{q: sqlcgen.New(pool)}
}

func (r *ArtifactRepo) Upsert(ctx context.Context, meetingID uuid.UUID, t domainartifact.Type, content string) (domainartifact.Artifact, error) {
	row, err := r.q.UpsertArtifact(ctx, sqlcgen.UpsertArtifactParams{
		MeetingID:    meetingID,
		ArtifactType: string(t),
		Content:      content,
	})
	if err != nil {
		return domainartifact.Artifact{}, fmt.Errorf("db.UpsertArtifact: %w", err)
	}
	return toDomainArtifact(row), nil
}

func (r *ArtifactRepo) ListByMeeting(ctx context.Context, meetingID uuid.UUID) ([]domainartifact.Artifact, error) {
	rows, err := r.q.ListArtifactsByMeeting(ctx, meetingID)
	if err != nil {
		return nil, fmt.Errorf("db.ListArtifactsByMeeting: %w", err)
	}
	out := make([]domainartifact.Artifact, len(rows))
	for i, row := range rows {
		out[i] = toDomainArtifact(row)
	}
	return out, nil
}

func toDomainArtifact(row sqlcgen.Artifact) domainartifact.Artifact {
	return domainartifact.Artifact{
		ID:        row.ID,
		MeetingID: row.MeetingID,
		Type:      domainartifact.Type(row.ArtifactType),
		Content:   row.Content,
		CreatedAt: row.CreatedAt,
	}
}
