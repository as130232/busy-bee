package apperr

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/as130232/busy-bee/busy-bee-be/pkg/consts/errcode"
)

func TestNew(t *testing.T) {
	err := New(errcode.NotFound)

	if err.Code != errcode.NotFound {
		t.Errorf("Code = %d, want %d", err.Code, errcode.NotFound)
	}
	if err.Cause != nil {
		t.Errorf("Cause = %v, want nil", err.Cause)
	}
}

func TestWrap(t *testing.T) {
	cause := errors.New("connection refused")
	err := Wrap(cause, errcode.Internal)

	t.Run("preserves cause via Unwrap", func(t *testing.T) {
		if !errors.Is(err, cause) {
			t.Error("errors.Is(err, cause) = false, want true")
		}
	})

	t.Run("cause message not in Error() output", func(t *testing.T) {
		// 外部錯誤原始 cause 不得外洩到面向用戶端的訊息
		if strings.Contains(err.ClientMsg(), "connection refused") {
			t.Errorf("ClientMsg() = %q leaks internal cause", err.ClientMsg())
		}
	})
}

func TestErrorsAs_ExtractsFromWrappedChain(t *testing.T) {
	inner := New(errcode.NotFound)
	wrapped := fmt.Errorf("application/meeting get: %w", inner)

	var ae *Error
	if !errors.As(wrapped, &ae) {
		t.Fatal("errors.As failed to extract *Error from wrapped chain")
	}
	if ae.Code != errcode.NotFound {
		t.Errorf("extracted Code = %d, want %d", ae.Code, errcode.NotFound)
	}
}

func TestError_MessageIncludesCodeAndCause(t *testing.T) {
	cause := errors.New("timeout")
	err := Wrap(cause, errcode.Internal)

	msg := err.Error()
	if !strings.Contains(msg, "timeout") {
		t.Errorf("Error() = %q, want cause included for logs", msg)
	}
}

func TestNew_WithParams(t *testing.T) {
	err := New(errcode.Param, "title")
	if got := err.ClientMsg(); got != "invalid parameter: title" {
		t.Errorf("ClientMsg() = %q, want params interpolated", got)
	}
}
