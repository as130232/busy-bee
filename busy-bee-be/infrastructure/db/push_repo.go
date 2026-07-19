package db

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	domainpush "github.com/as130232/busy-bee/busy-bee-be/domain/push"
	"github.com/as130232/busy-bee/busy-bee-be/infrastructure/db/sqlcgen"
)

// PushRepo 以 sqlc 實作 domain/push.Repository。
type PushRepo struct {
	q *sqlcgen.Queries
}

var _ domainpush.Repository = (*PushRepo)(nil)

func NewPushRepo(pool *pgxpool.Pool) *PushRepo {
	return &PushRepo{q: sqlcgen.New(pool)}
}

func (r *PushRepo) Upsert(ctx context.Context, sub domainpush.Subscription) (domainpush.Subscription, error) {
	row, err := r.q.UpsertPushSubscription(ctx, sqlcgen.UpsertPushSubscriptionParams{
		UserID:    sub.UserID,
		Endpoint:  sub.Endpoint,
		P256dhKey: sub.P256dh,
		AuthKey:   sub.Auth,
	})
	if err != nil {
		return domainpush.Subscription{}, fmt.Errorf("db.UpsertPushSubscription: %w", err)
	}
	return toDomainSub(row), nil
}

func (r *PushRepo) DeleteByEndpoint(ctx context.Context, endpoint string) error {
	if err := r.q.DeletePushSubscriptionByEndpoint(ctx, endpoint); err != nil {
		return fmt.Errorf("db.DeletePushSubscription: %w", err)
	}
	return nil
}

func (r *PushRepo) ListByUser(ctx context.Context, userID uuid.UUID) ([]domainpush.Subscription, error) {
	rows, err := r.q.ListPushSubscriptionsByUser(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("db.ListPushSubscriptions: %w", err)
	}
	out := make([]domainpush.Subscription, len(rows))
	for i, row := range rows {
		out[i] = toDomainSub(row)
	}
	return out, nil
}

func toDomainSub(row sqlcgen.PushSubscription) domainpush.Subscription {
	return domainpush.Subscription{
		ID:       row.ID,
		UserID:   row.UserID,
		Endpoint: row.Endpoint,
		P256dh:   row.P256dhKey,
		Auth:     row.AuthKey,
	}
}
