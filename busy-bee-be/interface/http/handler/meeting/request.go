package meeting

type createRequest struct {
	Title       string `json:"title"`
	ContentType string `json:"contentType"`
	// Scenario 紀錄情境（meeting/casual）；省略或無效值後端回退 meeting。
	Scenario string `json:"scenario"`
}
