package correction

import (
	"slices"
	"strings"
	"unicode/utf8"

	"github.com/as130232/busy-bee/busy-bee-be/domain/meeting"
)

const (
	// MaxGuidanceRules 錯字對照上限。
	MaxGuidanceRules = 20
	// MaxGuidanceNoteRunes 補充說明字數上限。
	MaxGuidanceNoteRunes = 1000
)

// Guidance 使用者在 AI 校正前的選填補充：錯字對照在送模型前先直接套用（結果穩定），
// 補充說明只放進校正 prompt 幫模型辨認用詞與理解上下文——不送進摘要與行動項，不會寫進報告。
// 隨校正紀錄保存，重試從斷點接續時能重建同樣的輸入。
type Guidance struct {
	Replacements []meeting.ReplaceRule `json:"replacements,omitempty"`
	Note         string                `json:"note,omitempty"`
}

// Normalize 去空白、略過兩邊皆空的列；有不合法的列或超過上限時 ok=false。
func (g Guidance) Normalize() (Guidance, bool) {
	out := Guidance{Note: strings.TrimSpace(g.Note)}
	for _, r := range g.Replacements {
		r = r.Normalize()
		if r.From == "" && r.To == "" {
			continue
		}
		if !r.Valid() {
			return Guidance{}, false
		}
		out.Replacements = append(out.Replacements, r)
	}
	if len(out.Replacements) > MaxGuidanceRules || utf8.RuneCountInString(out.Note) > MaxGuidanceNoteRunes {
		return Guidance{}, false
	}
	return out, true
}

// ApplyText 依序套用所有錯字對照。
func (g Guidance) ApplyText(s string) string {
	for _, r := range g.Replacements {
		s, _ = meeting.ReplaceText(s, r)
	}
	return s
}

// Apply 回傳套用錯字對照後的副本；人工修正過的段落不動（與 ApplyPatches 一致）。
// 文字有變的段落清掉低信心標記（已非 STT 原文）。
func (g Guidance) Apply(segs []meeting.TranscriptSegment) []meeting.TranscriptSegment {
	out := slices.Clone(segs)
	if len(g.Replacements) == 0 {
		return out
	}
	for i := range out {
		if out[i].ManuallyEdited {
			continue
		}
		if t := g.ApplyText(out[i].Text); t != out[i].Text {
			out[i].Text = t
			out[i].MarkedText = ""
		}
	}
	return out
}
