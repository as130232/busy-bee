// Package errcode 定義全站業務錯誤碼與其 HTTP status、用戶端訊息映射。
package errcode

import "net/http"

// ErrCode 業務錯誤碼。格式：HTTP status 三碼 + 兩碼流水號。
type ErrCode int

const (
	Success      ErrCode = 0
	Param        ErrCode = 40001
	Unauthorized ErrCode = 40101
	Forbidden    ErrCode = 40301
	NotFound     ErrCode = 40401
	Conflict     ErrCode = 40901
	// AI 錄音重新校正（F-CORRECT）
	CorrectionLimitReached ErrCode = 40902 // 已達每場校正上限
	CorrectionInProgress   ErrCode = 40903 // 已有進行中的校正
	CorrectionNotAllowed   ErrCode = 40904 // 紀錄未完成或無音檔/逐字稿
	NothingToRevert        ErrCode = 40905 // 最新一筆不是可還原的完成校正
	// 即時會議（F-LIVE）
	LiveNotActive          ErrCode = 40906 // 紀錄不在即時會議中
	ContextTooLarge        ErrCode = 41301 // 會前資料超過大小上限
	ContextUnsupportedType ErrCode = 41501 // 會前資料檔案格式不支援
	ContextExtractFailed   ErrCode = 42201 // 會前資料轉文字失敗
	TooManyRequests        ErrCode = 42901
	LiveQuotaExceeded      ErrCode = 42902 // 本月即時辨識額度已用完
	Internal               ErrCode = 50001
)

var httpStatus = map[ErrCode]int{
	Success:                http.StatusOK,
	Param:                  http.StatusBadRequest,
	Unauthorized:           http.StatusUnauthorized,
	Forbidden:              http.StatusForbidden,
	NotFound:               http.StatusNotFound,
	Conflict:               http.StatusConflict,
	CorrectionLimitReached: http.StatusConflict,
	CorrectionInProgress:   http.StatusConflict,
	CorrectionNotAllowed:   http.StatusConflict,
	NothingToRevert:        http.StatusConflict,
	LiveNotActive:          http.StatusConflict,
	ContextTooLarge:        http.StatusRequestEntityTooLarge,
	ContextUnsupportedType: http.StatusUnsupportedMediaType,
	ContextExtractFailed:   http.StatusUnprocessableEntity,
	TooManyRequests:        http.StatusTooManyRequests,
	LiveQuotaExceeded:      http.StatusTooManyRequests,
	Internal:               http.StatusInternalServerError,
}

// HTTPStatus 回傳錯誤碼對應的 HTTP status；未知錯誤碼一律視為 500。
func HTTPStatus(c ErrCode) int {
	if s, ok := httpStatus[c]; ok {
		return s
	}
	return http.StatusInternalServerError
}
