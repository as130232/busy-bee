package ctxutil

import (
	"context"
	"testing"
)

func TestRequestID_RoundTrip(t *testing.T) {
	ctx := WithRequestID(context.Background(), "req-123")

	if got := RequestID(ctx); got != "req-123" {
		t.Errorf("RequestID() = %q, want %q", got, "req-123")
	}
}

func TestRequestID_MissingReturnsEmpty(t *testing.T) {
	if got := RequestID(context.Background()); got != "" {
		t.Errorf("RequestID() = %q, want empty string", got)
	}
}
