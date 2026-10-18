// Package tempslot 限制同時寫大型暫存檔的工作數。Cloud Run 的 /tmp 放在記憶體裡、算進容器
// 記憶體上限，多個 200MB 音檔同時落地會讓容器因記憶體不足被終止；用共用名額讓它們排隊。
package tempslot

import (
	"context"
	"io"
	"sync"
)

// Slots 共用名額；nil 代表不限制（測試或本機用）。
type Slots struct {
	ch chan struct{}
}

func New(n int) *Slots {
	return &Slots{ch: make(chan struct{}, n)}
}

// Acquire 取得一個名額，等待期間 ctx 結束則回錯。release 可重複呼叫，只會歸還一次。
func (s *Slots) Acquire(ctx context.Context) (release func(), err error) {
	if s == nil {
		return func() {}, nil
	}
	select {
	case s.ch <- struct{}{}:
	case <-ctx.Done():
		return nil, ctx.Err()
	}
	var once sync.Once
	return func() { once.Do(func() { <-s.ch }) }, nil
}

// GuardTempFile 包住「讀入大檔、產出暫存檔」的函式：執行期間持有名額，回傳後就歸還
// （產出的壓縮檔很小，不必占名額到 cleanup）。
func (s *Slots) GuardTempFile(fn func(ctx context.Context, r io.Reader) (string, func(), error)) func(ctx context.Context, r io.Reader) (string, func(), error) {
	return func(ctx context.Context, r io.Reader) (string, func(), error) {
		release, err := s.Acquire(ctx)
		if err != nil {
			return "", nil, err
		}
		defer release()
		return fn(ctx, r)
	}
}
