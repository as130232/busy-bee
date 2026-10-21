package meeting

import "errors"

var (
	// ErrNotFound 會議不存在或非本人所有（owner-only 可見性，兩者不區分）。
	ErrNotFound = errors.New("meeting not found")
	// ErrStatusConflict 狀態轉移時 from-status 不符（併發或重複觸發）。
	ErrStatusConflict = errors.New("meeting status conflict")
	// ErrNoSpeech 音檔辨識不出任何文字（多為整段靜音）；重試不會改變結果。
	ErrNoSpeech = errors.New("no speech in audio")
)

// nonRetryable 標記重試也不會成功的錯誤。
type nonRetryable struct{ err error }

func (e nonRetryable) Error() string { return e.err.Error() }
func (e nonRetryable) Unwrap() error { return e.err }

// NonRetryable 包裝重試也不會成功的錯誤；佇列遇到時直接標失敗，不再 backoff 重試。
func NonRetryable(err error) error {
	if err == nil {
		return nil
	}
	return nonRetryable{err: err}
}

// IsNonRetryable 判斷錯誤鏈中是否有 NonRetryable 標記。
func IsNonRetryable(err error) bool {
	var n nonRetryable
	return errors.As(err, &n)
}
