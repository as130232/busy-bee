package user

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"

	domainuser "github.com/as130232/busy-bee/busy-bee-be/domain/user"
	"github.com/as130232/busy-bee/busy-bee-be/pkg/apperr"
	"github.com/as130232/busy-bee/busy-bee-be/pkg/consts/errcode"
)

type fakeRepo struct {
	gotIdentity domainuser.Identity
	returnUser  domainuser.User
	err         error
}

func (f *fakeRepo) UpsertByFirebaseUID(_ context.Context, id domainuser.Identity) (domainuser.User, error) {
	f.gotIdentity = id
	return f.returnUser, f.err
}

func (f *fakeRepo) GetByFirebaseUID(_ context.Context, _ string) (domainuser.User, error) {
	return f.returnUser, f.err
}

func TestSync_UpsertsAndReturnsUser(t *testing.T) {
	want := domainuser.User{ID: uuid.New(), FirebaseUID: "u1", Email: "a@x.com"}
	repo := &fakeRepo{returnUser: want}
	uc := NewSyncUC(repo)

	identity := domainuser.Identity{UID: "u1", Email: "a@x.com", Name: "Alice"}
	got, err := uc.Execute(context.Background(), identity)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if got.ID != want.ID {
		t.Errorf("user.ID = %v, want %v", got.ID, want.ID)
	}
	if repo.gotIdentity != identity {
		t.Errorf("repo received %+v, want %+v", repo.gotIdentity, identity)
	}
}

func TestSync_EmptyUIDReturnsParamError(t *testing.T) {
	uc := NewSyncUC(&fakeRepo{})

	_, err := uc.Execute(context.Background(), domainuser.Identity{UID: "", Email: "a@x.com"})

	var ae *apperr.Error
	if !errors.As(err, &ae) || ae.Code != errcode.Param {
		t.Fatalf("err = %v, want apperr with Param code", err)
	}
}

func TestSync_RepoErrorWrapped(t *testing.T) {
	cause := errors.New("db down")
	uc := NewSyncUC(&fakeRepo{err: cause})

	_, err := uc.Execute(context.Background(), domainuser.Identity{UID: "u1", Email: "a@x.com"})

	if !errors.Is(err, cause) {
		t.Fatalf("err = %v, want cause preserved in chain", err)
	}
	var ae *apperr.Error
	if !errors.As(err, &ae) || ae.Code != errcode.Internal {
		t.Fatalf("err = %v, want apperr Internal", err)
	}
}
