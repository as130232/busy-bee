package meeting

import "testing"

func TestStatus_CanTransitionTo(t *testing.T) {
	allowed := []struct{ from, to Status }{
		{StatusScheduled, StatusPending},        // 排程會議上傳了音訊
		{StatusPending, StatusTranscribing},     // worker 開始轉錄
		{StatusTranscribing, StatusAnalyzing},   // transcript 完成
		{StatusAnalyzing, StatusCompleted},      // 文件生成完成
		{StatusPending, StatusFailed},           // 任一階段失敗
		{StatusTranscribing, StatusFailed},
		{StatusAnalyzing, StatusFailed},
		{StatusFailed, StatusPending},           // 手動 retry
	}
	for _, tt := range allowed {
		if !tt.from.CanTransitionTo(tt.to) {
			t.Errorf("%s -> %s should be allowed", tt.from, tt.to)
		}
	}

	denied := []struct{ from, to Status }{
		{StatusCompleted, StatusPending},      // 完成的會議不可重跑
		{StatusCompleted, StatusFailed},
		{StatusScheduled, StatusTranscribing}, // 不可跳過 pending
		{StatusPending, StatusAnalyzing},      // 不可跳過 transcribing
		{StatusPending, StatusCompleted},
		{StatusScheduled, StatusScheduled},    // 不可自轉移
	}
	for _, tt := range denied {
		if tt.from.CanTransitionTo(tt.to) {
			t.Errorf("%s -> %s should be denied", tt.from, tt.to)
		}
	}
}

func TestStatus_IsValid(t *testing.T) {
	for _, s := range []Status{StatusScheduled, StatusPending, StatusTranscribing, StatusAnalyzing, StatusCompleted, StatusFailed} {
		if !s.IsValid() {
			t.Errorf("%s should be valid", s)
		}
	}
	if Status("bogus").IsValid() {
		t.Error("bogus status should be invalid")
	}
}
