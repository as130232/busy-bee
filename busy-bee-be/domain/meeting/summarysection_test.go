package meeting

import (
	"encoding/json"
	"testing"
)

// 物件形式：heading/text/speaker 完整解析。
func TestSummaryPointUnmarshalObject(t *testing.T) {
	var p SummaryPoint
	raw := `{"text":"與會者同意一基本方案","heading":"採用基本盤加人頭","speaker":"A"}`
	if err := json.Unmarshal([]byte(raw), &p); err != nil {
		t.Fatalf("unmarshal object: %v", err)
	}
	if p.Text != "與會者同意一基本方案" || p.Heading != "採用基本盤加人頭" || p.Speaker != "A" {
		t.Errorf("got %+v", p)
	}
}

// 裸字串形式：LLM 偶爾回字串，應收斂成 {Text: s}，不整段失敗。
func TestSummaryPointUnmarshalBareString(t *testing.T) {
	var p SummaryPoint
	if err := json.Unmarshal([]byte(`"討論登入流程改版方向"`), &p); err != nil {
		t.Fatalf("unmarshal bare string: %v", err)
	}
	if p.Text != "討論登入流程改版方向" || p.Heading != "" || p.Speaker != "" {
		t.Errorf("got %+v", p)
	}
}

// section 內混合裸字串與物件也要能解析（prompt 漂移容忍）。
func TestSummarySectionUnmarshalMixedItems(t *testing.T) {
	var s SummarySection
	raw := `{"type":"topics","title":"討論主題","items":["純條列重點",{"heading":"定價模式討論","text":"基本方案799含2管理員","speaker":"A"}]}`
	if err := json.Unmarshal([]byte(raw), &s); err != nil {
		t.Fatalf("unmarshal section: %v", err)
	}
	if s.Type != "topics" || s.Title != "討論主題" {
		t.Errorf("section meta got %+v", s)
	}
	if len(s.Items) != 2 {
		t.Fatalf("want 2 items, got %d", len(s.Items))
	}
	if s.Items[0].Text != "純條列重點" || s.Items[0].Heading != "" || s.Items[0].Speaker != "" {
		t.Errorf("item0 got %+v", s.Items[0])
	}
	if s.Items[1].Heading != "定價模式討論" || s.Items[1].Text != "基本方案799含2管理員" || s.Items[1].Speaker != "A" {
		t.Errorf("item1 got %+v", s.Items[1])
	}
}
