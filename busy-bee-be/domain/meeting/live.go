package meeting

import "time"

// LiveNotes 即時會議（F-LIVE）結束時保存的「會議中筆記」：最後一版即時重點與提問紀錄。
// 即時逐字稿不保存（以會後正式版為準）。
type LiveNotes struct {
	Notes LiveSummary `json:"notes"`
	QA    []LiveQA    `json:"qa"`
}

// LiveSummary 即時重點：目前在討論什麼（一句話）＋已談到的重點。
type LiveSummary struct {
	Current string   `json:"current"`
	Points  []string `json:"points"`
}

// LiveQA 會議中的一輪提問。
type LiveQA struct {
	Question string       `json:"question"`
	Answer   string       `json:"answer"`
	Sources  []LiveSource `json:"sources"`
	AskedAt  time.Time    `json:"askedAt"`
}

// LiveSource 回答的依據：Type 為 transcript（Label = 逐字稿時間 HH:MM）或 context（Label = 會前資料標題）。
type LiveSource struct {
	Type  string `json:"type"`
	Label string `json:"label"`
}
