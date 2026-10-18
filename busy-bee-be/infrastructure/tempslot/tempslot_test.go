package tempslot

import (
	"context"
	"errors"
	"io"
	"strings"
	"testing"
	"time"
)

func TestAcquire_BlocksUntilReleased(t *testing.T) {
	s := New(1)
	release, err := s.Acquire(context.Background())
	if err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	if _, err := s.Acquire(ctx); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("second Acquire err = %v, want DeadlineExceeded while slot held", err)
	}

	release()
	release() // 重複釋放不可多還名額
	r2, err := s.Acquire(context.Background())
	if err != nil {
		t.Fatalf("Acquire after release: %v", err)
	}
	ctx2, cancel2 := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel2()
	if _, err := s.Acquire(ctx2); err == nil {
		t.Fatal("double release must not create an extra slot")
	}
	r2()
}

func TestNilSlots_NoLimit(t *testing.T) {
	var s *Slots
	release, err := s.Acquire(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	release()
}

// GuardTempFile：名額被占用時不執行包住的函式；執行完就歸還名額。
func TestGuardTempFile(t *testing.T) {
	s := New(1)
	calls := 0
	fn := s.GuardTempFile(func(context.Context, io.Reader) (string, func(), error) {
		calls++
		return "out.mp3", func() {}, nil
	})

	hold, _ := s.Acquire(context.Background())
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
	defer cancel()
	if _, _, err := fn(ctx, strings.NewReader("x")); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("err = %v, want DeadlineExceeded", err)
	}
	if calls != 0 {
		t.Fatal("wrapped func must not run without a slot")
	}
	hold()

	if p, _, err := fn(context.Background(), strings.NewReader("x")); err != nil || p != "out.mp3" {
		t.Fatalf("got %q, %v", p, err)
	}
	ctx2, cancel2 := context.WithTimeout(context.Background(), time.Second)
	defer cancel2()
	r, err := s.Acquire(ctx2)
	if err != nil {
		t.Fatalf("slot not released after wrapped func returned: %v", err)
	}
	r()
}
