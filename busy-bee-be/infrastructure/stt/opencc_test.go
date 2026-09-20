package stt

import "testing"

func TestToTraditional(t *testing.T) {
	if s2tw == nil {
		t.Fatal("s2tw 未初始化，OpenCC 詞典 embed 可能有問題")
	}
	if got := toTraditional("国"); got != "國" {
		t.Errorf("toTraditional(国) = %q, want 國", got)
	}
	if got := toTraditional(""); got != "" {
		t.Errorf("toTraditional(\"\") = %q, want empty", got)
	}
}

func TestToTraditional_UninitializedDegrades(t *testing.T) {
	saved := s2tw
	s2tw = nil
	defer func() { s2tw = saved }()

	if got := toTraditional("国"); got != "国" {
		t.Errorf("toTraditional(国) with nil s2tw = %q, want 国（原文降級）", got)
	}
}
