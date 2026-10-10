//go:build integration

package stt

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// genTone 以 ffmpeg 產生 seconds 秒的正弦波 wav。
func genTone(t *testing.T, seconds int) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "tone.wav")
	cmd := exec.Command("ffmpeg", "-f", "lavfi", "-i", "sine=duration="+strconv.Itoa(seconds), "-y", p)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("gen tone: %v %s", err, out)
	}
	return p
}

func probeSeconds(t *testing.T, p string) float64 {
	t.Helper()
	out, err := exec.Command("ffprobe", "-v", "error", "-show_entries", "format=duration", "-of", "csv=p=0", p).Output()
	if err != nil {
		t.Fatalf("ffprobe: %v", err)
	}
	d, err := strconv.ParseFloat(strings.TrimSpace(string(out)), 64)
	if err != nil {
		t.Fatalf("parse duration %q: %v", out, err)
	}
	return d
}

func TestFFmpegClipper_CutsRange(t *testing.T) {
	requireFFmpeg(t)
	src := genTone(t, 10)
	clip, cleanup, err := FFmpegClipper{}.CutClip(context.Background(), src, 2000, 5000)
	if err != nil {
		t.Fatalf("CutClip: %v", err)
	}
	defer cleanup()
	if d := probeSeconds(t, clip); d < 2.8 || d > 3.2 {
		t.Errorf("clip 長度 = %.2fs，want ~3s", d)
	}
	cleanup()
	if _, err := os.Stat(clip); !os.IsNotExist(err) {
		t.Error("cleanup 後暫存檔應刪除")
	}
}

func TestCompressToTempFile(t *testing.T) {
	requireFFmpeg(t)
	f, err := os.Open(genTone(t, 3))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	p, cleanup, err := CompressToTempFile(context.Background(), f)
	if err != nil {
		t.Fatalf("CompressToTempFile: %v", err)
	}
	if d := probeSeconds(t, p); d < 2.8 || d > 3.2 {
		t.Errorf("長度 = %.2fs", d)
	}
	cleanup()
	if _, err := os.Stat(p); !os.IsNotExist(err) {
		t.Error("cleanup 後暫存檔應刪除")
	}
}
