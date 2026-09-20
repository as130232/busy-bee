package stt

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	domainmeeting "github.com/as130232/busy-bee/busy-bee-be/domain/meeting"
)

func TestAggregateDeepgramWords(t *testing.T) {
	// 模擬 Deepgram 回應：講者 0 說兩個字、講者 1 說一句、講者 0 再接一句英文詞。
	const raw = `{
      "metadata": {"duration": 4.2},
      "results": {"channels": [{"alternatives": [{"words": [
        {"word": "你好", "punctuated_word": "你好，", "start": 0.0, "end": 0.6, "speaker": 0},
        {"word": "大家", "punctuated_word": "大家", "start": 0.6, "end": 1.1, "speaker": 0},
        {"word": "對", "punctuated_word": "對。", "start": 1.2, "end": 1.8, "speaker": 1},
        {"word": "deploy", "punctuated_word": "deploy", "start": 2.0, "end": 2.5, "speaker": 0},
        {"word": "完成", "punctuated_word": "完成", "start": 2.5, "end": 3.0, "speaker": 0}
      ]}]}]}
    }`

	var dr deepgramResponse
	if err := json.Unmarshal([]byte(raw), &dr); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	segs := aggregateDeepgramWords(dr)

	// 期望聚合成 3 段：A(你好，大家) → B(對。) → A(deploy 完成)
	if len(segs) != 3 {
		t.Fatalf("segments = %d, want 3: %+v", len(segs), segs)
	}
	if segs[0].Speaker != "A" || segs[0].Text != "你好，大家" {
		t.Errorf("seg0 = %+v, want A/你好，大家", segs[0])
	}
	if segs[1].Speaker != "B" || segs[1].Text != "對。" {
		t.Errorf("seg1 = %+v, want B/對。", segs[1])
	}
	// 英文詞與中文之間應加空白
	if segs[2].Speaker != "A" || segs[2].Text != "deploy 完成" {
		t.Errorf("seg2 = %+v, want A/'deploy 完成'", segs[2])
	}
	// 時間碼：seg0 從 0 到 1100ms
	if segs[0].StartMs != 0 || segs[0].EndMs != 1100 {
		t.Errorf("seg0 time = %d-%d, want 0-1100", segs[0].StartMs, segs[0].EndMs)
	}
}

func TestBuildDeepgramQuery(t *testing.T) {
	cases := []struct {
		name           string
		lang           string
		wantLanguage   string
		wantHasLang    bool
		wantDetectLang string
		wantHasDetect  bool
	}{
		{"明確語言", "zh-TW", "zh-TW", true, "", false},
		{"auto 走 detect_language 不走 multi", "auto", "", false, "true", true},
		{"空值兩者皆不帶", "", "", false, "", false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			q := buildDeepgramQuery("nova-3", c.lang, nil)
			if got, has := q["language"]; has != c.wantHasLang || (has && got[0] != c.wantLanguage) {
				t.Errorf("language = %v (has=%v), want %q (has=%v)", got, has, c.wantLanguage, c.wantHasLang)
			}
			if got, has := q["detect_language"]; has != c.wantHasDetect || (has && got[0] != c.wantDetectLang) {
				t.Errorf("detect_language = %v (has=%v), want %q (has=%v)", got, has, c.wantDetectLang, c.wantHasDetect)
			}
			// language=multi 對純中文音訊會壞掉（已實測），任何情況都不應出現
			if q.Get("language") == "multi" {
				t.Error("不應使用 language=multi")
			}
		})
	}
}

func TestTranscribe_AutoDetectedChinese_ConvertsToTraditional(t *testing.T) {
	// 簡體「国」在偵測為中文（auto）時應轉換為繁體「國」。
	const raw = `{
      "metadata": {"duration": 1.0},
      "results": {"channels": [{
        "detected_language": "zh",
        "alternatives": [{"words": [
          {"word": "国", "punctuated_word": "国", "start": 0.0, "end": 0.5, "speaker": 0}
        ]}]
      }]}
    }`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(raw))
	}))
	defer srv.Close()

	c := NewDeepgram("k", "nova-3", "zh-TW", nil, WithDeepgramBaseURL(srv.URL))
	result, err := c.Transcribe(context.Background(), strings.NewReader("audio"), 5, "a.wav", domainmeeting.LanguageAuto)
	if err != nil {
		t.Fatalf("Transcribe() error = %v", err)
	}
	if result.Text != "A: 國" {
		t.Errorf("Text = %q, want \"A: 國\"（簡轉繁後）", result.Text)
	}
}

func TestTranscribe_ExplicitZhTW_DoesNotConvert(t *testing.T) {
	// 明確指定 zh-TW（非 auto）時，即使回應帶 detected_language 也不應觸發簡轉繁。
	const raw = `{
      "metadata": {"duration": 1.0},
      "results": {"channels": [{
        "detected_language": "zh",
        "alternatives": [{"words": [
          {"word": "国", "punctuated_word": "国", "start": 0.0, "end": 0.5, "speaker": 0}
        ]}]
      }]}
    }`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(raw))
	}))
	defer srv.Close()

	c := NewDeepgram("k", "nova-3", "zh-TW", nil, WithDeepgramBaseURL(srv.URL))
	result, err := c.Transcribe(context.Background(), strings.NewReader("audio"), 5, "a.wav", domainmeeting.LanguageZhTW)
	if err != nil {
		t.Fatalf("Transcribe() error = %v", err)
	}
	if result.Text != "A: 国" {
		t.Errorf("Text = %q, want \"A: 国\"（不應被轉換）", result.Text)
	}
}
