package db

import (
	"context"
	"fmt"
	"testing"
	"time"

	domainuser "github.com/as130232/busy-bee/busy-bee-be/domain/user"
)

func TestUserRepo_UpsertByFirebaseUID(t *testing.T) {
	pool := testPool(t)
	repo := NewUserRepo(pool)
	uid := fmt.Sprintf("test-uid-%d", time.Now().UnixNano())
	t.Cleanup(func() {
		pool.Exec(context.Background(), "DELETE FROM users WHERE firebase_uid = $1", uid)
	})

	first, err := repo.UpsertByFirebaseUID(context.Background(), domainuser.Identity{
		UID: uid, Email: "old@x.com", Name: "Old", Picture: "http://old",
	})
	if err != nil {
		t.Fatalf("first upsert error = %v", err)
	}
	if first.Email != "old@x.com" || first.FirebaseUID != uid {
		t.Errorf("first = %+v, want inserted values", first)
	}

	second, err := repo.UpsertByFirebaseUID(context.Background(), domainuser.Identity{
		UID: uid, Email: "new@x.com", Name: "New", Picture: "http://new",
	})
	if err != nil {
		t.Fatalf("second upsert error = %v", err)
	}
	if second.ID != first.ID {
		t.Errorf("second.ID = %v, want same row as first %v (upsert, not new insert)", second.ID, first.ID)
	}
	if second.Email != "new@x.com" || second.DisplayName != "New" {
		t.Errorf("second = %+v, want updated email/name", second)
	}
}
