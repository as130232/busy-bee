package errcode

import "fmt"

// msg 用戶端訊息模板；含 %s / %v 佔位時由 FormatMsg 以 params 填入。
var msg = map[ErrCode]string{
	Success:                "success",
	Param:                  "invalid parameter: %v",
	Unauthorized:           "unauthorized",
	Forbidden:              "forbidden",
	NotFound:               "resource not found",
	Conflict:               "resource state conflict",
	CorrectionLimitReached: "correction limit reached",
	CorrectionInProgress:   "correction already in progress",
	CorrectionNotAllowed:   "meeting cannot be corrected",
	NothingToRevert:        "nothing to revert",
	LiveNotActive:          "meeting is not live",
	ContextTooLarge:        "meeting context too large",
	ContextUnsupportedType: "unsupported meeting context file type",
	ContextExtractFailed:   "failed to read meeting context file",
	TooManyRequests:        "too many requests, please slow down",
	LiveQuotaExceeded:      "monthly live transcription quota exceeded",
	Internal:               "internal server error",
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
