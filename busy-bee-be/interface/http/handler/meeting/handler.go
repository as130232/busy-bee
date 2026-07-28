// Package meeting 提供會議相關 HTTP handlers。
// handler 依職責分檔：create.go（建立/上傳）、query.go（查詢）、
// schedule.go（排程）、manage.go（改名/刪除/講者/片段/重試）。
package meeting

import (
	appmeeting "github.com/as130232/busy-bee/busy-bee-be/application/meeting"
	appsearch "github.com/as130232/busy-bee/busy-bee-be/application/search"
)

// HandlerUCs Handler 依賴的 use cases。
type HandlerUCs struct {
	Create         *appmeeting.CreateUC
	Import         *appmeeting.ImportUC
	CompleteUpload *appmeeting.CompleteUploadUC
	ListArtifacts  *appmeeting.ListArtifactsUC
	List           *appmeeting.ListUC
	Get            *appmeeting.GetUC
	AudioURL       *appmeeting.AudioURLUC
	Retry          *appmeeting.RetryUC
	Schedule       *appmeeting.ScheduleUC
	Manage         *appmeeting.ManageUC
	EditSegment    *appmeeting.EditSegmentUC
	Search         *appsearch.SearchUC // 選填；nil 時 List 維持純字面
}

// maxSearchLen 搜尋字串長度上界（rune）；超過即截斷，防止超長輸入打進 ILIKE / 向量嵌入。
const maxSearchLen = 256

type Handler struct {
	uc HandlerUCs
}

func NewHandler(uc HandlerUCs) *Handler {
	return &Handler{uc: uc}
}
