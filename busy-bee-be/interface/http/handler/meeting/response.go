package meeting

import (
	"strings"
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
	Summary         string     `json:"summary,omitempty"`
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
		Summary:         m.Summary,
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

// segmentResponse 分講者逐字稿片段（startMs/endMs 為毫秒）。
type segmentResponse struct {
	Speaker string `json:"speaker"`
	Text    string `json:"text"`
	StartMs int    `json:"startMs"`
	EndMs   int    `json:"endMs"`
}

// meetingDetailResponse 詳情含 transcript 與分講者片段；列表不含（省流量）。
type meetingDetailResponse struct {
	meetingResponse
	Transcript string `json:"transcript"`
	// TranscriptSegments 分講者片段；供應商未分講者時為空陣列。
	TranscriptSegments []segmentResponse `json:"transcriptSegments"`
	// SpeakerNames 講者代號→顯示名（如 {"A":"Ben"}）；無自訂時為空物件。
	SpeakerNames map[string]string `json:"speakerNames"`
}

func toMeetingDetailResponse(m domainmeeting.Meeting) meetingDetailResponse {
	segments := make([]segmentResponse, len(m.TranscriptSegments))
	for i, s := range m.TranscriptSegments {
		segments[i] = segmentResponse{Speaker: s.Speaker, Text: s.Text, StartMs: s.StartMs, EndMs: s.EndMs}
	}
	names := m.SpeakerNames
	if names == nil {
		names = map[string]string{}
	}
	return meetingDetailResponse{
		meetingResponse:    toMeetingResponse(m),
		Transcript:         m.Transcript,
		TranscriptSegments: segments,
		SpeakerNames:       names,
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
			r.MatchSnippet = applySpeakerNames(h.Snippet, m.SpeakerNames)
			r.MatchType = h.MatchType
		}
		out[i] = r
	}
	return out
}

// applySpeakerNames 把片段中各行開頭的講者代號（"A: "）換成使用者自訂顯示名（"Ben: "）。
// 索引用穩定代號、顯示才套映射，故改講者名不需重建索引；此處只在回傳片段時替換。
func applySpeakerNames(snippet string, names map[string]string) string {
	if len(names) == 0 || snippet == "" {
		return snippet
	}
	lines := strings.Split(snippet, "\n")
	for i, line := range lines {
		idx := strings.Index(line, ": ")
		if idx <= 0 {
			continue
		}
		if name, ok := names[line[:idx]]; ok {
			lines[i] = name + line[idx:]
		}
	}
	return strings.Join(lines, "\n")
}
