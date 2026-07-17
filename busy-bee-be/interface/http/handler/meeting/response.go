package meeting

import (
	"time"

	appmeeting "github.com/as130232/busy-bee/busy-bee-be/application/meeting"
	domainmeeting "github.com/as130232/busy-bee/busy-bee-be/domain/meeting"
)

type meetingResponse struct {
	ID              string     `json:"id"`
	Title           string     `json:"title"`
	Status          string     `json:"status"`
	DurationSeconds int        `json:"durationSeconds"`
	ErrorMessage    string     `json:"errorMessage,omitempty"`
	ScheduledAt     *time.Time `json:"scheduledAt,omitempty"`
	RemindBeforeMin int        `json:"remindBeforeMin"`
	CreatedAt       time.Time  `json:"createdAt"`
}

type uploadResponse struct {
	URL     string            `json:"url"`
	Headers map[string]string `json:"headers"`
}

type createResponse struct {
	Meeting meetingResponse `json:"meeting"`
	Upload  uploadResponse  `json:"upload"`
}

func toMeetingResponse(m domainmeeting.Meeting) meetingResponse {
	return meetingResponse{
		ID:              m.ID.String(),
		Title:           m.Title,
		Status:          string(m.Status),
		DurationSeconds: m.DurationSeconds,
		ErrorMessage:    m.ErrorMessage,
		ScheduledAt:     m.ScheduledAt,
		RemindBeforeMin: m.RemindBeforeMin,
		CreatedAt:       m.CreatedAt,
	}
}

func toCreateResponse(out appmeeting.CreateOutput) createResponse {
	return createResponse{
		Meeting: toMeetingResponse(out.Meeting),
		Upload:  uploadResponse{URL: out.Upload.URL, Headers: out.Upload.Headers},
	}
}
