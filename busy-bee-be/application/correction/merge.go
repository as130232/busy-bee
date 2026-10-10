package correction

import (
	"strings"
	"unicode"

	"github.com/google/uuid"

	"github.com/as130232/busy-bee/busy-bee-be/domain/actionitem"
)

// MergePlan 校正後行動項的替換計畫。
type MergePlan struct {
	DeleteIDs      []uuid.UUID
	Insert         []actionitem.Extracted
	InsertSortFrom int
}

// SplitActionItems 手動新增 / 已勾選 / 改過內容 → 保留；其餘 AI 抽取項 → 可覆蓋。
func SplitActionItems(items []actionitem.ActionItem) (kept, replaceable []actionitem.ActionItem) {
	for _, it := range items {
		if it.Source == "manual" || it.Done || it.Edited {
			kept = append(kept, it)
		} else {
			replaceable = append(replaceable, it)
		}
	}
	return kept, replaceable
}

// PlanMerge 刪可覆蓋項、插入新抽取項（與保留項或彼此正規化後相同者略過），新項排在保留項之後。
func PlanMerge(kept, replaceable []actionitem.ActionItem, extracted []actionitem.Extracted) MergePlan {
	seen := make(map[string]bool, len(kept))
	for _, k := range kept {
		seen[normalizeDesc(k.Description)] = true
	}
	plan := MergePlan{InsertSortFrom: len(kept)}
	for _, r := range replaceable {
		plan.DeleteIDs = append(plan.DeleteIDs, r.ID)
	}
	for _, e := range extracted {
		key := normalizeDesc(e.Description)
		if key == "" || seen[key] {
			continue
		}
		seen[key] = true
		plan.Insert = append(plan.Insert, e)
	}
	return plan
}

// normalizeDesc 去除空白與標點、轉小寫，供去重比較。
func normalizeDesc(s string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(s) {
		if unicode.IsSpace(r) || unicode.IsPunct(r) {
			continue
		}
		b.WriteRune(r)
	}
	return b.String()
}
