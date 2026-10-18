//go:build integration

package db

import (
	"context"
	"testing"

	domainpush "github.com/as130232/busy-bee/busy-bee-be/domain/push"
)

// 刪除推播訂閱必須同時比對 user_id：知道別人的 endpoint 也刪不掉別人的訂閱。
func TestPushRepo_DeleteScopedToUser(t *testing.T) {
	pool := testPool(t)
	owner, other := testUser(t, pool), testUser(t, pool)
	repo := NewPushRepo(pool)
	ctx := context.Background()
	endpoint := "https://fcm.googleapis.com/fcm/send/" + owner.ID.String()
	t.Cleanup(func() { pool.Exec(ctx, "DELETE FROM push_subscriptions WHERE endpoint = $1", endpoint) })

	if _, err := repo.Upsert(ctx, domainpush.Subscription{UserID: owner.ID, Endpoint: endpoint, P256dh: "p", Auth: "a"}); err != nil {
		t.Fatal(err)
	}

	if err := repo.Delete(ctx, other.ID, endpoint); err != nil {
		t.Fatal(err)
	}
	if subs, _ := repo.ListByUser(ctx, owner.ID); len(subs) != 1 {
		t.Fatalf("other user's delete removed owner's subscription: %d left", len(subs))
	}

	if err := repo.Delete(ctx, owner.ID, endpoint); err != nil {
		t.Fatal(err)
	}
	if subs, _ := repo.ListByUser(ctx, owner.ID); len(subs) != 0 {
		t.Fatalf("owner delete left %d subscriptions", len(subs))
	}
}
