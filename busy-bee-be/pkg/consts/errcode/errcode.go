// Package errcode 定義全站業務錯誤碼與其 HTTP status、用戶端訊息映射。
package errcode

import "net/http"

// ErrCode 業務錯誤碼。格式：HTTP status 三碼 + 兩碼流水號。
type ErrCode int

const (
	Success         ErrCode = 0
	Param           ErrCode = 40001
	Unauthorized    ErrCode = 40101
	Forbidden       ErrCode = 40301
	NotFound        ErrCode = 40401
	Conflict        ErrCode = 40901
	TooManyRequests ErrCode = 42901
	Internal        ErrCode = 50001
)

var httpStatus = map[ErrCode]int{
	Success:         http.StatusOK,
	Param:           http.StatusBadRequest,
	Unauthorized:    http.StatusUnauthorized,
	Forbidden:       http.StatusForbidden,
	NotFound:        http.StatusNotFound,
	Conflict:        http.StatusConflict,
	TooManyRequests: http.StatusTooManyRequests,
	Internal:        http.StatusInternalServerError,
}

// HTTPStatus 回傳錯誤碼對應的 HTTP status；未知錯誤碼一律視為 500。
func HTTPStatus(c ErrCode) int {
	if s, ok := httpStatus[c]; ok {
		return s
	}
	return http.StatusInternalServerError
}
