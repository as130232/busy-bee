// Package config 提供 env-based 設定載入。
// 優先序：OS 環境變數 > .env.{APP_ENV} 檔案 > 預設值（無 YAML）。
package config

import (
	"bufio"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	Server   ServerConfig
	Log      LogConfig
	DB       DBConfig
	Auth     AuthConfig
	GCS      GCSConfig
	Groq     GroqConfig
	Gemini   GeminiConfig
	Deepgram DeepgramConfig
	Live     LiveConfig
	Push     PushConfig
	Worker   WorkerConfig
}

type WorkerConfig struct {
	// TaskTimeout 單一會議處理任務的執行上界，防止外部 API 卡死佔用 worker 名額。
	TaskTimeout time.Duration
}

type GroqConfig struct {
	APIKey string
}

type DeepgramConfig struct {
	APIKey   string
	Model    string
	Language string
	// Keywords 術語加權（如專有名詞、英文縮寫），提升辨識準確度；格式可帶權重 "term:2"。
	Keywords []string
}

// LiveConfig 即時會議（F-LIVE）設定。
type LiveConfig struct {
	// STTModel 瀏覽器直連 Deepgram 串流用的模型（與會後整批辨識的 DEEPGRAM_MODEL 分開）。
	STTModel string
	// MonthlyMinutesLimit 每位使用者每月即時辨識分鐘上限（超過只停即時辨識，錄音照常）。
	MonthlyMinutesLimit int
	// MaxContextChars 單場會前資料合計字數上限。
	MaxContextChars int
	// MaxContextFileBytes 單一會前資料檔案大小上限。
	MaxContextFileBytes int
}

type GeminiConfig struct {
	APIKey string
	Model  string
	// FallbackModel 主模型不存在（404，例如 Preview 下架）時自動改用的模型；空字串停用備援。
	FallbackModel string
}

type PushConfig struct {
	VAPIDPublicKey  string
	VAPIDPrivateKey string
	SubscriberEmail string
	// SweepSecret 保護內部提醒掃描端點（Cloud Scheduler 帶此密鑰呼叫）；空 = 端點停用。
	SweepSecret string
}

type GCSConfig struct {
	Bucket      string
	SignerEmail string
}

type AuthConfig struct {
	FirebaseProjectID string
	AllowedEmails     []string // email 白名單；空 = fail-closed 拒絕所有人
}

type ServerConfig struct {
	Env  string // local / qa / prod
	Port string
	// AllowedOrigins WebSocket 允許的來源白名單（coder/websocket OriginPatterns 格式，如 "app.example.com"、"*.example.com"）；
	// 空 = 不檢查來源（相容既有行為）。
	AllowedOrigins []string
}

type LogConfig struct {
	Level string // debug / info / warn / error
}

type DBConfig struct {
	URL string
}

// Load 載入設定。.env.{APP_ENV} 不存在時靜默略過（生產環境只用 OS env）。
func Load() (*Config, error) {
	appEnv := getEnv("APP_ENV", "local")

	fileVars, err := loadDotEnv(".env." + appEnv)
	if err != nil {
		return nil, err
	}
	lookup := func(key, def string) string {
		if v, ok := os.LookupEnv(key); ok {
			return v
		}
		if v, ok := fileVars[key]; ok {
			return v
		}
		return def
	}

	return &Config{
		Server: ServerConfig{
			Env:            appEnv,
			Port:           lookup("HTTP_PORT", "8080"),
			AllowedOrigins: splitCSV(lookup("FRONTEND_ORIGINS", "")),
		},
		Log: LogConfig{
			Level: lookup("LOG_LEVEL", "info"),
		},
		DB: DBConfig{
			URL: lookup("DB_URL", ""),
		},
		Auth: AuthConfig{
			FirebaseProjectID: lookup("FIREBASE_PROJECT_ID", ""),
			AllowedEmails:     splitCSV(lookup("ALLOWED_EMAILS", "")),
		},
		GCS: GCSConfig{
			Bucket:      lookup("GCS_BUCKET", ""),
			SignerEmail: lookup("GCS_SIGNER_EMAIL", ""),
		},
		Groq: GroqConfig{
			APIKey: lookup("GROQ_API_KEY", ""),
		},
		Gemini: GeminiConfig{
			APIKey: lookup("GEMINI_API_KEY", ""),
			// 固定版本：gemini-3-flash-preview 實測長會議細節最完整（3.x Lite 會漏決議）；見 ADR-015/017
			Model:         lookup("GEMINI_MODEL", "gemini-3-flash-preview"),
			FallbackModel: lookup("GEMINI_FALLBACK_MODEL", "gemini-3.5-flash"),
		},
		Deepgram: DeepgramConfig{
			APIKey: lookup("DEEPGRAM_API_KEY", ""),
			// nova-2 + zh-TW 才轉得出繁體中文；nova-3 的 multi 對中文回傳空結果。
			Model:    lookup("DEEPGRAM_MODEL", "nova-2"),
			Language: lookup("DEEPGRAM_LANGUAGE", "zh-TW"),
			Keywords: splitCSV(lookup("DEEPGRAM_KEYWORDS", "")),
		},
		Live: LiveConfig{
			STTModel:            lookup("DEEPGRAM_LIVE_MODEL", "nova-3"),
			MonthlyMinutesLimit: parseInt(lookup("LIVE_MONTHLY_MINUTES_LIMIT", ""), 3000),
			MaxContextChars:     parseInt(lookup("LIVE_CONTEXT_MAX_CHARS", ""), 100000),
			MaxContextFileBytes: parseInt(lookup("LIVE_CONTEXT_MAX_FILE_BYTES", ""), 10<<20),
		},
		Push: PushConfig{
			VAPIDPublicKey:  lookup("VAPID_PUBLIC_KEY", ""),
			VAPIDPrivateKey: lookup("VAPID_PRIVATE_KEY", ""),
			SubscriberEmail: lookup("VAPID_SUBSCRIBER_EMAIL", ""),
			SweepSecret:     lookup("REMINDER_SWEEP_SECRET", ""),
		},
		Worker: WorkerConfig{
			TaskTimeout: parseDuration(lookup("WORKER_TASK_TIMEOUT", "15m"), 15*time.Minute),
		},
	}, nil
}

// parseInt 解析正整數，空值或無效時回退 def。
func parseInt(s string, def int) int {
	n, err := strconv.Atoi(strings.TrimSpace(s))
	if err != nil || n <= 0 {
		return def
	}
	return n
}

// parseDuration 解析 Go duration 字串（如 "15m"），無效時回退 def。
func parseDuration(s string, def time.Duration) time.Duration {
	d, err := time.ParseDuration(s)
	if err != nil {
		return def
	}
	return d
}

func splitCSV(s string) []string {
	if s == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if v := strings.TrimSpace(p); v != "" {
			out = append(out, v)
		}
	}
	return out
}

func getEnv(key, def string) string {
	if v, ok := os.LookupEnv(key); ok {
		return v
	}
	return def
}

// loadDotEnv 解析 KEY=VALUE 格式的 .env 檔；支援註解（#）與空行。
func loadDotEnv(path string) (map[string]string, error) {
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	defer f.Close()

	vars := make(map[string]string)
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		vars[strings.TrimSpace(key)] = strings.TrimSpace(value)
	}
	return vars, sc.Err()
}
