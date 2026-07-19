package actionitem

import (
	"context"
	"errors"

	"github.com/google/uuid"

	domainactionitem "github.com/as130232/busy-bee/busy-bee-be/domain/actionitem"
	"github.com/as130232/busy-bee/busy-bee-be/pkg/apperr"
	"github.com/as130232/busy-bee/busy-bee-be/pkg/consts/errcode"
)

// ToggleUC 標記行動項完成 / 取消完成（owner-only：SetDone 以 user_id 過濾）。
type ToggleUC struct {
	items domainactionitem.Repository
}

func NewToggleUC(items domainactionitem.Repository) *ToggleUC {
	return &ToggleUC{items: items}
}

func (uc *ToggleUC) Execute(ctx context.Context, userID, itemID uuid.UUID, done bool) (domainactionitem.ActionItem, error) {
	item, err := uc.items.SetDone(ctx, itemID, userID, done)
	if err != nil {
		if errors.Is(err, domainactionitem.ErrNotFound) {
			return domainactionitem.ActionItem{}, apperr.New(errcode.NotFound)
		}
		return domainactionitem.ActionItem{}, apperr.Wrap(err, errcode.Internal)
	}
	return item, nil
}
