package errcode

import "fmt"

// msg 用戶端訊息模板；含 %s / %v 佔位時由 FormatMsg 以 params 填入。
var msg = map[ErrCode]string{
	Success:      "success",
	Param:        "invalid parameter: %v",
	Unauthorized: "unauthorized",
	Forbidden:    "forbidden",
	NotFound:     "resource not found",
	Internal:     "internal server error",
}

// FormatMsg 回傳錯誤碼的用戶端訊息；未知錯誤碼 fallback 到 Internal。
func FormatMsg(c ErrCode, params ...any) string {
	tmpl, ok := msg[c]
	if !ok {
		return msg[Internal]
	}
	if len(params) == 0 {
		return tmpl
	}
	return fmt.Sprintf(tmpl, params...)
}
