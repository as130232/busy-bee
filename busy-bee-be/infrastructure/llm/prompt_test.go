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

// 逐字稿會帶 STT 低信心標記「〔?…〕」，所有吃逐字稿的模板都必須說明其意義與「待確認」規則。
func TestPrompts_ExplainLowConfidenceMarkers(t *testing.T) {
	paths := []string{promptActionItems}
	for _, p := range scenarioPrompts {
		paths = append(paths, p)
	}
	for _, p := range paths {
		raw, err := promptFS.ReadFile(p)
		if err != nil {
			t.Fatalf("read %s: %v", p, err)
		}
		s := string(raw)
		if !strings.Contains(s, "〔?…〕") || !strings.Contains(s, "（語音不清，待確認）") {
			t.Errorf("%s 缺少低信心標記說明", p)
		}
	}
}
