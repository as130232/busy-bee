package stt

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

// CompressToMP3 以 ffmpeg 將音訊壓成 16kbps mono mp3（1 小時語音約 7MB）。
// 回傳的 ReadCloser 由 caller 負責 Close（Close 時清除暫存檔）。
func CompressToMP3(ctx context.Context, audio interface{ Read([]byte) (int, error) }) (*tempFileReader, int64, error) {
	dir, err := os.MkdirTemp("", "bb-stt-*")
	if err != nil {
		return nil, 0, fmt.Errorf("compress mkdtemp: %w", err)
	}
	cleanup := func() { os.RemoveAll(dir) }

	inPath := filepath.Join(dir, "in")
	outPath := filepath.Join(dir, "out.mp3")

	inFile, err := os.Create(inPath)
	if err != nil {
		cleanup()
		return nil, 0, fmt.Errorf("compress create temp: %w", err)
	}
	if _, err := inFile.ReadFrom(audio); err != nil {
		inFile.Close()
		cleanup()
		return nil, 0, fmt.Errorf("compress write temp: %w", err)
	}
	inFile.Close()

	cmd := exec.CommandContext(ctx, "ffmpeg", "-i", inPath, "-ac", "1", "-b:a", "16k", "-y", outPath)
	if out, err := cmd.CombinedOutput(); err != nil {
		cleanup()
		return nil, 0, fmt.Errorf("compress ffmpeg: %w: %s", err, tail(out, 300))
	}

	f, err := os.Open(outPath)
	if err != nil {
		cleanup()
		return nil, 0, fmt.Errorf("compress open output: %w", err)
	}
	info, err := f.Stat()
	if err != nil {
		f.Close()
		cleanup()
		return nil, 0, fmt.Errorf("compress stat output: %w", err)
	}
	return &tempFileReader{File: f, cleanup: cleanup}, info.Size(), nil
}

type tempFileReader struct {
	*os.File
	cleanup func()
}

func (t *tempFileReader) Close() error {
	err := t.File.Close()
	t.cleanup()
	return err
}

func tail(b []byte, n int) string {
	if len(b) <= n {
		return string(b)
	}
	return string(b[len(b)-n:])
}
