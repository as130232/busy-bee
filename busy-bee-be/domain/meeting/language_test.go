package meeting

import "testing"

func TestParseLanguage(t *testing.T) {
	cases := []struct {
		in   string
		want Language
	}{
		{"zh-TW", LanguageZhTW},
		{"en-US", LanguageEnUS},
		{"auto", LanguageAuto},
		{"", LanguageZhTW},        // 空值回退預設
		{"unknown", LanguageZhTW}, // 未知值回退預設（容忍舊資料/未來值）
		{"ZH-TW", LanguageZhTW},   // 大小寫不符即無效，回退
	}
	for _, c := range cases {
		if got := ParseLanguage(c.in); got != c.want {
			t.Errorf("ParseLanguage(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestLanguageIsValid(t *testing.T) {
	if !LanguageZhTW.IsValid() || !LanguageEnUS.IsValid() || !LanguageAuto.IsValid() {
		t.Error("zh-TW/en-US/auto should be valid")
	}
	if Language("nope").IsValid() {
		t.Error("unknown language should be invalid")
	}
}
