package stt

import (
	"encoding/json"
	"testing"
)

func TestAggregateDeepgramWords(t *testing.T) {
	// 模擬 Deepgram 回應：講者 0 說兩個字、講者 1 說一句、講者 0 再接一句英文詞。
	const raw = `{
      "metadata": {"duration": 4.2},
      "results": {"channels": [{"alternatives": [{"words": [
        {"word": "你好", "punctuated_word": "你好，", "start": 0.0, "end": 0.6, "speaker": 0},
        {"word": "大家", "punctuated_word": "大家", "start": 0.6, "end": 1.1, "speaker": 0},
        {"word": "對", "punctuated_word": "對。", "start": 1.2, "end": 1.8, "speaker": 1},
        {"word": "deploy", "punctuated_word": "deploy", "start": 2.0, "end": 2.5, "speaker": 0},
        {"word": "完成", "punctuated_word": "完成", "start": 2.5, "end": 3.0, "speaker": 0}
      ]}]}]}
    }`

	var dr deepgramResponse
	if err := json.Unmarshal([]byte(raw), &dr); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	segs := aggregateDeepgramWords(dr)

	// 期望聚合成 3 段：A(你好，大家) → B(對。) → A(deploy 完成)
	if len(segs) != 3 {
		t.Fatalf("segments = %d, want 3: %+v", len(segs), segs)
	}
	if segs[0].Speaker != "A" || segs[0].Text != "你好，大家" {
		t.Errorf("seg0 = %+v, want A/你好，大家", segs[0])
	}
	if segs[1].Speaker != "B" || segs[1].Text != "對。" {
		t.Errorf("seg1 = %+v, want B/對。", segs[1])
	}
	// 英文詞與中文之間應加空白
	if segs[2].Speaker != "A" || segs[2].Text != "deploy 完成" {
		t.Errorf("seg2 = %+v, want A/'deploy 完成'", segs[2])
	}
	// 時間碼：seg0 從 0 到 1100ms
	if segs[0].StartMs != 0 || segs[0].EndMs != 1100 {
		t.Errorf("seg0 time = %d-%d, want 0-1100", segs[0].StartMs, segs[0].EndMs)
	}
}
