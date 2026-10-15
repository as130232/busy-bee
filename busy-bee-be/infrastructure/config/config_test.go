package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoad_Defaults(t *testing.T) {
	t.Chdir(t.TempDir()) // 無 .env 檔的乾淨目錄

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.Server.Env != "local" {
		t.Errorf("Server.Env = %q, want %q", cfg.Server.Env, "local")
	}
	if cfg.Server.Port != "8080" {
		t.Errorf("Server.Port = %q, want %q", cfg.Server.Port, "8080")
	}
	if cfg.Log.Level != "info" {
		t.Errorf("Log.Level = %q, want %q", cfg.Log.Level, "info")
	}
	// 固定版本而非浮動別名（gemini-flash-latest 可能自動指到更貴/更弱的模型）；備援用 GA 版
	if cfg.Gemini.Model != "gemini-3-flash-preview" || cfg.Gemini.FallbackModel != "gemini-3.5-flash" {
		t.Errorf("Gemini = %q / fallback %q", cfg.Gemini.Model, cfg.Gemini.FallbackModel)
	}
}

func TestLoad_OSEnvOverridesDefault(t *testing.T) {
	t.Chdir(t.TempDir())
	t.Setenv("HTTP_PORT", "9090")
	t.Setenv("LOG_LEVEL", "debug")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.Server.Port != "9090" {
		t.Errorf("Server.Port = %q, want %q", cfg.Server.Port, "9090")
	}
	if cfg.Log.Level != "debug" {
		t.Errorf("Log.Level = %q, want %q", cfg.Log.Level, "debug")
	}
}

func TestLoad_DotEnvFile(t *testing.T) {
	dir := t.TempDir()
	envFile := filepath.Join(dir, ".env.local")
	content := "DB_URL=postgres://file-value\n# comment line\n\nGCS_BUCKET=file-bucket\n"
	if err := os.WriteFile(envFile, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Chdir(dir)

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.DB.URL != "postgres://file-value" {
		t.Errorf("DB.URL = %q, want value from .env.local", cfg.DB.URL)
	}
	if cfg.GCS.Bucket != "file-bucket" {
		t.Errorf("GCS.Bucket = %q, want value from .env.local", cfg.GCS.Bucket)
	}
}

func TestLoad_OSEnvTakesPrecedenceOverDotEnv(t *testing.T) {
	dir := t.TempDir()
	envFile := filepath.Join(dir, ".env.local")
	if err := os.WriteFile(envFile, []byte("DB_URL=postgres://file-value\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Chdir(dir)
	t.Setenv("DB_URL", "postgres://os-env-value")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.DB.URL != "postgres://os-env-value" {
		t.Errorf("DB.URL = %q, want OS env to win over .env file", cfg.DB.URL)
	}
}

func TestLoad_AuthConfig(t *testing.T) {
	t.Chdir(t.TempDir())
	t.Setenv("FIREBASE_PROJECT_ID", "busy-bee-prod")
	t.Setenv("ALLOWED_EMAILS", "a@x.com, B@X.com ,c@x.com")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.Auth.FirebaseProjectID != "busy-bee-prod" {
		t.Errorf("FirebaseProjectID = %q, want busy-bee-prod", cfg.Auth.FirebaseProjectID)
	}
	want := []string{"a@x.com", "B@X.com", "c@x.com"}
	if len(cfg.Auth.AllowedEmails) != len(want) {
		t.Fatalf("AllowedEmails = %v, want %v (comma-separated, trimmed)", cfg.Auth.AllowedEmails, want)
	}
	for i := range want {
		if cfg.Auth.AllowedEmails[i] != want[i] {
			t.Errorf("AllowedEmails[%d] = %q, want %q", i, cfg.Auth.AllowedEmails[i], want[i])
		}
	}
}

func TestLoad_GCSConfig(t *testing.T) {
	t.Chdir(t.TempDir())
	t.Setenv("GCS_BUCKET", "my-bucket")
	t.Setenv("GCS_SIGNER_EMAIL", "sa@proj.iam.gserviceaccount.com")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.GCS.Bucket != "my-bucket" {
		t.Errorf("GCS.Bucket = %q, want my-bucket", cfg.GCS.Bucket)
	}
	if cfg.GCS.SignerEmail != "sa@proj.iam.gserviceaccount.com" {
		t.Errorf("GCS.SignerEmail = %q", cfg.GCS.SignerEmail)
	}
}

func TestLoad_AllowedEmailsEmptyByDefault(t *testing.T) {
	t.Chdir(t.TempDir())

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if len(cfg.Auth.AllowedEmails) != 0 {
		t.Errorf("AllowedEmails = %v, want empty", cfg.Auth.AllowedEmails)
	}
}

func TestLoad_AppEnvSelectsDotEnvFile(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, ".env.qa"), []byte("DB_URL=postgres://qa\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Chdir(dir)
	t.Setenv("APP_ENV", "qa")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.Server.Env != "qa" {
		t.Errorf("Server.Env = %q, want %q", cfg.Server.Env, "qa")
	}
	if cfg.DB.URL != "postgres://qa" {
		t.Errorf("DB.URL = %q, want value from .env.qa", cfg.DB.URL)
	}
}

func TestLoad_LiveConfig(t *testing.T) {
	t.Chdir(t.TempDir())
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	// 即時串流固定 nova-3（2026-10-15 實測 zh-TW 串流可用、輸出繁體；nova-2 zh-TW 串流連線失敗）
	if cfg.Live.STTModel != "nova-3" || cfg.Live.MonthlyMinutesLimit != 3000 || cfg.Live.MaxContextChars != 100000 || cfg.Live.MaxContextFileBytes != 10<<20 {
		t.Errorf("Live defaults = %+v", cfg.Live)
	}

	t.Setenv("LIVE_MONTHLY_MINUTES_LIMIT", "500")
	t.Setenv("LIVE_CONTEXT_MAX_CHARS", "not-a-number")
	if cfg, _ = Load(); cfg.Live.MonthlyMinutesLimit != 500 || cfg.Live.MaxContextChars != 100000 {
		t.Errorf("Live override = %+v（無效數字回退預設）", cfg.Live)
	}
}
