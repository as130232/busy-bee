package actionitem

import (
	"context"
	"errors"
	"strings"

	"github.com/google/uuid"

	domainactionitem "github.com/as130232/busy-bee/busy-bee-be/domain/actionitem"
	"github.com/as130232/busy-bee/busy-bee-be/pkg/apperr"
	"github.com/as130232/busy-bee/busy-bee-be/pkg/consts/errcode"
)

// EditUC 修改待辦內容（owner-only：UpdateDescription 以 user_id 過濾）。
type EditUC struct {
	items domainactionitem.Repository
}

func NewEditUC(items domainactionitem.Repository) *EditUC {
	return &EditUC{items: items}
}

func (uc *EditUC) Execute(ctx context.Context, userID, itemID uuid.UUID, description string) (domainactionitem.ActionItem, error) {
	desc := strings.TrimSpace(description)
	if desc == "" {
		return domainactionitem.ActionItem{}, apperr.New(errcode.Param)
	}
	item, err := uc.items.UpdateDescription(ctx, itemID, userID, desc)
	if err != nil {
		if errors.Is(err, domainactionitem.ErrNotFound) {
			return domainactionitem.ActionItem{}, apperr.New(errcode.NotFound)
		}
		return domainactionitem.ActionItem{}, apperr.Wrap(err, errcode.Internal)
	}
	return item, nil
}
