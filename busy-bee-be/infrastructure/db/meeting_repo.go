package db

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	domainmeeting "github.com/as130232/busy-bee/busy-bee-be/domain/meeting"
	"github.com/as130232/busy-bee/busy-bee-be/infrastructure/db/sqlcgen"
)

// MeetingRepo 以 sqlc 實作 domain/meeting.Repository。
type MeetingRepo struct {
	q *sqlcgen.Queries
}

var _ domainmeeting.Repository = (*MeetingRepo)(nil)

func NewMeetingRepo(pool *pgxpool.Pool) *MeetingRepo {
	return &MeetingRepo{q: sqlcgen.New(pool)}
}

func (r *MeetingRepo) Create(ctx context.Context, m domainmeeting.Meeting) (domainmeeting.Meeting, error) {
	remind := m.RemindBeforeMin
	if remind <= 0 {
		remind = 15
	}
	row, err := r.q.CreateMeeting(ctx, sqlcgen.CreateMeetingParams{
		UserID:          m.UserID,
		Title:           m.Title,
		AudioGcsPath:    m.AudioGCSPath,
		Status:          string(m.Status),
		ScheduledAt:     m.ScheduledAt,
		RemindBeforeMin: int32(remind),
	})
	if err != nil {
		return domainmeeting.Meeting{}, fmt.Errorf("db.CreateMeeting: %w", err)
	}
	return toDomainMeeting(row), nil
}

func (r *MeetingRepo) GetForUser(ctx context.Context, id, userID uuid.UUID) (domainmeeting.Meeting, error) {
	row, err := r.q.GetMeetingForUser(ctx, sqlcgen.GetMeetingForUserParams{ID: id, UserID: userID})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domainmeeting.Meeting{}, domainmeeting.ErrNotFound
		}
		return domainmeeting.Meeting{}, fmt.Errorf("db.GetMeetingForUser: %w", err)
	}
	return toDomainMeeting(row), nil
}

func (r *MeetingRepo) UpdateStatus(ctx context.Context, id uuid.UUID, from, to domainmeeting.Status) (domainmeeting.Meeting, error) {
	row, err := r.q.UpdateMeetingStatus(ctx, sqlcgen.UpdateMeetingStatusParams{
		ID:         id,
		ToStatus:   string(to),
		FromStatus: string(from),
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domainmeeting.Meeting{}, domainmeeting.ErrStatusConflict
		}
		return domainmeeting.Meeting{}, fmt.Errorf("db.UpdateMeetingStatus: %w", err)
	}
	return toDomainMeeting(row), nil
}

func (r *MeetingRepo) Get(ctx context.Context, id uuid.UUID) (domainmeeting.Meeting, error) {
	row, err := r.q.GetMeeting(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domainmeeting.Meeting{}, domainmeeting.ErrNotFound
		}
		return domainmeeting.Meeting{}, fmt.Errorf("db.GetMeeting: %w", err)
	}
	return toDomainMeeting(row), nil
}

func (r *MeetingRepo) SaveTranscript(ctx context.Context, id uuid.UUID, transcript string, durationSeconds int) (domainmeeting.Meeting, error) {
	row, err := r.q.SaveMeetingTranscript(ctx, sqlcgen.SaveMeetingTranscriptParams{
		ID:              id,
		Transcript:      transcript,
		DurationSeconds: int32(durationSeconds),
	})
	if err != nil {
		return domainmeeting.Meeting{}, fmt.Errorf("db.SaveMeetingTranscript: %w", err)
	}
	return toDomainMeeting(row), nil
}

func (r *MeetingRepo) SetCompleted(ctx context.Context, id uuid.UUID) (domainmeeting.Meeting, error) {
	row, err := r.q.SetMeetingCompleted(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domainmeeting.Meeting{}, domainmeeting.ErrStatusConflict
		}
		return domainmeeting.Meeting{}, fmt.Errorf("db.SetMeetingCompleted: %w", err)
	}
	return toDomainMeeting(row), nil
}

func (r *MeetingRepo) SetFailed(ctx context.Context, id uuid.UUID, errorMessage string) (domainmeeting.Meeting, error) {
	row, err := r.q.SetMeetingFailed(ctx, sqlcgen.SetMeetingFailedParams{ID: id, ErrorMessage: errorMessage})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domainmeeting.Meeting{}, domainmeeting.ErrStatusConflict
		}
		return domainmeeting.Meeting{}, fmt.Errorf("db.SetMeetingFailed: %w", err)
	}
	return toDomainMeeting(row), nil
}

func toDomainMeeting(row sqlcgen.Meeting) domainmeeting.Meeting {
	return domainmeeting.Meeting{
		ID:              row.ID,
		UserID:          row.UserID,
		Title:           row.Title,
		AudioGCSPath:    row.AudioGcsPath,
		Status:          domainmeeting.Status(row.Status),
		Transcript:      row.Transcript,
		DurationSeconds: int(row.DurationSeconds),
		ErrorMessage:    row.ErrorMessage,
		ScheduledAt:     row.ScheduledAt,
		RemindBeforeMin: int(row.RemindBeforeMin),
		ProcessedAt:     row.ProcessedAt,
		CreatedAt:       row.CreatedAt,
		UpdatedAt:       row.UpdatedAt,
	}
}
