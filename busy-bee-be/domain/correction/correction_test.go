package correction

import "testing"

func TestStateFromLatest(t *testing.T) {
	cases := []struct {
		name   string
		latest *Correction
		count  int
		want   State
	}{
		{"無紀錄", nil, 0, State{}},
		{"完成可還原", &Correction{Status: StatusCompleted}, 1, State{Status: StatusCompleted, AttemptsUsed: 1, CanRevert: true}},
		{"已還原不可再還原", &Correction{Status: StatusReverted}, 2, State{Status: StatusReverted, AttemptsUsed: 2}},
		{"失敗帶訊息", &Correction{Status: StatusFailed, ErrorMessage: "x"}, 1, State{Status: StatusFailed, AttemptsUsed: 1, ErrorMessage: "x"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := StateFromLatest(c.latest, c.count); got != c.want {
				t.Errorf("got %+v, want %+v", got, c.want)
			}
		})
	}
}
