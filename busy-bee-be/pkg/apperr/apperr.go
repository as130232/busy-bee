// Package apperr 提供帶業務錯誤碼的結構化錯誤（Code + Params + Cause）。
// 原始 cause 只進 log（Error()），不得出現在用戶端訊息（ClientMsg()）。
package apperr

import (
	"fmt"

	"github.com/as130232/busy-bee/busy-bee-be/pkg/consts/errcode"
)

type Error struct {
	Code   errcode.ErrCode
	Params []any
	Cause  error
}

// New 建立業務錯誤（無底層 cause）。
func New(code errcode.ErrCode, params ...any) *Error {
	return &Error{Code: code, Params: params}
}

// Wrap 包裝外部錯誤（DB / 第三方 API），保留 cause 供 log 與 errors.Is 判斷。
func Wrap(cause error, code errcode.ErrCode, params ...any) *Error {
	return &Error{Code: code, Params: params, Cause: cause}
}

// Error 面向 log 的完整訊息，含 cause。
func (e *Error) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("errcode=%d msg=%q cause=%v", e.Code, e.ClientMsg(), e.Cause)
	}
	return fmt.Sprintf("errcode=%d msg=%q", e.Code, e.ClientMsg())
}

// ClientMsg 面向用戶端的訊息，不含 cause。
func (e *Error) ClientMsg() string {
	return errcode.FormatMsg(e.Code, e.Params...)
}

func (e *Error) Unwrap() error {
	return e.Cause
}
