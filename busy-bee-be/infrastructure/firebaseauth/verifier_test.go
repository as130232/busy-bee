package firebaseauth

import "testing"

// email_verified 只有明確為 bool true 才算已驗證；缺少或型別不對一律視為未驗證。
func TestClaimBool_EmailVerified(t *testing.T) {
	cases := []struct {
		claims map[string]any
		want   bool
	}{
		{map[string]any{"email_verified": true}, true},
		{map[string]any{"email_verified": false}, false},
		{map[string]any{"email_verified": "true"}, false},
		{map[string]any{}, false},
	}
	for _, c := range cases {
		if got := claimBool(c.claims, "email_verified"); got != c.want {
			t.Errorf("claimBool(%v) = %v, want %v", c.claims, got, c.want)
		}
	}
}
