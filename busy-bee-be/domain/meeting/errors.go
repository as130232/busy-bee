package meeting

import "errors"

var (
	// ErrNotFound 會議不存在或非本人所有（owner-only 可見性，兩者不區分）。
	ErrNotFound = errors.New("meeting not found")
	// ErrStatusConflict 狀態轉移時 from-status 不符（併發或重複觸發）。
	ErrStatusConflict = errors.New("meeting status conflict")
)
