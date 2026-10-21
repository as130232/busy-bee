package meeting

import (
	"context"
	"errors"
	"log/slog"

	"github.com/google/uuid"

	domainactionitem "github.com/as130232/busy-bee/busy-bee-be/domain/actionitem"
	domainmeeting "github.com/as130232/busy-bee/busy-bee-be/domain/meeting"
	"github.com/as130232/busy-bee/busy-bee-be/pkg/apperr"
	"github.com/as130232/busy-bee/busy-bee-be/pkg/consts/errcode"
)

// replaceRepo 全部取代所需的最小 repo 介面（*db.MeetingRepo 滿足）。
type replaceRepo interface {
	segmentEditorRepo
	SaveSummary(ctx context.Context, id, userID uuid.UUID, summary string) (domainmeeting.Meeting, error)
	SaveSummarySections(ctx context.Context, id, userID uuid.UUID, sections []domainmeeting.SummarySection) (domainmeeting.Meeting, error)
}

// replaceActionItems 行動項讀寫（*db.ActionItemRepo 滿足）。
type replaceActionItems interface {
	ListByMeeting(ctx context.Context, meetingID uuid.UUID) ([]domainactionitem.ActionItem, error)
	UpdateDescription(ctx context.Context, id, userID uuid.UUID, description string) (domainactionitem.ActionItem, error)
}

// ReplaceTextInput 全部取代參數。IncludeSummary 時一併取代摘要與行動項；DryRun 只計數不寫入（預覽用）。
type ReplaceTextInput struct {
	Rule           domainmeeting.ReplaceRule
	IncludeSummary bool
	DryRun         bool
}

// ReplaceCounts 各處取代次數。
type ReplaceCounts struct {
	Transcript  int
	Summary     int
	ActionItems int
}

// ReplaceTextResult 取代後的會議與計數。
type ReplaceTextResult struct {
	Meeting domainmeeting.Meeting
	Counts  ReplaceCounts
}

// ReplaceTextUC 逐字稿全部取代（修正 STT 錯字，如 DBA → DVA），可選擇一併取代摘要與行動項。
// 不呼叫 AI；被取代的片段標記人工修正，之後 AI 校正不會覆蓋。
type ReplaceTextUC struct {
	repo    replaceRepo
	items   replaceActionItems
	indexer MeetingIndexer // 選填；nil 時跳過重新索引
}

func NewReplaceTextUC(repo replaceRepo, items replaceActionItems, indexer MeetingIndexer) *ReplaceTextUC {
	return &ReplaceTextUC{repo: repo, items: items, indexer: indexer}
}

// Execute 依序寫入逐字稿 → 摘要 → 行動項（本人限定，僅限已完成的紀錄）。
// 中途失敗時已寫入的部分保留；重新執行只會處理剩下仍符合的地方。
func (uc *ReplaceTextUC) Execute(ctx context.Context, userID, meetingID uuid.UUID, in ReplaceTextInput) (ReplaceTextResult, error) {
	rule := in.Rule.Normalize()
	if !rule.Valid() {
		return ReplaceTextResult{}, apperr.New(errcode.Param, "rule")
	}
	m, err := uc.repo.GetForUser(ctx, meetingID, userID)
	if err != nil {
		return ReplaceTextResult{}, mapMeetingErr(err)
	}
	if m.Status != domainmeeting.StatusCompleted {
		return ReplaceTextResult{}, apperr.New(errcode.Conflict)
	}

	var counts ReplaceCounts
	segs, transcript, n := replaceTranscript(m, rule)
	counts.Transcript = n

	var summary string
	var sections []domainmeeting.SummarySection
	var summaryN, sectionsN int
	type itemChange struct {
		id   uuid.UUID
		desc string
	}
	var itemChanges []itemChange
	if in.IncludeSummary {
		summary, summaryN = domainmeeting.ReplaceText(m.Summary, rule)
		sections, sectionsN = replaceSections(m.SummarySections, rule)
		counts.Summary = summaryN + sectionsN
		items, err := uc.items.ListByMeeting(ctx, m.ID)
		if err != nil {
			return ReplaceTextResult{}, apperr.Wrap(err, errcode.Internal)
		}
		for _, it := range items {
			if d, k := domainmeeting.ReplaceText(it.Description, rule); k > 0 {
				itemChanges = append(itemChanges, itemChange{id: it.ID, desc: d})
				counts.ActionItems += k
			}
		}
	}
	if in.DryRun {
		return ReplaceTextResult{Meeting: m, Counts: counts}, nil
	}

	if counts.Transcript > 0 {
		if m, err = uc.repo.UpdateTranscriptSegments(ctx, m.ID, userID, segs, transcript); err != nil {
			return ReplaceTextResult{}, mapMeetingErr(err)
		}
	}
	if sectionsN > 0 {
		if m, err = uc.repo.SaveSummarySections(ctx, m.ID, userID, sections); err != nil {
			return ReplaceTextResult{}, mapMeetingErr(err)
		}
	}
	if summaryN > 0 {
		if m, err = uc.repo.SaveSummary(ctx, m.ID, userID, summary); err != nil {
			return ReplaceTextResult{}, mapMeetingErr(err)
		}
	}
	for _, c := range itemChanges {
		if _, err := uc.items.UpdateDescription(ctx, c.id, userID, c.desc); err != nil {
			return ReplaceTextResult{}, apperr.Wrap(err, errcode.Internal)
		}
	}

	if counts.Transcript > 0 && uc.indexer != nil {
		if ierr := uc.indexer.Execute(ctx, m.ID); ierr != nil {
			slog.WarnContext(ctx, "meeting.replace_text.reindex_failed", "meeting_id", m.ID, "err", ierr)
		}
	}
	slog.InfoContext(ctx, "meeting.replace_text", "meeting_id", m.ID,
		"transcript", counts.Transcript, "summary", counts.Summary, "action_items", counts.ActionItems)
	return ReplaceTextResult{Meeting: m, Counts: counts}, nil
}

// replaceTranscript 取代逐字稿；有片段時逐段取代（變動者標人工修正）並重新攤平，否則取代攤平文字。
func replaceTranscript(m domainmeeting.Meeting, rule domainmeeting.ReplaceRule) ([]domainmeeting.TranscriptSegment, string, int) {
	if len(m.TranscriptSegments) == 0 {
		t, n := domainmeeting.ReplaceText(m.Transcript, rule)
		return nil, t, n
	}
	segs := make([]domainmeeting.TranscriptSegment, len(m.TranscriptSegments))
	copy(segs, m.TranscriptSegments)
	total := 0
	for i := range segs {
		t, n := domainmeeting.ReplaceText(segs[i].Text, rule)
		if n == 0 {
			continue
		}
		segs[i].Text = t
		segs[i].MarkedText = ""       // 人工修正後原低信心標記失效
		segs[i].ManuallyEdited = true // AI 校正時不得覆蓋
		total += n
	}
	return segs, domainmeeting.FlattenSegments(segs), total
}

func replaceSections(in []domainmeeting.SummarySection, rule domainmeeting.ReplaceRule) ([]domainmeeting.SummarySection, int) {
	out := make([]domainmeeting.SummarySection, len(in))
	total := 0
	for i, s := range in {
		s.Items = append([]domainmeeting.SummaryPoint(nil), s.Items...)
		var n int
		s.Title, n = domainmeeting.ReplaceText(s.Title, rule)
		total += n
		for j := range s.Items {
			s.Items[j].Text, n = domainmeeting.ReplaceText(s.Items[j].Text, rule)
			total += n
			s.Items[j].Heading, n = domainmeeting.ReplaceText(s.Items[j].Heading, rule)
			total += n
		}
		out[i] = s
	}
	return out, total
}

func mapMeetingErr(err error) error {
	if errors.Is(err, domainmeeting.ErrNotFound) {
		return apperr.New(errcode.NotFound)
	}
	return apperr.Wrap(err, errcode.Internal)
}
