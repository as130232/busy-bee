package actionitem

import (
	"context"
	"errors"

	"github.com/google/uuid"

	domainactionitem "github.com/as130232/busy-bee/busy-bee-be/domain/actionitem"
	"github.com/as130232/busy-bee/busy-bee-be/pkg/apperr"
	"github.com/as130232/busy-bee/busy-bee-be/pkg/consts/errcode"
)

// DeleteUC 刪除單筆待辦（owner-only：Delete 以 user_id 過濾）。
type DeleteUC struct {
	items domainactionitem.Repository
}

func NewDeleteUC(items domainactionitem.Repository) *DeleteUC {
	return &DeleteUC{items: items}
}

func (uc *DeleteUC) Execute(ctx context.Context, userID, itemID uuid.UUID) error {
	if err := uc.items.Delete(ctx, itemID, userID); err != nil {
		if errors.Is(err, domainactionitem.ErrNotFound) {
			return apperr.New(errcode.NotFound)
		}
		return apperr.Wrap(err, errcode.Internal)
	}
	return nil
}
