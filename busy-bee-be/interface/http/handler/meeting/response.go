package meeting

import (
	"time"

	"github.com/google/uuid"

	appmeeting "github.com/as130232/busy-bee/busy-bee-be/application/meeting"
	domainartifact "github.com/as130232/busy-bee/busy-bee-be/domain/artifact"
	domainmeeting "github.com/as130232/busy-bee/busy-bee-be/domain/meeting"
	domainsearch "github.com/as130232/busy-bee/busy-bee-be/domain/search"
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
	// 搜尋命中片段（僅 search 非空且語意命中時出現）
	MatchSnippet string `json:"matchSnippet,omitempty"`
	MatchType    string `json:"matchType,omitempty"`
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

type artifactResponse struct {
	ID        string    `json:"id"`
	Type      string    `json:"type"`
	Content   string    `json:"content"`
	CreatedAt time.Time `json:"createdAt"`
}

func toArtifactResponses(list []domainartifact.Artifact) []artifactResponse {
	out := make([]artifactResponse, len(list))
	for i, a := range list {
		out[i] = artifactResponse{
			ID:        a.ID.String(),
			Type:      string(a.Type),
			Content:   a.Content,
			CreatedAt: a.CreatedAt,
		}
	}
	return out
}

// meetingDetailResponse 詳情含 transcript；列表不含（省流量）。
type meetingDetailResponse struct {
	meetingResponse
	Transcript string `json:"transcript"`
}

func toMeetingDetailResponse(m domainmeeting.Meeting) meetingDetailResponse {
	return meetingDetailResponse{
		meetingResponse: toMeetingResponse(m),
		Transcript:      m.Transcript,
	}
}

func toMeetingListResponses(list []domainmeeting.Meeting) []meetingResponse {
	out := make([]meetingResponse, len(list))
	for i, m := range list {
		out[i] = toMeetingResponse(m)
	}
	return out
}

// toSearchResponses 為搜尋結果填入命中片段（hits 內的會議帶 snippet/type）。
func toSearchResponses(list []domainmeeting.Meeting, hits map[uuid.UUID]domainsearch.SearchResult) []meetingResponse {
	out := make([]meetingResponse, len(list))
	for i, m := range list {
		r := toMeetingResponse(m)
		if h, ok := hits[m.ID]; ok {
			r.MatchSnippet = h.Snippet
			r.MatchType = h.MatchType
		}
		out[i] = r
	}
	return out
}
