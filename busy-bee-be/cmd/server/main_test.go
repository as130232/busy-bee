package main

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"testing"
)

func TestCloudLoggingAttr(t *testing.T) {
	cases := []struct {
		log  func(*slog.Logger)
		want string
	}{
		{func(l *slog.Logger) { l.Debug("m") }, "DEBUG"},
		{func(l *slog.Logger) { l.Info("m") }, "INFO"},
		{func(l *slog.Logger) { l.Warn("m") }, "WARNING"},
		{func(l *slog.Logger) { l.Error("m") }, "ERROR"},
	}
	for _, c := range cases {
		var buf bytes.Buffer
		l := slog.New(slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug, ReplaceAttr: cloudLoggingAttr}))
		c.log(l)
		var got map[string]any
		if err := json.Unmarshal(buf.Bytes(), &got); err != nil {
			t.Fatal(err)
		}
		if got["severity"] != c.want {
			t.Errorf("severity = %v, want %s", got["severity"], c.want)
		}
		if _, ok := got["level"]; ok {
			t.Errorf("level key should be replaced: %v", got)
		}
		if got["msg"] != "m" {
			t.Errorf("msg should be kept: %v", got)
		}
	}
}

func TestCloudLoggingAttrKeepsGroupedLevel(t *testing.T) {
	var buf bytes.Buffer
	l := slog.New(slog.NewJSONHandler(&buf, &slog.HandlerOptions{ReplaceAttr: cloudLoggingAttr}))
	l.WithGroup("g").Info("m", "level", "custom")
	var got map[string]any
	if err := json.Unmarshal(buf.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	g, _ := got["g"].(map[string]any)
	if g["level"] != "custom" {
		t.Errorf("grouped level attr should be untouched: %v", got)
	}
}
