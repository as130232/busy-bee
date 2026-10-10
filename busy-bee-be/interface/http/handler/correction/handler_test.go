package correction

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	domaincorrection "github.com/as130232/busy-bee/busy-bee-be/domain/correction"
	domainuser "github.com/as130232/busy-bee/busy-bee-be/domain/user"
	"github.com/as130232/busy-bee/busy-bee-be/pkg/apperr"
	"github.com/as130232/busy-bee/busy-bee-be/pkg/consts/errcode"
)

type fakeUC struct {
	state     domaincorrection.State
	err       error
	gotUser   uuid.UUID
	gotMeetID uuid.UUID
}

func (f *fakeUC) Execute(_ context.Context, userID, meetingID uuid.UUID) (domaincorrection.State, error) {
	f.gotUser, f.gotMeetID = userID, meetingID
	return f.state, f.err
}

var testUserID = uuid.New()

func router(get, start, revert *fakeUC) *gin.Engine {
	gin.SetMode(gin.TestMode)
	e := gin.New()
	e.Use(func(c *gin.Context) {
		c.Request = c.Request.WithContext(domainuser.WithID(c.Request.Context(), testUserID))
	})
	h := NewHandler(get, start, revert)
	e.GET("/meetings/:id/correction", h.Get)
	e.POST("/meetings/:id/correction", h.Start)
	e.POST("/meetings/:id/correction/revert", h.Revert)
	return e
}

type envelope struct {
	ErrCode int `json:"errCode"`
	Data    struct {
		Correction struct {
			Status       string `json:"status"`
			AttemptsUsed int    `json:"attemptsUsed"`
			AttemptsMax  int    `json:"attemptsMax"`
			CanRevert    bool   `json:"canRevert"`
			ErrorMessage string `json:"errorMessage"`
		} `json:"correction"`
	} `json:"data"`
}

func do(t *testing.T, e *gin.Engine, method, path string) (*httptest.ResponseRecorder, envelope) {
	t.Helper()
	w := httptest.NewRecorder()
	e.ServeHTTP(w, httptest.NewRequest(method, path, nil))
	var env envelope
	if err := json.Unmarshal(w.Body.Bytes(), &env); err != nil {
		t.Fatalf("invalid JSON %s: %v", w.Body.String(), err)
	}
	return w, env
}

func TestStart_Returns200WithState(t *testing.T) {
	start := &fakeUC{state: domaincorrection.State{Status: domaincorrection.StatusQueued, AttemptsUsed: 1}}
	mid := uuid.New()
	w, env := do(t, router(&fakeUC{}, start, &fakeUC{}), http.MethodPost, "/meetings/"+mid.String()+"/correction")
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d body = %s", w.Code, w.Body.String())
	}
	c := env.Data.Correction
	if c.Status != "queued" || c.AttemptsUsed != 1 || c.AttemptsMax != 2 || c.CanRevert {
		t.Errorf("correction = %+v", c)
	}
	if start.gotUser != testUserID || start.gotMeetID != mid {
		t.Errorf("ids = %v %v", start.gotUser, start.gotMeetID)
	}
}

func TestStart_LimitReachedMaps409(t *testing.T) {
	start := &fakeUC{err: apperr.New(errcode.CorrectionLimitReached)}
	w, env := do(t, router(&fakeUC{}, start, &fakeUC{}), http.MethodPost, "/meetings/"+uuid.NewString()+"/correction")
	if w.Code != http.StatusConflict || env.ErrCode != int(errcode.CorrectionLimitReached) {
		t.Errorf("status = %d errCode = %d", w.Code, env.ErrCode)
	}
}

func TestGet_EmptyStateHasEmptyStatus(t *testing.T) {
	w, env := do(t, router(&fakeUC{}, &fakeUC{}, &fakeUC{}), http.MethodGet, "/meetings/"+uuid.NewString()+"/correction")
	if w.Code != http.StatusOK || env.Data.Correction.Status != "" || env.Data.Correction.AttemptsMax != 2 {
		t.Errorf("status = %d correction = %+v", w.Code, env.Data.Correction)
	}
}

func TestRevert_RoutesToRevertUC(t *testing.T) {
	revert := &fakeUC{state: domaincorrection.State{Status: domaincorrection.StatusReverted, AttemptsUsed: 1}}
	w, env := do(t, router(&fakeUC{}, &fakeUC{}, revert), http.MethodPost, "/meetings/"+uuid.NewString()+"/correction/revert")
	if w.Code != http.StatusOK || env.Data.Correction.Status != "reverted" {
		t.Errorf("status = %d correction = %+v", w.Code, env.Data.Correction)
	}
}

func TestGet_InvalidIDIs400(t *testing.T) {
	w, _ := do(t, router(&fakeUC{}, &fakeUC{}, &fakeUC{}), http.MethodGet, "/meetings/abc/correction")
	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d", w.Code)
	}
}
