//go:build integration

package db

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"

	domainactionitem "github.com/as130232/busy-bee/busy-bee-be/domain/actionitem"
	domaincorrection "github.com/as130232/busy-bee/busy-bee-be/domain/correction"
	domainmeeting "github.com/as130232/busy-bee/busy-bee-be/domain/meeting"
)

func TestCorrectionRepo_CreateLimitsAndActive(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	u := testUser(t, pool)
	m, err := NewMeetingRepo(pool).Create(ctx, domainmeeting.Meeting{UserID: u.ID, Title: "校正", Status: domainmeeting.StatusCompleted, Transcript: "t"})
	if err != nil {
		t.Fatalf("create meeting: %v", err)
	}
	repo := NewCorrectionRepo(pool)
	snap := domaincorrection.Snapshot{Transcript: "t"}

	c1, err := repo.CreateWithSnapshot(ctx, m.ID, u.ID, snap, domaincorrection.Guidance{})
	if err != nil || c1.Attempt != 1 || c1.Status != domaincorrection.StatusQueued {
		t.Fatalf("first create = %+v err %v", c1, err)
	}
	if _, err := repo.CreateWithSnapshot(ctx, m.ID, u.ID, snap, domaincorrection.Guidance{}); !errors.Is(err, domaincorrection.ErrActiveExists) {
		t.Fatalf("進行中再建立應 ErrActiveExists，got %v", err)
	}
	if _, err := repo.UpdateStatus(ctx, c1.ID, domaincorrection.StatusQueued, domaincorrection.StatusFailed, "x"); err != nil {
		t.Fatalf("fail c1: %v", err)
	}
	c2, err := repo.CreateWithSnapshot(ctx, m.ID, u.ID, snap, domaincorrection.Guidance{})
	if err != nil || c2.Attempt != 2 {
		t.Fatalf("second create = %+v err %v", c2, err)
	}
	if _, err := repo.UpdateStatus(ctx, c2.ID, domaincorrection.StatusQueued, domaincorrection.StatusFailed, "x"); err != nil {
		t.Fatalf("fail c2: %v", err)
	}
	if _, err := repo.CreateWithSnapshot(ctx, m.ID, u.ID, snap, domaincorrection.Guidance{}); !errors.Is(err, domaincorrection.ErrStatusConflict) {
		t.Fatalf("超過上限應 ErrStatusConflict，got %v", err)
	}
	if n, _ := repo.CountForMeeting(ctx, m.ID, u.ID); n != 2 {
		t.Errorf("count = %d, want 2", n)
	}
	latest, err := repo.GetLatestForMeeting(ctx, m.ID, u.ID)
	if err != nil || latest.ID != c2.ID || latest.ErrorMessage != "x" {
		t.Errorf("latest = %+v err %v", latest, err)
	}
}

func TestCorrectionRepo_ApplyAndRevert(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	u := testUser(t, pool)
	meetings := NewMeetingRepo(pool)
	items := NewActionItemRepo(pool)
	repo := NewCorrectionRepo(pool)

	m, err := meetings.Create(ctx, domainmeeting.Meeting{UserID: u.ID, Title: "校正", Status: domainmeeting.StatusCompleted, Transcript: "A: 小娃"})
	if err != nil {
		t.Fatalf("create meeting: %v", err)
	}
	origSegs := []domainmeeting.TranscriptSegment{{Speaker: "A", Text: "小娃"}}
	origSecs := []domainmeeting.SummarySection{{Type: "summary", Title: "摘要", Items: []domainmeeting.SummaryPoint{{Text: "舊重點"}}}}
	if _, err := meetings.UpdateTranscriptSegments(ctx, m.ID, u.ID, origSegs, "A: 小娃"); err != nil {
		t.Fatal(err)
	}
	if _, err := meetings.SaveSummarySections(ctx, m.ID, u.ID, origSecs); err != nil {
		t.Fatal(err)
	}
	if _, err := meetings.SaveSummary(ctx, m.ID, u.ID, "舊"); err != nil {
		t.Fatal(err)
	}
	llmItem, _ := items.Insert(ctx, m.ID, u.ID, domainactionitem.Extracted{Description: "舊項"}, 0)
	if _, err := items.InsertManual(ctx, m.ID, u.ID, "手動項", ""); err != nil {
		t.Fatal(err)
	}

	c, err := repo.CreateWithSnapshot(ctx, m.ID, u.ID, domaincorrection.Snapshot{
		Segments: origSegs, Transcript: "A: 小娃", SummarySections: origSecs, Summary: "舊",
	}, domaincorrection.Guidance{})
	if err != nil {
		t.Fatal(err)
	}
	if c, err = repo.UpdateStatus(ctx, c.ID, domaincorrection.StatusQueued, domaincorrection.StatusRunning, ""); err != nil {
		t.Fatal(err)
	}
	c.Progress.Chunks["0"] = []domaincorrection.Patch{{I: 0, Text: "小王"}}
	if err := repo.SaveProgress(ctx, c.ID, c.Progress); err != nil {
		t.Fatal(err)
	}
	if got, _ := repo.Get(ctx, c.ID); len(got.Progress.Chunks["0"]) != 1 {
		t.Errorf("progress 未落庫：%+v", got.Progress)
	}

	if err := repo.ApplyResult(ctx, c, domaincorrection.Result{
		Patches:         []domaincorrection.Patch{{I: 0, Text: "小王"}},
		SummarySections: []domainmeeting.SummarySection{{Type: "summary", Title: "摘要", Items: []domainmeeting.SummaryPoint{{Text: "新重點"}}}},
		Summary:         "新",
		DeleteItemIDs:   []uuid.UUID{llmItem.ID},
		InsertItems:     []domainactionitem.Extracted{{Description: "新項"}},
		InsertSortFrom:  1,
	}); err != nil {
		t.Fatalf("apply: %v", err)
	}
	got, _ := meetings.GetForUser(ctx, m.ID, u.ID)
	if got.TranscriptSegments[0].Text != "小王" || got.TranscriptSegments[0].OriginalText != "小娃" || got.Summary != "新" || got.SummarySections[0].Items[0].Text != "新重點" {
		t.Errorf("apply 後 meeting = %+v", got)
	}
	list, _ := items.ListByMeeting(ctx, m.ID)
	descs := map[string]bool{}
	for _, it := range list {
		descs[it.Description] = true
	}
	if len(list) != 2 || !descs["手動項"] || !descs["新項"] || descs["舊項"] {
		t.Errorf("apply 後行動項 = %v", descs)
	}
	if cc, _ := repo.Get(ctx, c.ID); cc.Status != domaincorrection.StatusCompleted || cc.CompletedAt == nil {
		t.Errorf("status = %s", cc.Status)
	}

	c.Status = domaincorrection.StatusCompleted
	if err := repo.Revert(ctx, c); err != nil {
		t.Fatalf("revert: %v", err)
	}
	got, _ = meetings.GetForUser(ctx, m.ID, u.ID)
	if got.TranscriptSegments[0].Text != "小娃" || got.TranscriptSegments[0].OriginalText != "" || got.Summary != "舊" || got.SummarySections[0].Items[0].Text != "舊重點" {
		t.Errorf("revert 後 meeting = %+v", got)
	}
	if after, _ := items.ListByMeeting(ctx, m.ID); len(after) != 2 {
		t.Errorf("還原不應動行動項，got %d 筆", len(after))
	}
	if cc, _ := repo.Get(ctx, c.ID); cc.Status != domaincorrection.StatusReverted || cc.RevertedAt == nil {
		t.Errorf("status = %s", cc.Status)
	}
	if err := repo.Revert(ctx, c); !errors.Is(err, domaincorrection.ErrStatusConflict) {
		t.Errorf("重複還原應 ErrStatusConflict，got %v", err)
	}
}

func TestActionItemRepo_UpdateDescriptionMarksEdited(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	u := testUser(t, pool)
	m, _ := NewMeetingRepo(pool).Create(ctx, domainmeeting.Meeting{UserID: u.ID, Title: "x", Status: domainmeeting.StatusCompleted})
	items := NewActionItemRepo(pool)
	it, _ := items.Insert(ctx, m.ID, u.ID, domainactionitem.Extracted{Description: "原"}, 0)
	if it.Edited || it.Source != "llm" {
		t.Fatalf("新建項 = %+v", it)
	}
	upd, err := items.UpdateDescription(ctx, it.ID, u.ID, "改")
	if err != nil || !upd.Edited {
		t.Errorf("改內容後應 Edited，got %+v err %v", upd, err)
	}
}

// 審查 C1/C2/I1：收尾與還原都以交易內「當下」資料為準，使用者在 LLM 呼叫期間的編輯/勾選不被覆蓋或刪除。
func TestCorrectionRepo_ApplyAndRevertRespectConcurrentUserChanges(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	u := testUser(t, pool)
	meetings := NewMeetingRepo(pool)
	items := NewActionItemRepo(pool)
	repo := NewCorrectionRepo(pool)

	m, _ := meetings.Create(ctx, domainmeeting.Meeting{UserID: u.ID, Title: "併發", Status: domainmeeting.StatusCompleted})
	orig := []domainmeeting.TranscriptSegment{{Speaker: "A", Text: "小娃"}, {Speaker: "B", Text: "接流"}}
	if _, err := meetings.UpdateTranscriptSegments(ctx, m.ID, u.ID, orig, "x"); err != nil {
		t.Fatal(err)
	}
	ai, _ := items.Insert(ctx, m.ID, u.ID, domainactionitem.Extracted{Description: "AI 項"}, 0)

	c, _ := repo.CreateWithSnapshot(ctx, m.ID, u.ID, domaincorrection.Snapshot{Segments: orig, Transcript: "x"}, domaincorrection.Guidance{})
	c, _ = repo.UpdateStatus(ctx, c.ID, domaincorrection.StatusQueued, domaincorrection.StatusRunning, "")

	// worker 讀完後、寫回前：使用者手改段落 1、勾選 AI 項
	edited := []domainmeeting.TranscriptSegment{orig[0], {Speaker: "B", Text: "使用者改的", ManuallyEdited: true}}
	if _, err := meetings.UpdateTranscriptSegments(ctx, m.ID, u.ID, edited, "y"); err != nil {
		t.Fatal(err)
	}
	if _, err := items.SetDone(ctx, ai.ID, u.ID, true); err != nil {
		t.Fatal(err)
	}

	if err := repo.ApplyResult(ctx, c, domaincorrection.Result{
		Patches:       []domaincorrection.Patch{{I: 0, Text: "小王"}, {I: 1, Text: "金流"}},
		DeleteItemIDs: []uuid.UUID{ai.ID},
	}); err != nil {
		t.Fatalf("apply: %v", err)
	}
	got, _ := meetings.GetForUser(ctx, m.ID, u.ID)
	if got.TranscriptSegments[0].Text != "小王" || got.TranscriptSegments[1].Text != "使用者改的" || !got.TranscriptSegments[1].ManuallyEdited {
		t.Errorf("apply 後 segments = %+v（使用者的編輯應保留）", got.TranscriptSegments)
	}
	if got.Transcript != "A: 小王\nB: 使用者改的" {
		t.Errorf("transcript = %q", got.Transcript)
	}
	if list, _ := items.ListByMeeting(ctx, m.ID); len(list) != 1 {
		t.Errorf("已勾選的 AI 項不應被刪，剩 %d 筆", len(list))
	}

	// 校正後使用者又手改段落 0 → 還原時保留
	after := []domainmeeting.TranscriptSegment{{Speaker: "A", Text: "校正後手改", ManuallyEdited: true, OriginalText: "小娃"}, got.TranscriptSegments[1]}
	if _, err := meetings.UpdateTranscriptSegments(ctx, m.ID, u.ID, after, "z"); err != nil {
		t.Fatal(err)
	}
	c.Status = domaincorrection.StatusCompleted
	if err := repo.Revert(ctx, c); err != nil {
		t.Fatalf("revert: %v", err)
	}
	got, _ = meetings.GetForUser(ctx, m.ID, u.ID)
	if got.TranscriptSegments[0].Text != "校正後手改" || got.TranscriptSegments[0].OriginalText != "" {
		t.Errorf("還原應保留校正後的手動修改：%+v", got.TranscriptSegments[0])
	}
	if got.TranscriptSegments[1].Text != "使用者改的" {
		t.Errorf("seg1 = %+v", got.TranscriptSegments[1])
	}
	if got.Transcript != "A: 校正後手改\nB: 使用者改的" {
		t.Errorf("transcript = %q", got.Transcript)
	}
}

// 使用者補充隨校正紀錄保存，worker 重試時讀回同樣內容。
func TestCorrectionRepo_GuidanceRoundTrip(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	u := testUser(t, pool)
	m, err := NewMeetingRepo(pool).Create(ctx, domainmeeting.Meeting{UserID: u.ID, Title: "補充", Status: domainmeeting.StatusCompleted, Transcript: "t"})
	if err != nil {
		t.Fatal(err)
	}
	repo := NewCorrectionRepo(pool)
	g := domaincorrection.Guidance{Replacements: []domainmeeting.ReplaceRule{{From: "DBA", To: "DVA"}}, Note: "內部代號"}
	c, err := repo.CreateWithSnapshot(ctx, m.ID, u.ID, domaincorrection.Snapshot{Transcript: "t"}, g)
	if err != nil {
		t.Fatal(err)
	}
	got, err := repo.Get(ctx, c.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Guidance.Replacements) != 1 || got.Guidance.Replacements[0].To != "DVA" || got.Guidance.Note != "內部代號" {
		t.Errorf("guidance = %+v", got.Guidance)
	}
}
