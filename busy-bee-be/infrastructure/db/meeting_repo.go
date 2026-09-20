package db

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	domainmeeting "github.com/as130232/busy-bee/busy-bee-be/domain/meeting"
	"github.com/as130232/busy-bee/busy-bee-be/infrastructure/db/sqlcgen"
)

// unmarshalJSONB 解析 meetings 的 JSONB 欄位。空值跳過；解析失敗記錯而非靜默回空——
// 這些欄位理應為本服務自產的合法 JSON，失敗代表 DB 資料異常，需可觀測以便排查。
func unmarshalJSONB(ctx context.Context, meetingID uuid.UUID, field string, data []byte, dst any) {
	if len(data) == 0 {
		return
	}
	if err := json.Unmarshal(data, dst); err != nil {
		slog.ErrorContext(ctx, "db.meeting.jsonb_unmarshal", "meeting_id", meetingID, "field", field, "err", err)
	}
}

// mapNoRows 將 pgx.ErrNoRows 轉成指定的 domain sentinel（如 ErrNotFound / ErrStatusConflict），
// 其餘錯誤以 op 名稱包裝。收斂 repository 內重複的 ErrNoRows 判斷樣板。
func mapNoRows(err error, sentinel error, op string) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return sentinel
	}
	return fmt.Errorf("%s: %w", op, err)
}

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
		Scenario:        string(domainmeeting.ParseScenario(string(m.Scenario))),
		Language:        string(domainmeeting.ParseLanguage(string(m.Language))),
		ScheduledAt:     m.ScheduledAt,
		RemindBeforeMin: int32(remind),
		SourceUrl:       m.SourceURL,
	})
	if err != nil {
		return domainmeeting.Meeting{}, fmt.Errorf("db.CreateMeeting: %w", err)
	}
	return toDomainMeeting(ctx, row), nil
}

func (r *MeetingRepo) GetForUser(ctx context.Context, id, userID uuid.UUID) (domainmeeting.Meeting, error) {
	row, err := r.q.GetMeetingForUser(ctx, sqlcgen.GetMeetingForUserParams{ID: id, UserID: userID})
	if err != nil {
		return domainmeeting.Meeting{}, mapNoRows(err, domainmeeting.ErrNotFound, "db.GetMeetingForUser")
	}
	return toDomainMeeting(ctx, row), nil
}

func (r *MeetingRepo) UpdateStatus(ctx context.Context, id uuid.UUID, from, to domainmeeting.Status) (domainmeeting.Meeting, error) {
	row, err := r.q.UpdateMeetingStatus(ctx, sqlcgen.UpdateMeetingStatusParams{
		ID:         id,
		ToStatus:   string(to),
		FromStatus: string(from),
	})
	if err != nil {
		return domainmeeting.Meeting{}, mapNoRows(err, domainmeeting.ErrStatusConflict, "db.UpdateMeetingStatus")
	}
	return toDomainMeeting(ctx, row), nil
}

func (r *MeetingRepo) Get(ctx context.Context, id uuid.UUID) (domainmeeting.Meeting, error) {
	row, err := r.q.GetMeeting(ctx, id)
	if err != nil {
		return domainmeeting.Meeting{}, mapNoRows(err, domainmeeting.ErrNotFound, "db.GetMeeting")
	}
	return toDomainMeeting(ctx, row), nil
}

func (r *MeetingRepo) SaveTranscript(ctx context.Context, id, userID uuid.UUID, transcript string, segments []domainmeeting.TranscriptSegment, durationSeconds int) (domainmeeting.Meeting, error) {
	segJSON, err := marshalJSONB(segments, "[]")
	if err != nil {
		return domainmeeting.Meeting{}, fmt.Errorf("db.SaveMeetingTranscript marshal segments: %w", err)
	}
	row, err := r.q.SaveMeetingTranscript(ctx, sqlcgen.SaveMeetingTranscriptParams{
		ID:                 id,
		UserID:             userID,
		Transcript:         transcript,
		TranscriptSegments: segJSON,
		DurationSeconds:    int32(durationSeconds),
	})
	if err != nil {
		return domainmeeting.Meeting{}, mapNoRows(err, domainmeeting.ErrNotFound, "db.SaveMeetingTranscript")
	}
	return toDomainMeeting(ctx, row), nil
}

func (r *MeetingRepo) SaveSummary(ctx context.Context, id, userID uuid.UUID, summary string) (domainmeeting.Meeting, error) {
	row, err := r.q.UpdateMeetingSummary(ctx, sqlcgen.UpdateMeetingSummaryParams{ID: id, UserID: userID, Summary: summary})
	if err != nil {
		return domainmeeting.Meeting{}, mapNoRows(err, domainmeeting.ErrNotFound, "db.UpdateMeetingSummary")
	}
	return toDomainMeeting(ctx, row), nil
}

func (r *MeetingRepo) SaveSummarySections(ctx context.Context, id, userID uuid.UUID, sections []domainmeeting.SummarySection) (domainmeeting.Meeting, error) {
	secJSON, err := marshalJSONB(sections, "[]")
	if err != nil {
		return domainmeeting.Meeting{}, fmt.Errorf("db.UpdateMeetingSummarySections marshal: %w", err)
	}
	row, err := r.q.UpdateMeetingSummarySections(ctx, sqlcgen.UpdateMeetingSummarySectionsParams{ID: id, UserID: userID, SummarySections: secJSON})
	if err != nil {
		return domainmeeting.Meeting{}, mapNoRows(err, domainmeeting.ErrNotFound, "db.UpdateMeetingSummarySections")
	}
	return toDomainMeeting(ctx, row), nil
}

func (r *MeetingRepo) UpdateTranscriptSegments(ctx context.Context, id, userID uuid.UUID, segments []domainmeeting.TranscriptSegment, transcript string) (domainmeeting.Meeting, error) {
	segJSON, err := marshalJSONB(segments, "[]")
	if err != nil {
		return domainmeeting.Meeting{}, fmt.Errorf("db.UpdateMeetingTranscriptSegments marshal: %w", err)
	}
	row, err := r.q.UpdateMeetingTranscriptSegments(ctx, sqlcgen.UpdateMeetingTranscriptSegmentsParams{
		ID:                 id,
		UserID:             userID,
		Transcript:         transcript,
		TranscriptSegments: segJSON,
	})
	if err != nil {
		return domainmeeting.Meeting{}, mapNoRows(err, domainmeeting.ErrNotFound, "db.UpdateMeetingTranscriptSegments")
	}
	return toDomainMeeting(ctx, row), nil
}

func (r *MeetingRepo) UpdateSpeakerNames(ctx context.Context, id, userID uuid.UUID, names map[string]string) (domainmeeting.Meeting, error) {
	namesJSON, err := marshalJSONB(names, "{}")
	if err != nil {
		return domainmeeting.Meeting{}, fmt.Errorf("db.UpdateMeetingSpeakerNames marshal names: %w", err)
	}
	row, err := r.q.UpdateMeetingSpeakerNames(ctx, sqlcgen.UpdateMeetingSpeakerNamesParams{
		ID:           id,
		UserID:       userID,
		SpeakerNames: namesJSON,
	})
	if err != nil {
		return domainmeeting.Meeting{}, mapNoRows(err, domainmeeting.ErrNotFound, "db.UpdateMeetingSpeakerNames")
	}
	return toDomainMeeting(ctx, row), nil
}

func (r *MeetingRepo) SetCompleted(ctx context.Context, id uuid.UUID) (domainmeeting.Meeting, error) {
	row, err := r.q.SetMeetingCompleted(ctx, id)
	if err != nil {
		return domainmeeting.Meeting{}, mapNoRows(err, domainmeeting.ErrStatusConflict, "db.SetMeetingCompleted")
	}
	return toDomainMeeting(ctx, row), nil
}

func (r *MeetingRepo) SetFailed(ctx context.Context, id uuid.UUID, errorMessage string) (domainmeeting.Meeting, error) {
	row, err := r.q.SetMeetingFailed(ctx, sqlcgen.SetMeetingFailedParams{ID: id, ErrorMessage: errorMessage})
	if err != nil {
		return domainmeeting.Meeting{}, mapNoRows(err, domainmeeting.ErrStatusConflict, "db.SetMeetingFailed")
	}
	return toDomainMeeting(ctx, row), nil
}

func (r *MeetingRepo) ListForUser(ctx context.Context, userID uuid.UUID, search string) ([]domainmeeting.Meeting, error) {
	rows, err := r.q.ListMeetingsForUser(ctx, sqlcgen.ListMeetingsForUserParams{
		UserID: userID,
		Search: search,
	})
	if err != nil {
		return nil, fmt.Errorf("db.ListMeetingsForUser: %w", err)
	}
	out := make([]domainmeeting.Meeting, len(rows))
	for i, row := range rows {
		out[i] = toDomainMeeting(ctx, row)
	}
	return out, nil
}

func (r *MeetingRepo) ListUnfinishedIDs(ctx context.Context) ([]uuid.UUID, error) {
	ids, err := r.q.ListUnfinishedMeetingIDs(ctx)
	if err != nil {
		return nil, fmt.Errorf("db.ListUnfinishedMeetingIDs: %w", err)
	}
	return ids, nil
}

func toDomainMeeting(ctx context.Context, row sqlcgen.Meeting) domainmeeting.Meeting {
	var segments []domainmeeting.TranscriptSegment
	unmarshalJSONB(ctx, row.ID, "transcriptSegments", row.TranscriptSegments, &segments)
	var speakerNames map[string]string
	unmarshalJSONB(ctx, row.ID, "speakerNames", row.SpeakerNames, &speakerNames)
	var sections []domainmeeting.SummarySection
	unmarshalJSONB(ctx, row.ID, "summarySections", row.SummarySections, &sections)
	return domainmeeting.Meeting{
		ID:                 row.ID,
		UserID:             row.UserID,
		Title:              row.Title,
		AudioGCSPath:       row.AudioGcsPath,
		SourceURL:          row.SourceUrl,
		Tags:               row.Tags,
		Status:             domainmeeting.Status(row.Status),
		Scenario:           domainmeeting.ParseScenario(row.Scenario),
		Language:           domainmeeting.ParseLanguage(row.Language),
		Transcript:         row.Transcript,
		Summary:            row.Summary,
		SummarySections:    sections,
		TranscriptSegments: segments,
		SpeakerNames:       speakerNames,
		DurationSeconds:    int(row.DurationSeconds),
		ErrorMessage:       row.ErrorMessage,
		ScheduledAt:        row.ScheduledAt,
		RemindBeforeMin:    int(row.RemindBeforeMin),
		ProcessedAt:        row.ProcessedAt,
		CreatedAt:          row.CreatedAt,
		UpdatedAt:          row.UpdatedAt,
	}
}

// marshalJSONB 將值序列化為 jsonb 位元組；空值（nil slice/map）以指定的空 JSON 字面量
// （"[]" 或 "{}"）取代，避免寫入 SQL NULL 或 JSON null。
func marshalJSONB[T any](v T, emptyLiteral string) ([]byte, error) {
	b, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	if string(b) == "null" {
		return []byte(emptyLiteral), nil
	}
	return b, nil
}

func (r *MeetingRepo) CreateScheduled(ctx context.Context, userID uuid.UUID, p domainmeeting.ScheduleParams) (domainmeeting.Meeting, error) {
	at := p.ScheduledAt
	row, err := r.q.CreateScheduledMeeting(ctx, sqlcgen.CreateScheduledMeetingParams{
		UserID:          userID,
		Title:           p.Title,
		Scenario:        string(domainmeeting.ParseScenario(string(p.Scenario))),
		Language:        string(domainmeeting.ParseLanguage(string(p.Language))),
		ScheduledAt:     &at,
		RemindBeforeMin: int32(p.RemindBeforeMin),
	})
	if err != nil {
		return domainmeeting.Meeting{}, fmt.Errorf("db.CreateScheduledMeeting: %w", err)
	}
	return toDomainMeeting(ctx, row), nil
}

func (r *MeetingRepo) UpdateSchedule(ctx context.Context, id, userID uuid.UUID, p domainmeeting.ScheduleParams) (domainmeeting.Meeting, error) {
	at := p.ScheduledAt
	row, err := r.q.UpdateMeetingSchedule(ctx, sqlcgen.UpdateMeetingScheduleParams{
		ID:              id,
		UserID:          userID,
		Title:           p.Title,
		ScheduledAt:     &at,
		RemindBeforeMin: int32(p.RemindBeforeMin),
	})
	if err != nil {
		return domainmeeting.Meeting{}, mapNoRows(err, domainmeeting.ErrNotFound, "db.UpdateMeetingSchedule")
	}
	return toDomainMeeting(ctx, row), nil
}

func (r *MeetingRepo) ListDueReminders(ctx context.Context) ([]domainmeeting.Meeting, error) {
	rows, err := r.q.ListDueReminders(ctx)
	if err != nil {
		return nil, fmt.Errorf("db.ListDueReminders: %w", err)
	}
	out := make([]domainmeeting.Meeting, len(rows))
	for i, row := range rows {
		out[i] = toDomainMeeting(ctx, row)
	}
	return out, nil
}

func (r *MeetingRepo) MarkReminded(ctx context.Context, id uuid.UUID) error {
	if err := r.q.MarkMeetingReminded(ctx, id); err != nil {
		return fmt.Errorf("db.MarkMeetingReminded: %w", err)
	}
	return nil
}

func (r *MeetingRepo) Rename(ctx context.Context, id, userID uuid.UUID, title string) (domainmeeting.Meeting, error) {
	row, err := r.q.RenameMeeting(ctx, sqlcgen.RenameMeetingParams{ID: id, UserID: userID, Title: title})
	if err != nil {
		return domainmeeting.Meeting{}, mapNoRows(err, domainmeeting.ErrNotFound, "db.RenameMeeting")
	}
	return toDomainMeeting(ctx, row), nil
}

func (r *MeetingRepo) UpdateTags(ctx context.Context, id, userID uuid.UUID, tags []string) (domainmeeting.Meeting, error) {
	if tags == nil {
		tags = []string{}
	}
	row, err := r.q.UpdateMeetingTags(ctx, sqlcgen.UpdateMeetingTagsParams{ID: id, UserID: userID, Tags: tags})
	if err != nil {
		return domainmeeting.Meeting{}, mapNoRows(err, domainmeeting.ErrNotFound, "db.UpdateMeetingTags")
	}
	return toDomainMeeting(ctx, row), nil
}

// Delete 刪除會議並回傳其音檔路徑（供上層清理 GCS）；不存在或非本人回 ErrNotFound。
func (r *MeetingRepo) Delete(ctx context.Context, id, userID uuid.UUID) (string, error) {
	audioPath, err := r.q.DeleteMeeting(ctx, sqlcgen.DeleteMeetingParams{ID: id, UserID: userID})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", domainmeeting.ErrNotFound
		}
		return "", fmt.Errorf("db.DeleteMeeting: %w", err)
	}
	return audioPath, nil
}
