package errcode

import (
	"net/http"
	"testing"
)

func TestHTTPStatus(t *testing.T) {
	tests := []struct {
		name string
		code ErrCode
		want int
	}{
		{"success maps to 200", Success, http.StatusOK},
		{"param maps to 400", Param, http.StatusBadRequest},
		{"unauthorized maps to 401", Unauthorized, http.StatusUnauthorized},
		{"forbidden maps to 403", Forbidden, http.StatusForbidden},
		{"not found maps to 404", NotFound, http.StatusNotFound},
		{"conflict maps to 409", Conflict, http.StatusConflict},
		{"too many requests maps to 429", TooManyRequests, http.StatusTooManyRequests},
		{"correction limit maps to 409", CorrectionLimitReached, http.StatusConflict},
		{"correction in progress maps to 409", CorrectionInProgress, http.StatusConflict},
		{"correction not allowed maps to 409", CorrectionNotAllowed, http.StatusConflict},
		{"nothing to revert maps to 409", NothingToRevert, http.StatusConflict},
		{"live not active maps to 409", LiveNotActive, http.StatusConflict},
		{"live quota exceeded maps to 429", LiveQuotaExceeded, http.StatusTooManyRequests},
		{"context too large maps to 413", ContextTooLarge, http.StatusRequestEntityTooLarge},
		{"request too large maps to 413", RequestTooLarge, http.StatusRequestEntityTooLarge},
		{"context unsupported type maps to 415", ContextUnsupportedType, http.StatusUnsupportedMediaType},
		{"context extract failed maps to 422", ContextExtractFailed, http.StatusUnprocessableEntity},
		{"internal maps to 500", Internal, http.StatusInternalServerError},
		{"unknown code maps to 500", ErrCode(99999), http.StatusInternalServerError},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := HTTPStatus(tt.code); got != tt.want {
				t.Errorf("HTTPStatus(%d) = %d, want %d", tt.code, got, tt.want)
			}
		})
	}
}

func TestFormatMsg(t *testing.T) {
	t.Run("known code returns message", func(t *testing.T) {
		if got := FormatMsg(NotFound); got == "" {
			t.Error("FormatMsg(NotFound) returned empty string")
		}
	})

	t.Run("params are interpolated", func(t *testing.T) {
		got := FormatMsg(Param, "title")
		if got == "" {
			t.Fatal("FormatMsg(Param, ...) returned empty string")
		}
		want := "invalid parameter: title"
		if got != want {
			t.Errorf("FormatMsg(Param, %q) = %q, want %q", "title", got, want)
		}
	})

	t.Run("unknown code falls back to internal message", func(t *testing.T) {
		if got := FormatMsg(ErrCode(99999)); got != FormatMsg(Internal) {
			t.Errorf("FormatMsg(unknown) = %q, want internal fallback %q", got, FormatMsg(Internal))
		}
	})
}
