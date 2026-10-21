package correction

import (
	"strings"

	"github.com/as130232/busy-bee/busy-bee-be/domain/correction"
	"github.com/as130232/busy-bee/busy-bee-be/domain/meeting"
)

// 修正後文字與原文的長度比須落在此範圍，否則視為模型臆測補寫或大量刪減而丟棄。
const (
	minLenRatio = 0.5
	maxLenRatio = 2.0
)

// ValidatePatches 丟棄不合法的 patch：越界、鎖定段、空字串、與原文相同、長度比超出 [0.5, 2.0]。
func ValidatePatches(segs []meeting.TranscriptSegment, ch correction.Chunk, in []correction.Patch) []correction.Patch {
	out := make([]correction.Patch, 0, len(in))
	for _, p := range in {
		if p.I < ch.First || p.I > ch.Last || p.I < 0 || p.I >= len(segs) {
			continue
		}
		orig := segs[p.I]
		text := stripSpeakerPrefix(strings.TrimSpace(p.Text), orig.Speaker)
		if orig.ManuallyEdited || text == "" || text == orig.Text {
			continue
		}
		ratio := float64(len([]rune(text))) / float64(max(1, len([]rune(orig.Text))))
		if ratio < minLenRatio || ratio > maxLenRatio {
			continue
		}
		out = append(out, correction.Patch{I: p.I, Text: text})
	}
	return out
}

// stripSpeakerPrefix 去除模型抄進修正文字的「講者: 」前綴（prompt 以「講者: 文字」列出段落，模型偶爾整行照抄）。
// 只比對該段自己的講者代號，避免誤刪本來就以英文字開頭的內容。
func stripSpeakerPrefix(text, speaker string) string {
	if speaker == "" {
		return text
	}
	for _, sep := range []string{":", "："} {
		if rest, ok := strings.CutPrefix(text, speaker+sep); ok {
			return strings.TrimSpace(rest)
		}
	}
	return text
}

// GuidancePatches 合併使用者錯字對照與模型修正（original 為快照，applied 為已套用對照的版本）：
// 模型有改的段落以模型版本為準並再套一次對照（模型改回原錯字也會被修正），其餘只因對照而變的段落另產生 patch。
// 每段至多一個 patch；沒有對照時原樣回傳模型修正。
func GuidancePatches(original, applied []meeting.TranscriptSegment, llm []correction.Patch, g correction.Guidance) []correction.Patch {
	if len(g.Replacements) == 0 {
		return llm
	}
	byI := make(map[int]string, len(llm))
	for _, p := range llm {
		byI[p.I] = g.ApplyText(p.Text)
	}
	out := make([]correction.Patch, 0, len(llm))
	for i := range applied {
		text, ok := byI[i]
		if !ok {
			text = applied[i].Text
		}
		if text != original[i].Text {
			out = append(out, correction.Patch{I: i, Text: text})
		}
	}
	return out
}
