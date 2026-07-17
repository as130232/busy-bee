package user

import (
	"context"
	"testing"
)

func TestIdentityContext_RoundTrip(t *testing.T) {
	id := Identity{UID: "fb-uid-1", Email: "a@example.com", Name: "A", Picture: "http://p"}
	ctx := WithIdentity(context.Background(), id)

	got, ok := IdentityFrom(ctx)
	if !ok {
		t.Fatal("IdentityFrom() ok = false, want true")
	}
	if got != id {
		t.Errorf("IdentityFrom() = %+v, want %+v", got, id)
	}
}

func TestIdentityFrom_MissingReturnsFalse(t *testing.T) {
	if _, ok := IdentityFrom(context.Background()); ok {
		t.Error("IdentityFrom(empty ctx) ok = true, want false")
	}
}
