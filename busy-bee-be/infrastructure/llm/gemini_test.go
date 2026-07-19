package llm

import "testing"

func TestParseActionItems(t *testing.T) {
	t.Run("純 JSON 陣列", func(t *testing.T) {
		items, err := parseActionItems(`[{"description":"整理規格","assignee":"小明","due":"下週五"}]`)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(items) != 1 {
			t.Fatalf("want 1 item, got %d", len(items))
		}
		if items[0].Description != "整理規格" || items[0].Assignee != "小明" || items[0].DueText != "下週五" {
			t.Fatalf("unexpected item: %+v", items[0])
		}
	})

	t.Run("markdown fence 包裹", func(t *testing.T) {
		raw := "```json\n[{\"description\":\"寄信給客戶\",\"assignee\":\"\",\"due\":\"\"}]\n```"
		items, err := parseActionItems(raw)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(items) != 1 || items[0].Description != "寄信給客戶" {
			t.Fatalf("unexpected items: %+v", items)
		}
	})

	t.Run("空陣列", func(t *testing.T) {
		items, err := parseActionItems("[]")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(items) != 0 {
			t.Fatalf("want 0 items, got %d", len(items))
		}
	})

	t.Run("壞 JSON 回錯", func(t *testing.T) {
		if _, err := parseActionItems("not json at all"); err == nil {
			t.Fatal("expected error for invalid JSON")
		}
	})
}
