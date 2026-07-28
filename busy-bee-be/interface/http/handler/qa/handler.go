// Package qa 提供跨會議 RAG 問答 HTTP handler（單次問答、無狀態）。
package qa

import (
	appsearch "github.com/as130232/busy-bee/busy-bee-be/application/search"
)

// maxQuestionLen 問題長度上界（rune）；超過即截斷，防止超長輸入打進向量嵌入與 LLM（成本護欄）。
const maxQuestionLen = 500

type Handler struct {
	uc *appsearch.QAUC
}

func NewHandler(uc *appsearch.QAUC) *Handler {
	return &Handler{uc: uc}
}
