package llm

import (
	"testing"

	"google.golang.org/genai"
)

func TestParseExtraction(t *testing.T) {
	t.Run("純 JSON 物件（摘要 + 行動項）", func(t *testing.T) {
		out, err := parseExtraction(`{"summary":"討論規格","actionItems":[{"description":"整理規格","assignee":"小明","due":"下週五"}]}`)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if out.Summary != "討論規格" {
			t.Fatalf("summary = %q", out.Summary)
		}
		if len(out.Items) != 1 || out.Items[0].Description != "整理規格" || out.Items[0].Assignee != "小明" || out.Items[0].DueText != "下週五" {
			t.Fatalf("unexpected items: %+v", out.Items)
		}
	})

	t.Run("markdown fence 包裹", func(t *testing.T) {
		raw := "```json\n{\"summary\":\"寄送通知\",\"actionItems\":[{\"description\":\"寄信給客戶\",\"assignee\":\"\",\"due\":\"\"}]}\n```"
		out, err := parseExtraction(raw)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if out.Summary != "寄送通知" || len(out.Items) != 1 || out.Items[0].Description != "寄信給客戶" {
			t.Fatalf("unexpected: %+v", out)
		}
	})

	t.Run("無行動項（空陣列）", func(t *testing.T) {
		out, err := parseExtraction(`{"summary":"閒聊","actionItems":[]}`)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if out.Summary != "閒聊" || len(out.Items) != 0 {
			t.Fatalf("want summary + 0 items, got %+v", out)
		}
	})

	t.Run("壞 JSON 回錯", func(t *testing.T) {
		if _, err := parseExtraction("not json at all"); err == nil {
			t.Fatal("expected error for invalid JSON")
		}
	})
}

func TestGenerateConfig_ThinkingLevel(t *testing.T) {
	for _, m := range []string{"gemini-3-flash-preview", "gemini-3.1-flash-lite", "gemini-3.5-flash-lite"} {
		cfg := generateConfig(m)
		if cfg == nil || cfg.ThinkingConfig == nil || cfg.ThinkingConfig.ThinkingLevel != genai.ThinkingLevelLow {
			t.Errorf("%s 應設 thinkingLevel=LOW，got %+v", m, cfg)
		}
	}
	for _, m := range []string{"gemini-flash-latest", "gemini-2.5-flash"} {
		if cfg := generateConfig(m); cfg != nil {
			t.Errorf("%s 應維持供應商預設（nil），got %+v", m, cfg)
		}
	}
}

func TestExistingBlock(t *testing.T) {
	if existingBlock(nil) != "（無）" {
		t.Error("空清單應為（無）")
	}
	if got := existingBlock([]string{"a", "b"}); got != "- a\n- b" {
		t.Errorf("got %q", got)
	}
}
