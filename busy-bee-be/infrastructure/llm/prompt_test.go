package llm

import (
	"strings"
	"testing"
)

func TestBuildPrompt_PRD(t *testing.T) {
	p, err := buildPrompt(promptPRD, "今天討論登入功能")
	if err != nil {
		t.Fatalf("buildPrompt error = %v", err)
	}
	for _, want := range []string{
		"今天討論登入功能",  // transcript 已代入
		"背景與問題",        // PRD 章節骨架
		"決議與行動項",
		"會議未討論",        // 防幻覺規則
		"禁止捏造",
	} {
		if !strings.Contains(p, want) {
			t.Errorf("prd prompt missing %q", want)
		}
	}
	if strings.Contains(p, "{{TRANSCRIPT}}") {
		t.Error("placeholder not replaced")
	}
}

func TestBuildPrompt_TechSpec(t *testing.T) {
	p, err := buildPrompt(promptTechSpec, "transcript-x")
	if err != nil {
		t.Fatalf("buildPrompt error = %v", err)
	}
	for _, want := range []string{"transcript-x", "技術方案概述", "風險與緩解", "會議未討論"} {
		if !strings.Contains(p, want) {
			t.Errorf("tech spec prompt missing %q", want)
		}
	}
}
