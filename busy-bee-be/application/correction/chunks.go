// Package correction AI 錄音重新校正（F-CORRECT）use cases 與純函式（切段、patch、行動項合併）。
package correction

import (
	"github.com/as130232/busy-bee/busy-bee-be/domain/correction"
	"github.com/as130232/busy-bee/busy-bee-be/domain/meeting"
)

const (
	chunkTargetMs   = 720_000 // 每段約 12 分鐘
	overlapMs       = 30_000  // 音訊片段往前重疊，讓模型聽到跨段語句開頭
	contextSegments = 3       // 上一段結尾帶入的唯讀段落數
)

// PlanChunks 依 segment 時間切段：累積到「再加下一個 segment 會超過 chunkTargetMs」即切，
// 切點必在 segment 邊界；單一 segment 超長時自成一段。非首段的音訊往前重疊 overlapMs。
func PlanChunks(segs []meeting.TranscriptSegment) []correction.Chunk {
	var out []correction.Chunk
	first := 0
	for i := range segs {
		isLast := i == len(segs)-1
		if !isLast && segs[i+1].EndMs-segs[first].StartMs <= chunkTargetMs {
			continue
		}
		from := segs[0].StartMs // 第一段無前文可重疊
		if first > 0 {
			from = max(0, segs[first].StartMs-overlapMs)
		}
		out = append(out, correction.Chunk{
			Index: len(out), First: first, Last: i,
			ClipFromMs: from, ClipToMs: segs[i].EndMs,
		})
		first = i + 1
	}
	return out
}
