package db

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"

	domainuser "github.com/as130232/busy-bee/busy-bee-be/domain/user"
	"github.com/as130232/busy-bee/busy-bee-be/infrastructure/db/sqlcgen"
)

// UserRepo 以 sqlc 實作 domain/user.Repository。
type UserRepo struct {
	q *sqlcgen.Queries
}

var _ domainuser.Repository = (*UserRepo)(nil)

func NewUserRepo(pool *pgxpool.Pool) *UserRepo {
	return &UserRepo{q: sqlcgen.New(pool)}
}

func (r *UserRepo) UpsertByFirebaseUID(ctx context.Context, id domainuser.Identity) (domainuser.User, error) {
	row, err := r.q.UpsertUserByFirebaseUID(ctx, sqlcgen.UpsertUserByFirebaseUIDParams{
		FirebaseUid: id.UID,
		Email:       id.Email,
		DisplayName: id.Name,
		AvatarUrl:   id.Picture,
	})
	if err != nil {
		return domainuser.User{}, fmt.Errorf("db.UpsertUserByFirebaseUID: %w", err)
	}
	return toDomainUser(row), nil
}

func toDomainUser(row sqlcgen.User) domainuser.User {
	return domainuser.User{
		ID:          row.ID,
		FirebaseUID: row.FirebaseUid,
		Email:       row.Email,
		DisplayName: row.DisplayName,
		AvatarURL:   row.AvatarUrl,
		CreatedAt:   row.CreatedAt,
		UpdatedAt:   row.UpdatedAt,
	}
}
