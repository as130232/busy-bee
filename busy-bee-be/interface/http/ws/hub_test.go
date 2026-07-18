package ws

import (
	"context"
	"encoding/json"
	"errors"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	domainmeeting "github.com/as130232/busy-bee/busy-bee-be/domain/meeting"
	domainuser "github.com/as130232/busy-bee/busy-bee-be/domain/user"
)

type fakeVerifier struct {
	identity domainuser.Identity
	err      error
}

func (f fakeVerifier) Verify(_ context.Context, _ string) (domainuser.Identity, error) {
	return f.identity, f.err
}

type fakeUserRepo struct {
	user domainuser.User
	err  error
}

func (f *fakeUserRepo) UpsertByFirebaseUID(_ context.Context, _ domainuser.Identity) (domainuser.User, error) {
	return f.user, f.err
}
func (f *fakeUserRepo) GetByFirebaseUID(_ context.Context, _ string) (domainuser.User, error) {
	return f.user, f.err
}

func setupWS(t *testing.T, verifier domainuser.TokenVerifier, repo domainuser.Repository, allowed []string) (*Hub, string) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	hub := NewHub()

	e := gin.New()
	e.GET("/ws", hub.Handler(verifier, repo, allowed))
	srv := httptest.NewServer(e)
	t.Cleanup(srv.Close)
	t.Cleanup(hub.Close)

	return hub, "ws" + strings.TrimPrefix(srv.URL, "http") + "/ws"
}

func dial(t *testing.T, url string) *websocket.Conn {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	conn, _, err := websocket.Dial(ctx, url, nil)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	return conn
}

func sendAuth(t *testing.T, conn *websocket.Conn, token string) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	msg, _ := json.Marshal(map[string]string{"type": "auth", "token": token})
	if err := conn.Write(ctx, websocket.MessageText, msg); err != nil {
		t.Fatalf("write auth: %v", err)
	}
}

func readMsg(t *testing.T, conn *websocket.Conn, timeout time.Duration) (map[string]any, error) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	_, data, err := conn.Read(ctx)
	if err != nil {
		return nil, err
	}
	var m map[string]any
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatalf("invalid message JSON: %s", data)
	}
	return m, nil
}

func TestWS_BadTokenClosed(t *testing.T) {
	_, url := setupWS(t, fakeVerifier{err: errors.New("bad token")}, &fakeUserRepo{}, []string{"a@x.com"})
	conn := dial(t, url)
	defer conn.CloseNow()

	sendAuth(t, conn, "garbage")

	if _, err := readMsg(t, conn, 2*time.Second); err == nil {
		// 若有回訊息應為 error 類型後關閉；再讀必須失敗
		if _, err2 := readMsg(t, conn, 2*time.Second); err2 == nil {
			t.Fatal("connection should be closed after bad token")
		}
	}
}

func TestWS_NotWhitelistedClosed(t *testing.T) {
	v := fakeVerifier{identity: domainuser.Identity{UID: "u", Email: "stranger@x.com"}}
	_, url := setupWS(t, v, &fakeUserRepo{}, []string{"a@x.com"})
	conn := dial(t, url)
	defer conn.CloseNow()

	sendAuth(t, conn, "tok")

	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if _, err := readMsg(t, conn, time.Second); err != nil {
			return // closed as expected
		}
	}
	t.Fatal("connection should be closed for non-whitelisted email")
}

func TestWS_AuthOkAndReceivesOwnEvents(t *testing.T) {
	me := domainuser.User{ID: uuid.New(), FirebaseUID: "fb-1"}
	v := fakeVerifier{identity: domainuser.Identity{UID: "fb-1", Email: "a@x.com"}}
	hub, url := setupWS(t, v, &fakeUserRepo{user: me}, []string{"a@x.com"})

	conn := dial(t, url)
	defer conn.CloseNow()
	sendAuth(t, conn, "tok")

	ack, err := readMsg(t, conn, 2*time.Second)
	if err != nil || ack["type"] != "authOk" {
		t.Fatalf("ack = %v, err = %v, want authOk", ack, err)
	}

	meetingID := uuid.New()
	hub.NotifyStatus(context.Background(), domainmeeting.StatusEvent{
		MeetingID: meetingID, UserID: me.ID, Status: domainmeeting.StatusTranscribing,
	})

	msg, err := readMsg(t, conn, 2*time.Second)
	if err != nil {
		t.Fatalf("read event: %v", err)
	}
	if msg["type"] != "meetingStatus" || msg["status"] != "transcribing" || msg["meetingId"] != meetingID.String() {
		t.Errorf("event = %v", msg)
	}
}

func TestWS_DoesNotReceiveOthersEvents(t *testing.T) {
	me := domainuser.User{ID: uuid.New(), FirebaseUID: "fb-1"}
	v := fakeVerifier{identity: domainuser.Identity{UID: "fb-1", Email: "a@x.com"}}
	hub, url := setupWS(t, v, &fakeUserRepo{user: me}, []string{"a@x.com"})

	conn := dial(t, url)
	defer conn.CloseNow()
	sendAuth(t, conn, "tok")
	if ack, err := readMsg(t, conn, 2*time.Second); err != nil || ack["type"] != "authOk" {
		t.Fatal("auth failed")
	}

	hub.NotifyStatus(context.Background(), domainmeeting.StatusEvent{
		MeetingID: uuid.New(), UserID: uuid.New(), Status: domainmeeting.StatusCompleted, // 別人的事件
	})

	if msg, err := readMsg(t, conn, 500*time.Millisecond); err == nil {
		t.Errorf("received other user's event: %v", msg)
	}
}

func TestWS_MultipleConnectionsSameUserAllReceive(t *testing.T) {
	me := domainuser.User{ID: uuid.New(), FirebaseUID: "fb-1"}
	v := fakeVerifier{identity: domainuser.Identity{UID: "fb-1", Email: "a@x.com"}}
	hub, url := setupWS(t, v, &fakeUserRepo{user: me}, []string{"a@x.com"})

	conns := make([]*websocket.Conn, 2)
	for i := range conns {
		conns[i] = dial(t, url)
		defer conns[i].CloseNow()
		sendAuth(t, conns[i], "tok")
		if ack, err := readMsg(t, conns[i], 2*time.Second); err != nil || ack["type"] != "authOk" {
			t.Fatal("auth failed")
		}
	}

	hub.NotifyStatus(context.Background(), domainmeeting.StatusEvent{
		MeetingID: uuid.New(), UserID: me.ID, Status: domainmeeting.StatusCompleted,
	})

	for i, c := range conns {
		msg, err := readMsg(t, c, 2*time.Second)
		if err != nil || msg["status"] != "completed" {
			t.Errorf("conn %d: msg = %v, err = %v", i, msg, err)
		}
	}
}
