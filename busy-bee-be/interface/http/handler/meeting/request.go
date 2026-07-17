package meeting

type createRequest struct {
	Title       string `json:"title"`
	ContentType string `json:"contentType"`
}
