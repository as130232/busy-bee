package gcs

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"os"
	"testing"
	"time"

	"cloud.google.com/go/storage"
)

const (
	testBucket = "busy-bee-502710-audio"
	testSigner = "busy-bee-storage@busy-bee-502710.iam.gserviceaccount.com"
)

func testStorage(t *testing.T) *Storage {
	t.Helper()
	if os.Getenv("CI") != "" {
		t.Skip("skip GCS integration test in CI (no ADC)")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	s, err := New(ctx, testBucket, testSigner)
	if err != nil {
		t.Skipf("GCS unavailable (no ADC?): %v", err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

func cleanupObject(t *testing.T, path string) {
	t.Helper()
	t.Cleanup(func() {
		ctx := context.Background()
		c, err := storage.NewClient(ctx)
		if err != nil {
			return
		}
		defer c.Close()
		c.Bucket(testBucket).Object(path).Delete(ctx)
	})
}

func TestSignedUploadURL_PutSucceedsWithinLimit(t *testing.T) {
	s := testStorage(t)
	path := fmt.Sprintf("test/%d.webm", time.Now().UnixNano())
	cleanupObject(t, path)

	target, err := s.SignedUploadURL(context.Background(), path, "audio/webm", 1024)
	if err != nil {
		t.Fatalf("SignedUploadURL() error = %v", err)
	}

	req, _ := http.NewRequest(http.MethodPut, target.URL, bytes.NewReader([]byte("fake-audio")))
	for k, v := range target.Headers {
		req.Header.Set(k, v)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("PUT error = %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("PUT status = %d, want 200", resp.StatusCode)
	}

	exists, err := s.Exists(context.Background(), path)
	if err != nil {
		t.Fatalf("Exists() error = %v", err)
	}
	if !exists {
		t.Error("Exists() = false after successful upload")
	}
}

func TestSignedUploadURL_RejectsOversizedUpload(t *testing.T) {
	s := testStorage(t)
	path := fmt.Sprintf("test/%d-big.webm", time.Now().UnixNano())
	cleanupObject(t, path)

	target, err := s.SignedUploadURL(context.Background(), path, "audio/webm", 10) // 上限 10 bytes
	if err != nil {
		t.Fatalf("SignedUploadURL() error = %v", err)
	}

	big := bytes.Repeat([]byte("x"), 100)
	req, _ := http.NewRequest(http.MethodPut, target.URL, bytes.NewReader(big))
	for k, v := range target.Headers {
		req.Header.Set(k, v)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("PUT error = %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusOK {
		t.Fatal("oversized PUT should be rejected by content-length-range")
	}
}

func TestExists_MissingObjectReturnsFalse(t *testing.T) {
	s := testStorage(t)

	exists, err := s.Exists(context.Background(), "test/never-uploaded.webm")
	if err != nil {
		t.Fatalf("Exists() error = %v", err)
	}
	if exists {
		t.Error("Exists() = true for missing object")
	}
}
