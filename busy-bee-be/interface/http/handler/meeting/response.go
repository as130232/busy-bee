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
	Scenario        string     `json:"scenario"`
	Language        string     `json:"language"`
	Tags            []string   `json:"tags"`
	Imported        bool       `json:"imported"`
	Summary         string     `json:"summary,omitempty"`
	DurationSeconds int        `json:"durationSeconds"`
	ErrorMessage    string     `json:"errorMessage,omitempty"`
	ScheduledAt     *time.Time `json:"scheduledAt,omitempty"`
	RemindBeforeMin int        `json:"remindBeforeMin"`
	// LiveStartedAt 即時會議開始時間；有值且 status=scheduled 代表即時會議已結束、待上傳錄音。
	LiveStartedAt *time.Time `json:"liveStartedAt,omitempty"`
	CreatedAt     time.Time  `json:"createdAt"`
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
		Scenario:        string(m.Scenario),
		Language:        string(m.Language),
		Tags:            tagsOrEmpty(m.Tags),
		Imported:        m.SourceURL != "",
		Summary:         m.Summary,
		DurationSeconds: m.DurationSeconds,
		ErrorMessage:    m.ErrorMessage,
		ScheduledAt:     m.ScheduledAt,
		RemindBeforeMin: m.RemindBeforeMin,
		LiveStartedAt:   m.LiveStartedAt,
		CreatedAt:       m.CreatedAt,
	}
}

// tagsOrEmpty 確保 JSON 輸出為 [] 而非 null（前端一律當陣列處理）。
func tagsOrEmpty(tags []string) []string {
	if tags == nil {
		return []string{}
	}
	return tags
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
	// ManuallyEdited 使用者手動修正過（AI 校正不會覆蓋）。
	ManuallyEdited bool `json:"manuallyEdited"`
	// OriginalText AI 校正前的文字；有值即代表此段被 AI 校正修改過。
	OriginalText string `json:"originalText,omitempty"`
}

// summaryPointResponse 區塊內一個重點：text 必填，heading/speaker 有值才附。
type summaryPointResponse struct {
	Text    string `json:"text"`
	Heading string `json:"heading,omitempty"`
	Speaker string `json:"speaker,omitempty"`
}

// summarySectionResponse 依情境產生的結構化摘要區塊。
type summarySectionResponse struct {
	Type  string                 `json:"type"`
	Title string                 `json:"title"`
	Items []summaryPointResponse `json:"items"`
}

// meetingDetailResponse 詳情含 transcript 與分講者片段；列表不含（省流量）。
type meetingDetailResponse struct {
	meetingResponse
	Transcript string `json:"transcript"`
	// TranscriptSegments 分講者片段；供應商未分講者時為空陣列。
	TranscriptSegments []segmentResponse `json:"transcriptSegments"`
	// SpeakerNames 講者代號→顯示名（如 {"A":"Ben"}）；無自訂時為空物件。
	SpeakerNames map[string]string `json:"speakerNames"`
	// SummarySections 依情境產生的結構化摘要區塊；未處理時為空陣列。
	SummarySections []summarySectionResponse `json:"summarySections"`
	// UncertainRatio 逐字稿低信心字元比例（0–1），前端據以提示是否建議 AI 校正。
	UncertainRatio float64 `json:"uncertainRatio"`
	// LiveNotes 即時會議的會議中筆記（最後一版重點＋提問紀錄）；非即時會議省略。
	LiveNotes *liveNotesResponse `json:"liveNotes,omitempty"`
}

type liveNotesResponse struct {
	Notes liveSummaryResponse `json:"notes"`
	QA    []liveQAResponse    `json:"qa"`
}

type liveSummaryResponse struct {
	Current string   `json:"current"`
	Points  []string `json:"points"`
}

type liveQAResponse struct {
	Question string               `json:"question"`
	Answer   string               `json:"answer"`
	Sources  []liveSourceResponse `json:"sources"`
	AskedAt  time.Time            `json:"askedAt"`
}

type liveSourceResponse struct {
	Type  string `json:"type"`
	Label string `json:"label"`
}

func toLiveNotesResponse(n *domainmeeting.LiveNotes) *liveNotesResponse {
	if n == nil {
		return nil
	}
	points := n.Notes.Points
	if points == nil {
		points = []string{}
	}
	qa := make([]liveQAResponse, len(n.QA))
	for i, q := range n.QA {
		src := make([]liveSourceResponse, len(q.Sources))
		for j, s := range q.Sources {
			src[j] = liveSourceResponse{Type: s.Type, Label: s.Label}
		}
		qa[i] = liveQAResponse{Question: q.Question, Answer: q.Answer, Sources: src, AskedAt: q.AskedAt}
	}
	return &liveNotesResponse{Notes: liveSummaryResponse{Current: n.Notes.Current, Points: points}, QA: qa}
}

func toMeetingDetailResponse(m domainmeeting.Meeting) meetingDetailResponse {
	segments := make([]segmentResponse, len(m.TranscriptSegments))
	for i, s := range m.TranscriptSegments {
		segments[i] = segmentResponse{
			Speaker: s.Speaker, Text: s.Text, StartMs: s.StartMs, EndMs: s.EndMs,
			ManuallyEdited: s.ManuallyEdited, OriginalText: s.OriginalText,
		}
	}
	names := m.SpeakerNames
	if names == nil {
		names = map[string]string{}
	}
	sections := make([]summarySectionResponse, len(m.SummarySections))
	for i, s := range m.SummarySections {
		items := make([]summaryPointResponse, len(s.Items))
		for j, p := range s.Items {
			items[j] = summaryPointResponse{Text: p.Text, Heading: p.Heading, Speaker: p.Speaker}
		}
		sections[i] = summarySectionResponse{Type: s.Type, Title: s.Title, Items: items}
	}
	return meetingDetailResponse{
		meetingResponse:    toMeetingResponse(m),
		Transcript:         m.Transcript,
		TranscriptSegments: segments,
		SpeakerNames:       names,
		SummarySections:    sections,
		UncertainRatio:     domainmeeting.UncertainRatio(m.TranscriptSegments),
		LiveNotes:          toLiveNotesResponse(m.LiveNotes),
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
