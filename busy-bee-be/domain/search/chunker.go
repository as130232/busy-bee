package search

import "strings"

// SplitIntoChunks 依句子邊界把逐字稿切成約 targetChars 字的塊，
// 每塊開頭帶上一塊最後 overlapSentences 句（避免切斷語意）。空白文字回 nil。
func SplitIntoChunks(text string, targetChars, overlapSentences int) []string {
	sentences := splitSentences(text)
	if len(sentences) == 0 {
		return nil
	}

	var chunks []string
	var cur []string
	curLen := 0
	for _, s := range sentences {
		cur = append(cur, s)
		curLen += len([]rune(s))
		if curLen >= targetChars {
			chunks = append(chunks, strings.Join(cur, ""))
			// 準備下一塊：帶 overlap 句
			if overlapSentences > 0 && len(cur) >= overlapSentences {
				cur = append([]string{}, cur[len(cur)-overlapSentences:]...)
				curLen = 0
				for _, s := range cur {
					curLen += len([]rune(s))
				}
			} else {
				cur = nil
				curLen = 0
			}
		}
	}
	if len(cur) > 0 {
		last := strings.Join(cur, "")
		// 若最後殘塊與前一塊完全相同（純 overlap），不重複加
		if len(chunks) == 0 || chunks[len(chunks)-1] != last {
			chunks = append(chunks, last)
		}
	}
	return chunks
}

// splitSentences 依中英文句末標點切句，保留標點於句尾。
func splitSentences(text string) []string {
	text = strings.TrimSpace(text)
	if text == "" {
		return nil
	}
	var out []string
	var b strings.Builder
	for _, r := range text {
		b.WriteRune(r)
		switch r {
		case '。', '！', '？', '.', '!', '?', '\n':
			s := strings.TrimSpace(b.String())
			if s != "" {
				out = append(out, s)
			}
			b.Reset()
		}
	}
	if s := strings.TrimSpace(b.String()); s != "" {
		out = append(out, s)
	}
	return out
}
