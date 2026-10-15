package stt

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestDeepgramGrant_Issue(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.Header.Get("Authorization") != "Token key" {
			t.Errorf("method %s auth %q", r.Method, r.Header.Get("Authorization"))
		}
		var body struct {
			TTL float64 `json:"ttl_seconds"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		if body.TTL != grantTTLSeconds {
			t.Errorf("ttl = %v", body.TTL)
		}
		_, _ = w.Write([]byte(`{"access_token":"jwt-1","expires_in":30}`))
	}))
	defer srv.Close()

	tok, err := NewDeepgramGrant("key", WithGrantURL(srv.URL)).Issue(context.Background())
	if err != nil || tok.AccessToken != "jwt-1" || tok.ExpiresIn != 30 {
		t.Fatalf("tok = %+v err %v", tok, err)
	}
}

func TestDeepgramGrant_ErrorDoesNotLeakBody(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte(`{"err_code":"FORBIDDEN","err_msg":"Insufficient permissions."}`))
	}))
	defer srv.Close()

	_, err := NewDeepgramGrant("secret-key", WithGrantURL(srv.URL)).Issue(context.Background())
	if err == nil || !strings.Contains(err.Error(), "403") || strings.Contains(err.Error(), "secret-key") {
		t.Fatalf("err = %v", err)
	}
}

func TestDeepgramGrant_EmptyToken(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{}`))
	}))
	defer srv.Close()
	if _, err := NewDeepgramGrant("k", WithGrantURL(srv.URL)).Issue(context.Background()); err == nil {
		t.Fatal("empty access_token must error")
	}
}
