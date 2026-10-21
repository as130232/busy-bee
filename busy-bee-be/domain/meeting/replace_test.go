package meeting

import "testing"

func TestReplaceText(t *testing.T) {
	cases := []struct {
		name, in, from, to, want string
		n                        int
	}{
		{"英文縮寫整字取代", "DBA 報告給 DBA 看", "DBA", "DVA", "DVA 報告給 DVA 看", 2},
		{"不改到較長單字", "DBAs 和 MyDBA 與 DBA", "DBA", "DVA", "DBAs 和 MyDBA 與 DVA", 1},
		{"中英相鄰仍算邊界", "請看DBA報告", "DBA", "DVA", "請看DVA報告", 1},
		{"大小寫須相符", "dba 與 DBA", "DBA", "DVA", "dba 與 DVA", 1},
		{"數字邊界", "v2 與 v20", "v2", "v3", "v3 與 v20", 1},
		{"中文直接取代", "王曉明和王曉明說", "王曉明", "王小明", "王小明和王小明說", 2},
		{"無符合", "沒有", "DBA", "DVA", "沒有", 0},
		{"取代結果含原字不重複處理", "AB", "AB", "ABAB", "ABAB", 1},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, n := ReplaceText(c.in, ReplaceRule{From: c.from, To: c.to})
			if got != c.want || n != c.n {
				t.Errorf("ReplaceText(%q) = %q, %d; want %q, %d", c.in, got, n, c.want, c.n)
			}
		})
	}
}

func TestReplaceRule_Validate(t *testing.T) {
	bad := []ReplaceRule{{From: " ", To: "x"}, {From: "a", To: ""}, {From: "a", To: "a"}, {From: string(make([]rune, 101)), To: "x"}}
	for _, r := range bad {
		if r.Normalize().Valid() {
			t.Errorf("rule %+v should be invalid", r)
		}
	}
	if r := (ReplaceRule{From: " DBA ", To: " DVA "}).Normalize(); !r.Valid() || r.From != "DBA" || r.To != "DVA" {
		t.Errorf("normalize = %+v", r)
	}
}
