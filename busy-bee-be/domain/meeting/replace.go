package meeting

import (
	"strings"
	"unicode/utf8"
)

// MaxReplaceRuneLen 取代規則單邊長度上限（rune）。
const MaxReplaceRuneLen = 100

// ReplaceRule 一組「錯 → 對」文字取代（修正 STT 錯字，如 DBA → DVA）。
// 大小寫須相符；From 以英數字開頭／結尾時，該側須落在字詞邊界（DBA 不會改到 DBAs、MyDBA）。
type ReplaceRule struct {
	From string `json:"from"`
	To   string `json:"to"`
}

// Normalize 去除前後空白。
func (r ReplaceRule) Normalize() ReplaceRule {
	return ReplaceRule{From: strings.TrimSpace(r.From), To: strings.TrimSpace(r.To)}
}

// Valid 兩邊皆非空、不相同且不超過長度上限（呼叫前先 Normalize）。
func (r ReplaceRule) Valid() bool {
	return r.From != "" && r.To != "" && r.From != r.To &&
		utf8.RuneCountInString(r.From) <= MaxReplaceRuneLen && utf8.RuneCountInString(r.To) <= MaxReplaceRuneLen
}

// ReplaceText 依規則取代 s 中所有符合處，回傳結果與取代次數。
func ReplaceText(s string, r ReplaceRule) (string, int) {
	if r.From == "" || !strings.Contains(s, r.From) {
		return s, 0
	}
	first, _ := utf8.DecodeRuneInString(r.From)
	last, _ := utf8.DecodeLastRuneInString(r.From)
	checkLeft, checkRight := isASCIIWordRune(first), isASCIIWordRune(last)

	var b strings.Builder
	n, i := 0, 0
	for {
		j := strings.Index(s[i:], r.From)
		if j < 0 {
			break
		}
		start, end := i+j, i+j+len(r.From)
		if (checkLeft && start > 0 && isASCIIWordByte(s[start-1])) || (checkRight && end < len(s) && isASCIIWordByte(s[end])) {
			b.WriteString(s[i : start+1])
			i = start + 1
			continue
		}
		b.WriteString(s[i:start])
		b.WriteString(r.To)
		n++
		i = end
	}
	if n == 0 {
		return s, 0
	}
	b.WriteString(s[i:])
	return b.String(), n
}

func isASCIIWordRune(r rune) bool { return r < utf8.RuneSelf && isASCIIWordByte(byte(r)) }

func isASCIIWordByte(c byte) bool {
	return c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9'
}
