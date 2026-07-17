// Package config 提供 env-based 設定載入。
// 優先序：OS 環境變數 > .env.{APP_ENV} 檔案 > 預設值（無 YAML）。
package config

import (
	"bufio"
	"os"
	"strings"
)

type Config struct {
	Server ServerConfig
	Log    LogConfig
	DB     DBConfig
	Redis  RedisConfig
	Auth   AuthConfig
	GCS    GCSConfig
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
}

type LogConfig struct {
	Level string // debug / info / warn / error
}

type DBConfig struct {
	URL string
}

type RedisConfig struct {
	Addr     string
	Password string
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
			Env:  appEnv,
			Port: lookup("HTTP_PORT", "8080"),
		},
		Log: LogConfig{
			Level: lookup("LOG_LEVEL", "info"),
		},
		DB: DBConfig{
			URL: lookup("DB_URL", ""),
		},
		Redis: RedisConfig{
			Addr:     lookup("REDIS_ADDR", ""),
			Password: lookup("REDIS_PASSWORD", ""),
		},
		Auth: AuthConfig{
			FirebaseProjectID: lookup("FIREBASE_PROJECT_ID", ""),
			AllowedEmails:     splitCSV(lookup("ALLOWED_EMAILS", "")),
		},
		GCS: GCSConfig{
			Bucket:      lookup("GCS_BUCKET", ""),
			SignerEmail: lookup("GCS_SIGNER_EMAIL", ""),
		},
	}, nil
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
