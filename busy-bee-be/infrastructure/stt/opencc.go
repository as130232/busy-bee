package stt

import "github.com/longbridgeapp/opencc"

// s2tw 簡體→台灣繁體（含慣用詞彙轉換，如「軟件」→「軟體」）；詞典由 go:embed 內嵌，無外部檔案依賴。
// 初始化失敗時保持 nil，toTraditional 原文降級（best-effort，不影響主流程）。
var s2tw *opencc.OpenCC

func init() {
	if c, err := opencc.New("s2twp"); err == nil {
		s2tw = c
	}
}

// toTraditional 供 auto 語言（Deepgram detect_language=true）偵測為中文時的簡轉繁後製；
// 未初始化或轉換失敗時原文降級，不中斷轉錄流程。
func toTraditional(s string) string {
	if s2tw == nil || s == "" {
		return s
	}
	out, err := s2tw.Convert(s)
	if err != nil {
		return s
	}
	return out
}
