package db

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"

	domainlive "github.com/as130232/busy-bee/busy-bee-be/domain/live"
	domainmeeting "github.com/as130232/busy-bee/busy-bee-be/domain/meeting"
	"github.com/as130232/busy-bee/busy-bee-be/infrastructure/db/sqlcgen"
)

// MeetingRepo 同時實作即時會議（F-LIVE）的 domain/live.Repository。
var _ domainlive.Repository = (*MeetingRepo)(nil)

func (r *MeetingRepo) CreateLive(ctx context.Context, m domainmeeting.Meeting) (domainmeeting.Meeting, error) {
	row, err := r.q.CreateLiveMeeting(ctx, sqlcgen.CreateLiveMeetingParams{
		UserID:       m.UserID,
		Title:        m.Title,
		AudioGcsPath: m.AudioGCSPath,
		Scenario:     string(domainmeeting.ParseScenario(string(m.Scenario))),
		Language:     string(domainmeeting.ParseLanguage(string(m.Language))),
	})
	if err != nil {
		return domainmeeting.Meeting{}, fmt.Errorf("db.CreateLiveMeeting: %w", err)
	}
	return toDomainMeeting(ctx, row), nil
}

func (r *MeetingRepo) StartLiveFromScheduled(ctx context.Context, id, userID uuid.UUID, audioPath string) (domainmeeting.Meeting, error) {
	row, err := r.q.StartLiveFromScheduled(ctx, sqlcgen.StartLiveFromScheduledParams{ID: id, UserID: userID, AudioGcsPath: audioPath})
	if err != nil {
		return domainmeeting.Meeting{}, mapNoRows(err, domainmeeting.ErrStatusConflict, "db.StartLiveFromScheduled")
	}
	return toDomainMeeting(ctx, row), nil
}

func (r *MeetingRepo) AddLiveSeconds(ctx context.Context, id, userID uuid.UUID, seconds int) (domainmeeting.Meeting, error) {
	row, err := r.q.AddMeetingLiveSeconds(ctx, sqlcgen.AddMeetingLiveSecondsParams{ID: id, UserID: userID, Seconds: int32(seconds)})
	if err != nil {
		return domainmeeting.Meeting{}, mapNoRows(err, domainmeeting.ErrStatusConflict, "db.AddMeetingLiveSeconds")
	}
	return toDomainMeeting(ctx, row), nil
}

func (r *MeetingRepo) SumLiveSecondsSince(ctx context.Context, userID uuid.UUID, since time.Time) (int64, error) {
	total, err := r.q.SumLiveSecondsSince(ctx, sqlcgen.SumLiveSecondsSinceParams{UserID: userID, Since: &since})
	if err != nil {
		return 0, fmt.Errorf("db.SumLiveSecondsSince: %w", err)
	}
	return total, nil
}

func (r *MeetingRepo) FinishLive(ctx context.Context, id, userID uuid.UUID, notes domainmeeting.LiveNotes) (domainmeeting.Meeting, error) {
	if notes.QA == nil {
		notes.QA = []domainmeeting.LiveQA{}
	}
	if notes.Notes.Points == nil {
		notes.Notes.Points = []string{}
	}
	b, err := marshalJSONB(notes, "{}")
	if err != nil {
		return domainmeeting.Meeting{}, fmt.Errorf("db.FinishLiveMeeting marshal: %w", err)
	}
	row, err := r.q.FinishLiveMeeting(ctx, sqlcgen.FinishLiveMeetingParams{ID: id, UserID: userID, LiveNotes: b})
	if err != nil {
		return domainmeeting.Meeting{}, mapNoRows(err, domainmeeting.ErrStatusConflict, "db.FinishLiveMeeting")
	}
	return toDomainMeeting(ctx, row), nil
}
