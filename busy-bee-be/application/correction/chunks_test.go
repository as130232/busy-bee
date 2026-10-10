package correction

import (
	"reflect"
	"testing"

	"github.com/as130232/busy-bee/busy-bee-be/domain/correction"
	"github.com/as130232/busy-bee/busy-bee-be/domain/meeting"
)

func seg(startMs, endMs int) meeting.TranscriptSegment {
	return meeting.TranscriptSegment{Speaker: "A", Text: "x", StartMs: startMs, EndMs: endMs}
}

func TestPlanChunks_SplitsAtSegmentBoundaries(t *testing.T) {
	// 每段 5 分鐘、共 5 段（25 分鐘）。累積區間（首段起點～末段終點）不超過 12 分鐘：
	// [0,1]=10 分、加 2 會變 15 分 → 切；[2,3]=10 分；[4]。非首段音訊往前重疊 30 秒。
	segs := []meeting.TranscriptSegment{seg(0, 300_000), seg(300_000, 600_000), seg(600_000, 900_000), seg(900_000, 1_200_000), seg(1_200_000, 1_500_000)}
	got := PlanChunks(segs)
	want := []correction.Chunk{
		{Index: 0, First: 0, Last: 1, ClipFromMs: 0, ClipToMs: 600_000},
		{Index: 1, First: 2, Last: 3, ClipFromMs: 570_000, ClipToMs: 1_200_000},
		{Index: 2, First: 4, Last: 4, ClipFromMs: 1_170_000, ClipToMs: 1_500_000},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %+v\nwant %+v", got, want)
	}
}

func TestPlanChunks_SingleLongSegmentIsOwnChunk(t *testing.T) {
	got := PlanChunks([]meeting.TranscriptSegment{seg(0, 1_000_000), seg(1_000_000, 1_010_000)})
	if len(got) != 2 || got[0].First != 0 || got[0].Last != 0 || got[1].First != 1 {
		t.Errorf("got %+v", got)
	}
}

func TestPlanChunks_Empty(t *testing.T) {
	if got := PlanChunks(nil); len(got) != 0 {
		t.Errorf("got %+v", got)
	}
}
