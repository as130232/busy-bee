package db

import (
	"testing"

	"github.com/as130232/busy-bee/busy-bee-be/infrastructure/db/sqlcgen"
)

func TestToDomainActionItem_SourceAndEdited(t *testing.T) {
	got := toDomainActionItem(sqlcgen.ActionItem{Source: "manual", Edited: true})
	if got.Source != "manual" || !got.Edited {
		t.Errorf("got Source=%q Edited=%v", got.Source, got.Edited)
	}
}
