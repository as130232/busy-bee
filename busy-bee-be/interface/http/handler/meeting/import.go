package meeting

import (
	"github.com/gin-gonic/gin"

	appmeeting "github.com/as130232/busy-bee/busy-bee-be/application/meeting"
	domainuser "github.com/as130232/busy-bee/busy-bee-be/domain/user"
	"github.com/as130232/busy-bee/busy-bee-be/interface/http/response"
	"github.com/as130232/busy-bee/busy-bee-be/pkg/apperr"
	"github.com/as130232/busy-bee/busy-bee-be/pkg/consts/errcode"
)

type importRequest struct {
	URL      string `json:"url"`
	Title    string `json:"title"`
	Scenario string `json:"scenario"`
}

// Import POST /api/v1/meetings/import — 由外部連結（YouTube/Podcast/直接音檔）匯入，音訊由 worker 抓取後跑管線。
func (h *Handler) Import(c *gin.Context) {
	userID, ok := domainuser.IDFrom(c.Request.Context())
	if !ok {
		response.Fail(c, apperr.New(errcode.Unauthorized))
		return
	}

	var req importRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, apperr.Wrap(err, errcode.Param, "body"))
		return
	}

	m, err := h.uc.Import.Execute(c.Request.Context(), userID, appmeeting.ImportInput{
		URL:      req.URL,
		Title:    req.Title,
		Scenario: req.Scenario,
	})
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, gin.H{"meeting": toMeetingResponse(m)})
}
