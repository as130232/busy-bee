package llm

import (
	"strings"
	"testing"

	domainmeeting "github.com/as130232/busy-bee/busy-bee-be/domain/meeting"
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

// 會議摘要須保留細節（2026-10 使用者回饋：舊版過度概括），並獨立列出流程細節與待確認事項。
func TestMeetingPrompt_RequiresDetailSections(t *testing.T) {
	raw, err := promptFS.ReadFile(scenarioPrompts[domainmeeting.ScenarioMeeting])
	if err != nil {
		t.Fatal(err)
	}
	s := string(raw)
	for _, want := range []string{`"details"`, `"open_questions"`, "具體性規則", "禁止"} {
		if !strings.Contains(s, want) {
			t.Errorf("會議 prompt 缺少 %q", want)
		}
	}
	if strings.Contains(s, "≤50 字") {
		t.Error("會議 prompt 不應再以 50 字上限壓縮細節")
	}
}
