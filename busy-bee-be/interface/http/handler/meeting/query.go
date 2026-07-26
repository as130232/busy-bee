package meeting

import (
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	domainuser "github.com/as130232/busy-bee/busy-bee-be/domain/user"
	"github.com/as130232/busy-bee/busy-bee-be/interface/http/response"
	"github.com/as130232/busy-bee/busy-bee-be/pkg/apperr"
	"github.com/as130232/busy-bee/busy-bee-be/pkg/consts/errcode"
)

// List GET /api/v1/meetings?search= — 本人會議列表（新→舊）。
func (h *Handler) List(c *gin.Context) {
	userID, ok := domainuser.IDFrom(c.Request.Context())
	if !ok {
		response.Fail(c, apperr.New(errcode.Unauthorized))
		return
	}

	query := strings.TrimSpace(c.Query("search"))
	// 上界保護：截斷過長查詢字串，避免超長輸入打進 ILIKE / 向量嵌入（成本與濫用防護）。
	// 以 rune 為單位截斷，避免切斷多位元組（中文）字元。
	if r := []rune(query); len(r) > maxSearchLen {
		query = string(r[:maxSearchLen])
	}
	// search 非空且已注入 SearchUC → 走 hybrid（字面 + 語意）；否則維持純字面列表
	if query == "" || h.uc.Search == nil {
		list, err := h.uc.List.Execute(c.Request.Context(), userID, query)
		if err != nil {
			response.Fail(c, err)
			return
		}
		response.OK(c, gin.H{"meetings": toMeetingListResponses(list)})
		return
	}

	meetings, hits, err := h.uc.Search.Execute(c.Request.Context(), userID, query)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, gin.H{"meetings": toSearchResponses(meetings, hits)})
}

// Get GET /api/v1/meetings/:id — 會議詳情（含 transcript）。
func (h *Handler) Get(c *gin.Context) {
	userID, ok := domainuser.IDFrom(c.Request.Context())
	if !ok {
		response.Fail(c, apperr.New(errcode.Unauthorized))
		return
	}
	meetingID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.Fail(c, apperr.Wrap(err, errcode.Param, "id"))
		return
	}

	m, err := h.uc.Get.Execute(c.Request.Context(), userID, meetingID)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, gin.H{"meeting": toMeetingDetailResponse(m)})
}

// ListArtifacts GET /api/v1/meetings/:id/artifacts — 取回生成文件。
func (h *Handler) ListArtifacts(c *gin.Context) {
	userID, ok := domainuser.IDFrom(c.Request.Context())
	if !ok {
		response.Fail(c, apperr.New(errcode.Unauthorized))
		return
	}
	meetingID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.Fail(c, apperr.Wrap(err, errcode.Param, "id"))
		return
	}

	list, err := h.uc.ListArtifacts.Execute(c.Request.Context(), userID, meetingID)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, gin.H{"artifacts": toArtifactResponses(list)})
}

// AudioURL GET /api/v1/meetings/:id/audio-url — 取得本人會議音檔的限時播放 URL。
func (h *Handler) AudioURL(c *gin.Context) {
	userID, ok := domainuser.IDFrom(c.Request.Context())
	if !ok {
		response.Fail(c, apperr.New(errcode.Unauthorized))
		return
	}
	meetingID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.Fail(c, apperr.Wrap(err, errcode.Param, "id"))
		return
	}
	url, err := h.uc.AudioURL.Execute(c.Request.Context(), userID, meetingID)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, gin.H{"url": url})
}
