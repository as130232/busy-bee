package llm

import (
	"strings"
	"testing"
)

func TestBuildPrompt_ReplacesTranscriptPlaceholder(t *testing.T) {
	p, err := buildPrompt(promptActionItems, "今天討論登入功能")
	if err != nil {
		t.Fatalf("buildPrompt error = %v", err)
	}
	if !strings.Contains(p, "今天討論登入功能") {
		t.Error("transcript 未代入")
	}
	if strings.Contains(p, "{{TRANSCRIPT}}") {
		t.Error("placeholder not replaced")
	}
}

func TestBuildPrompt_MissingTemplateErrors(t *testing.T) {
	if _, err := buildPrompt("prompts/does_not_exist.md", "x"); err == nil {
		t.Error("expected error for missing template")
	}
}
