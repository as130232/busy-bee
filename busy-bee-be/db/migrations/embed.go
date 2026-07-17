// Package migrations 以 embed 提供 migration 檔，供 cmd/migrate 與測試使用。
package migrations

import "embed"

//go:embed *.sql
var FS embed.FS
