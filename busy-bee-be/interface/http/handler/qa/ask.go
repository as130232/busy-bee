package qa

import (
	"strings"

	"github.com/gin-gonic/gin"

	domainuser "github.com/as130232/busy-bee/busy-bee-be/domain/user"
	"github.com/as130232/busy-bee/busy-bee-be/interface/http/response"
	"github.com/as130232/busy-bee/busy-bee-be/pkg/apperr"
	"github.com/as130232/busy-bee/busy-bee-be/pkg/consts/errcode"
)

// Ask POST /api/v1/meetings/qa — 跨會議 RAG 問答（單次、無狀態）。
// 依問題語意檢索本人全部會議片段，用 LLM 生成帶 [n] 引用的答案。
func (h *Handler) Ask(c *gin.Context) {
	userID, ok := domainuser.IDFrom(c.Request.Context())
	if !ok {
		response.Fail(c, apperr.New(errcode.Unauthorized))
		return
	}

	var req askRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, apperr.Wrap(err, errcode.Param, "question"))
		return
	}
	question := strings.TrimSpace(req.Question)
	if question == "" {
		response.Fail(c, apperr.New(errcode.Param, "question"))
		return
	}
	// 上界保護：以 rune 截斷過長問題，避免超長輸入打進向量嵌入與 LLM（成本護欄）。
	if r := []rune(question); len(r) > maxQuestionLen {
		question = string(r[:maxQuestionLen])
	}

	res, err := h.uc.Execute(c.Request.Context(), userID, question)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, toAnswerResponse(res))
}
