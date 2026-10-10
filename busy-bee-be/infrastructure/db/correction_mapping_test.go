package db

import (
	"testing"

	domaincorrection "github.com/as130232/busy-bee/busy-bee-be/domain/correction"
	"github.com/as130232/busy-bee/busy-bee-be/infrastructure/db/sqlcgen"
)

func TestToDomainCorrection(t *testing.T) {
	got, err := toDomainCorrection(sqlcgen.TranscriptCorrection{
		Attempt:                 2,
		Status:                  "running",
		Progress:                []byte(`{"chunks":{"0":[{"i":3,"text":"小王"}]}}`),
		OriginalSegments:        []byte(`[{"speaker":"A","text":"小娃","startMs":0,"endMs":1}]`),
		OriginalTranscript:      "A: 小娃",
		OriginalSummarySections: []byte(`[]`),
		OriginalSummary:         "舊",
	})
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if got.Attempt != 2 || got.Status != domaincorrection.StatusRunning || got.Original.Summary != "舊" {
		t.Errorf("got %+v", got)
	}
	if p := got.Progress.Chunks["0"]; len(p) != 1 || p[0].I != 3 || p[0].Text != "小王" {
		t.Errorf("progress = %+v", got.Progress)
	}
	if len(got.Original.Segments) != 1 || got.Original.Segments[0].Text != "小娃" {
		t.Errorf("segments = %+v", got.Original.Segments)
	}
}

func TestToDomainCorrection_EmptyProgressInitialisesMap(t *testing.T) {
	got, err := toDomainCorrection(sqlcgen.TranscriptCorrection{Progress: []byte(`{}`), OriginalSegments: []byte(`[]`), OriginalSummarySections: []byte(`[]`)})
	if err != nil || got.Progress.Chunks == nil {
		t.Errorf("Chunks 應初始化為空 map，got %+v err %v", got.Progress, err)
	}
}
