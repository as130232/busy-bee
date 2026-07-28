// Package fetch 以 HTTP（直接音檔/Podcast）與 yt-dlp（YouTube/其他站台）實作
// domain/meeting.AudioFetcher：把外部連結的音訊抓成本地暫存檔供上傳 GCS。
package fetch

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"

	domainmeeting "github.com/as130232/busy-bee/busy-bee-be/domain/meeting"
)

const (
	// defaultMaxDurationSec 抓取來源的最長時長（90 分鐘）；超過拒絕，保護 STT 費用。
	defaultMaxDurationSec = 90 * 60
	// defaultMaxBytes 抓取音檔大小上限（200MB），與前端上傳一致。
	defaultMaxBytes = 200 * 1024 * 1024
)

// audioExts 視為「直接音檔」的副檔名（走 HTTP，不動用 yt-dlp）。
var audioExts = map[string]bool{
	".mp3": true, ".m4a": true, ".wav": true, ".aac": true,
	".ogg": true, ".opus": true, ".flac": true, ".mp4": true,
}

// Fetcher 依連結型態路由：直接音檔/Podcast 走 HTTP，其餘走 yt-dlp。
type Fetcher struct {
	http           *http.Client
	ytdlpPath      string
	tempDir        string
	maxDurationSec int
	maxBytes       int64
}

var _ domainmeeting.AudioFetcher = (*Fetcher)(nil)

func New() *Fetcher {
	return &Fetcher{
		http:           &http.Client{Timeout: 10 * time.Minute},
		ytdlpPath:      "yt-dlp",
		tempDir:        os.TempDir(),
		maxDurationSec: defaultMaxDurationSec,
		maxBytes:       defaultMaxBytes,
	}
}

// Fetch 驗證 URL 後依型態抓取音訊。回傳的 Reader 於 Close 時清理暫存檔。
func (f *Fetcher) Fetch(ctx context.Context, rawURL string) (domainmeeting.FetchedAudio, error) {
	u, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return domainmeeting.FetchedAudio{}, fmt.Errorf("fetch: 無效的連結")
	}
	if f.looksDirectAudio(ctx, u) {
		return f.fetchHTTP(ctx, u)
	}
	return f.fetchYtdlp(ctx, rawURL)
}

// looksDirectAudio 副檔名為音檔即判定；否則 HEAD 檢查 content-type 是否 audio/*。
func (f *Fetcher) looksDirectAudio(ctx context.Context, u *url.URL) bool {
	if audioExts[strings.ToLower(path.Ext(u.Path))] {
		return true
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodHead, u.String(), nil)
	if err != nil {
		return false
	}
	resp, err := f.http.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	return strings.HasPrefix(resp.Header.Get("Content-Type"), "audio/")
}

// fetchHTTP 直接下載音檔到暫存檔，超過大小上限即中止。
func (f *Fetcher) fetchHTTP(ctx context.Context, u *url.URL) (domainmeeting.FetchedAudio, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return domainmeeting.FetchedAudio{}, fmt.Errorf("fetch http request: %w", err)
	}
	resp, err := f.http.Do(req)
	if err != nil {
		return domainmeeting.FetchedAudio{}, fmt.Errorf("fetch http get: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return domainmeeting.FetchedAudio{}, fmt.Errorf("fetch http status %d", resp.StatusCode)
	}

	tf, err := os.CreateTemp(f.tempDir, "bb-fetch-*")
	if err != nil {
		return domainmeeting.FetchedAudio{}, fmt.Errorf("fetch temp file: %w", err)
	}
	// 多讀 1 byte 判斷是否超過上限。
	n, err := io.Copy(tf, io.LimitReader(resp.Body, f.maxBytes+1))
	if err != nil {
		cleanup(tf)
		return domainmeeting.FetchedAudio{}, fmt.Errorf("fetch http copy: %w", err)
	}
	if n > f.maxBytes {
		cleanup(tf)
		return domainmeeting.FetchedAudio{}, fmt.Errorf("fetch: 音檔超過 %d MB 上限", f.maxBytes/1024/1024)
	}
	if _, err := tf.Seek(0, io.SeekStart); err != nil {
		cleanup(tf)
		return domainmeeting.FetchedAudio{}, fmt.Errorf("fetch seek: %w", err)
	}
	return domainmeeting.FetchedAudio{
		Reader:      &tempReader{File: tf},
		Size:        n,
		ContentType: resp.Header.Get("Content-Type"),
		Title:       titleFromURL(u),
	}, nil
}

// fetchYtdlp 先取 metadata（擋超長、免白下載），再抽音訊成 m4a 暫存檔。
func (f *Fetcher) fetchYtdlp(ctx context.Context, rawURL string) (domainmeeting.FetchedAudio, error) {
	metaOut, err := exec.CommandContext(ctx, f.ytdlpPath,
		"--no-playlist", "--skip-download",
		"--print", "%(title)s", "--print", "%(duration)s",
		rawURL,
	).Output()
	if err != nil {
		if errors.Is(err, exec.ErrNotFound) {
			return domainmeeting.FetchedAudio{}, fmt.Errorf("fetch: 伺服器未安裝 yt-dlp，無法匯入此連結")
		}
		return domainmeeting.FetchedAudio{}, fmt.Errorf("fetch yt-dlp metadata: %w", err)
	}
	lines := strings.Split(strings.TrimSpace(string(metaOut)), "\n")
	title := ""
	if len(lines) > 0 {
		title = strings.TrimSpace(lines[0])
	}
	duration := 0
	if len(lines) > 1 {
		duration, _ = strconv.Atoi(strings.TrimSpace(lines[1]))
	}
	if duration > f.maxDurationSec {
		return domainmeeting.FetchedAudio{}, fmt.Errorf("fetch: 內容長度 %d 分鐘超過 %d 分鐘上限", duration/60, f.maxDurationSec/60)
	}

	base := filepath.Join(f.tempDir, "bb-yt-"+uuid.NewString())
	audioPath := base + ".m4a"
	cmd := exec.CommandContext(ctx, f.ytdlpPath,
		"-x", "--audio-format", "m4a", "--no-playlist",
		"--max-filesize", strconv.FormatInt(f.maxBytes, 10),
		"-o", base+".%(ext)s",
		rawURL,
	)
	if out, err := cmd.CombinedOutput(); err != nil {
		_ = os.Remove(audioPath)
		return domainmeeting.FetchedAudio{}, fmt.Errorf("fetch yt-dlp download: %w (%s)", err, lastLine(out))
	}
	fi, err := os.Stat(audioPath)
	if err != nil {
		return domainmeeting.FetchedAudio{}, fmt.Errorf("fetch yt-dlp output missing: %w", err)
	}
	file, err := os.Open(audioPath)
	if err != nil {
		return domainmeeting.FetchedAudio{}, fmt.Errorf("fetch open output: %w", err)
	}
	return domainmeeting.FetchedAudio{
		Reader:      &tempReader{File: file},
		Size:        fi.Size(),
		ContentType: "audio/mp4",
		Title:       title,
		DurationSec: duration,
	}, nil
}

// titleFromURL 由路徑檔名推標題（去副檔名）；無則給預設。
func titleFromURL(u *url.URL) string {
	name := strings.TrimSuffix(path.Base(u.Path), path.Ext(u.Path))
	if name == "" || name == "/" || name == "." {
		return "匯入的音訊"
	}
	return name
}

func lastLine(b []byte) string {
	lines := strings.Split(strings.TrimSpace(string(b)), "\n")
	return lines[len(lines)-1]
}

func cleanup(f *os.File) {
	name := f.Name()
	_ = f.Close()
	_ = os.Remove(name)
}

// tempReader 是暫存檔的 ReadCloser，Close 時一併刪檔（避免 /tmp 累積）。
type tempReader struct {
	*os.File
}

func (t *tempReader) Close() error {
	name := t.File.Name()
	err := t.File.Close()
	_ = os.Remove(name)
	return err
}
