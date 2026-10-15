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

	segs := aggregateDeepgramWords(dr, true)

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
	result, err := c.Transcribe(context.Background(), strings.NewReader("audio"), 5, "a.wav", domainmeeting.LanguageAuto, nil)
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
	result, err := c.Transcribe(context.Background(), strings.NewReader("audio"), 5, "a.wav", domainmeeting.LanguageZhTW, nil)
	if err != nil {
		t.Fatalf("Transcribe() error = %v", err)
	}
	if result.Text != "A: 国" {
		t.Errorf("Text = %q, want \"A: 国\"（不應被轉換）", result.Text)
	}
}

func TestAggregateDeepgramWords_MarksLowConfidence(t *testing.T) {
	// 「預算」「三百萬」低信心且相鄰 → 合併成一個標記；英文低信心詞前保留空白；未帶 confidence 視為可信。
	const raw = `{
      "results": {"channels": [{"alternatives": [{"words": [
        {"word": "今年", "punctuated_word": "今年", "start": 0.0, "end": 0.4, "speaker": 0, "confidence": 0.98},
        {"word": "預算", "punctuated_word": "預算", "start": 0.4, "end": 0.8, "speaker": 0, "confidence": 0.41},
        {"word": "三百萬", "punctuated_word": "三百萬。", "start": 0.8, "end": 1.4, "speaker": 0, "confidence": 0.3},
        {"word": "用", "punctuated_word": "用", "start": 1.5, "end": 1.7, "speaker": 0, "confidence": 0.95},
        {"word": "kafka", "punctuated_word": "Kafka", "start": 1.7, "end": 2.2, "speaker": 0, "confidence": 0.2},
        {"word": "好", "punctuated_word": "好。", "start": 2.5, "end": 2.8, "speaker": 1, "confidence": 0.99},
        {"word": "嗯", "punctuated_word": "嗯", "start": 3.0, "end": 3.2, "speaker": 0}
      ]}]}]}
    }`
	var dr deepgramResponse
	if err := json.Unmarshal([]byte(raw), &dr); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	segs := aggregateDeepgramWords(dr, true)
	if len(segs) != 3 {
		t.Fatalf("segments = %d, want 3: %+v", len(segs), segs)
	}
	if segs[0].Text != "今年預算三百萬。用 Kafka" {
		t.Errorf("seg0 Text = %q（不應含標記）", segs[0].Text)
	}
	if segs[0].MarkedText != "今年〔?預算三百萬。〕用 〔?Kafka〕" {
		t.Errorf("seg0 MarkedText = %q", segs[0].MarkedText)
	}
	if segs[1].MarkedText != "" || segs[2].MarkedText != "" {
		t.Errorf("無低信心字詞的段落 MarkedText 應為空：%q / %q", segs[1].MarkedText, segs[2].MarkedText)
	}
	if segs[0].AnalysisText() != segs[0].MarkedText || segs[1].AnalysisText() != "好。" {
		t.Error("AnalysisText 應優先回傳 MarkedText，無標記時回傳 Text")
	}
}

func TestBuildDeepgramQuery_KeywordsByModel(t *testing.T) {
	kws := []string{"Kubernetes:2", "Busy Bee", "a:b"}

	q2 := buildDeepgramQuery("nova-2", "zh-TW", kws)
	if got := q2["keywords"]; len(got) != 3 || got[0] != "Kubernetes:2" {
		t.Errorf("nova-2 keywords = %v, want 原樣帶權重", got)
	}
	if _, has := q2["keyterm"]; has {
		t.Error("nova-2 不應帶 keyterm")
	}

	q3 := buildDeepgramQuery("nova-3", "zh-TW", kws)
	if _, has := q3["keywords"]; has {
		t.Error("nova-3 不支援 keywords，不應帶")
	}
	want := []string{"Kubernetes", "Busy Bee", "a:b"}
	got := q3["keyterm"]
	if len(got) != len(want) {
		t.Fatalf("nova-3 keyterm = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("keyterm[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestAggregateDeepgramWords_JoinsEnglishFragmentsInChinese(t *testing.T) {
	// 實測 nova-3 中文模式回傳：Kubernetes 被切成時間相連的 "k","er","net","es"；
	// "API" 與 "gateway" 中間有停頓（非片段），應保留空白。
	const raw = `{
      "results": {"channels": [{"alternatives": [{"words": [
        {"word": "我們用", "punctuated_word": "我們用", "start": 31.5, "end": 31.98, "speaker": 0},
        {"word": "k", "punctuated_word": "k", "start": 31.983624, "end": 32.303627, "speaker": 0},
        {"word": "er", "punctuated_word": "er", "start": 32.303627, "end": 32.463623, "speaker": 0},
        {"word": "net", "punctuated_word": "net", "start": 32.463623, "end": 32.703625, "speaker": 0},
        {"word": "es", "punctuated_word": "es", "start": 32.703625, "end": 32.943626, "speaker": 0},
        {"word": "部署", "punctuated_word": "部署，", "start": 32.95, "end": 33.4, "speaker": 0},
        {"word": "api", "punctuated_word": "API", "start": 33.5, "end": 33.9, "speaker": 0},
        {"word": "gateway", "punctuated_word": "gateway", "start": 34.2, "end": 34.6, "speaker": 0}
      ]}]}]}
    }`
	var dr deepgramResponse
	if err := json.Unmarshal([]byte(raw), &dr); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if got := aggregateDeepgramWords(dr, true)[0].Text; got != "我們用 kernetes 部署， API gateway" {
		t.Errorf("中文模式 Text = %q", got)
	}
	// 英文模式不接合：英文單字本來就時間相連，接起來會黏成一團
	if got := aggregateDeepgramWords(dr, false)[0].Text; got != "我們用 k er net es 部署， API gateway" {
		t.Errorf("非中文模式 Text = %q", got)
	}
}

func TestTranscribe_MergesMeetingKeyterms(t *testing.T) {
	var got []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = r.URL.Query()["keyterm"]
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"metadata":{"duration":1},"results":{"channels":[{"alternatives":[{"words":[{"word":"好","punctuated_word":"好","start":0,"end":0.5,"speaker":0}]}]}]}}`))
	}))
	defer srv.Close()

	c := NewDeepgram("k", "nova-3", "zh-TW", []string{"Kubernetes:2", "Stage"}, WithDeepgramBaseURL(srv.URL))
	if _, err := c.Transcribe(context.Background(), strings.NewReader("a"), 1, "a.webm", domainmeeting.LanguageZhTW, []string{"stage", "Jenkins"}); err != nil {
		t.Fatal(err)
	}
	if strings.Join(got, ",") != "Kubernetes,Stage,Jenkins" {
		t.Errorf("keyterm = %v（全域優先、不分大小寫去重）", got)
	}
}
