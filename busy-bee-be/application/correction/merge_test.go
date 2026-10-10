package correction

import (
	"testing"

	"github.com/google/uuid"

	"github.com/as130232/busy-bee/busy-bee-be/domain/actionitem"
)

func item(desc, source string, done, edited bool) actionitem.ActionItem {
	return actionitem.ActionItem{ID: uuid.New(), Description: desc, Source: source, Done: done, Edited: edited}
}

func TestSplitActionItems(t *testing.T) {
	manual := item("手動", "manual", false, false)
	done := item("已勾", "llm", true, false)
	edited := item("改過", "llm", false, true)
	plain := item("原樣", "llm", false, false)
	kept, repl := SplitActionItems([]actionitem.ActionItem{manual, done, edited, plain})
	if len(kept) != 3 || len(repl) != 1 || repl[0].ID != plain.ID {
		t.Errorf("kept=%v repl=%v", kept, repl)
	}
}

func TestPlanMerge_DedupesAgainstKept(t *testing.T) {
	kept := []actionitem.ActionItem{item("整理規格書。", "manual", false, false), item("寄信", "llm", true, false)}
	repl := []actionitem.ActionItem{item("舊的", "llm", false, false)}
	ext := []actionitem.Extracted{{Description: "整理 規格書"}, {Description: "確認現金流"}, {Description: "確認現金流！"}, {Description: "  "}}
	got := PlanMerge(kept, repl, ext)
	if len(got.DeleteIDs) != 1 || got.DeleteIDs[0] != repl[0].ID {
		t.Errorf("DeleteIDs = %v", got.DeleteIDs)
	}
	if len(got.Insert) != 1 || got.Insert[0].Description != "確認現金流" {
		t.Errorf("Insert = %+v（只差空白/標點者應去重、空白描述略過）", got.Insert)
	}
	if got.InsertSortFrom != 2 {
		t.Errorf("InsertSortFrom = %d, want 2（接在保留項後）", got.InsertSortFrom)
	}
}
