package search

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"

	domainmeeting "github.com/as130232/busy-bee/busy-bee-be/domain/meeting"
	domainsearch "github.com/as130232/busy-bee/busy-bee-be/domain/search"
)

const (
	// qaTopK RAG 檢索取回的片段數上界（token 預算：約 400 字 × 8 ≈ 3200 字 context）。
	qaTopK = 8
	// qaMinScore QA 片段相似度下限；比搜尋的 semanticMinScore(0.65) 略放寬以提升召回。
	qaMinScore = 0.6
	// qaCatalogMax 會議清單納入 prompt 的上界（新→舊），控管 token。
	qaCatalogMax = 40
	// qaNoMatchMsg 無任何可用資料時的固定回覆（此路徑不呼叫 LLM，成本護欄）。
	qaNoMatchMsg = "在你的會議紀錄中找不到與這個問題相關的內容。"
)

// meetingCatalog 提供使用者會議目錄（標題/日期/摘要），供 QA 回答列舉、時間型問題。
type meetingCatalog interface {
	ListForUser(ctx context.Context, userID uuid.UUID, search string) ([]domainmeeting.Meeting, error)
}

// QAUC 跨會議 RAG 問答：問題向量化 → 檢索 top-K 片段 + 會議清單 → LLM 生成帶引用答案。
// 無任何可用資料時不呼叫 LLM，直接回提示（成本護欄）。單次問答、無狀態。
type QAUC struct {
	embedder  domainsearch.Embedder
	retriever domainsearch.QARetriever
	answerer  domainsearch.Answerer
	catalog   meetingCatalog
}

func NewQAUC(embedder domainsearch.Embedder, retriever domainsearch.QARetriever, answerer domainsearch.Answerer, catalog meetingCatalog) *QAUC {
	return &QAUC{embedder: embedder, retriever: retriever, answerer: answerer, catalog: catalog}
}

// Execute 回傳答案與引用來源。內容片段供內容型問題（帶 [n]），會議清單供列舉/時間型問題。
// 兩者皆無時回 NoMatch=true（未呼叫 LLM）。
func (uc *QAUC) Execute(ctx context.Context, userID uuid.UUID, question string) (domainsearch.QAResult, error) {
	vec, err := uc.embedder.Embed(ctx, question)
	if err != nil {
		return domainsearch.QAResult{}, fmt.Errorf("qa embed: %w", err)
	}
	chunks, err := uc.retriever.SearchSimilarForQA(ctx, userID, vec, qaTopK)
	if err != nil {
		return domainsearch.QAResult{}, fmt.Errorf("qa retrieve: %w", err)
	}

	// 會議清單（列舉/時間型問題用）；失敗則降級為空清單，不擋內容問答。
	var briefs []domainsearch.MeetingBrief
	if mets, cerr := uc.catalog.ListForUser(ctx, userID, ""); cerr != nil {
		slog.WarnContext(ctx, "qa.catalog_degraded", "err", cerr)
	} else {
		briefs = toMeetingBriefs(mets)
	}

	// 濾除低於門檻者並重新編號（1-based），編號要與傳給 LLM 的來源清單一致。
	sources := make([]domainsearch.QASource, 0, len(chunks))
	for _, ch := range chunks {
		if ch.Score < qaMinScore {
			continue
		}
		sources = append(sources, domainsearch.QASource{
			Index:     len(sources) + 1,
			MeetingID: ch.MeetingID,
			Title:     ch.Title,
			Content:   ch.Content,
		})
	}

	if len(sources) == 0 && len(briefs) == 0 {
		return domainsearch.QAResult{Answer: qaNoMatchMsg, NoMatch: true}, nil
	}

	answer, err := uc.answerer.Answer(ctx, question, sources, briefs)
	if err != nil {
		return domainsearch.QAResult{}, fmt.Errorf("qa answer: %w", err)
	}
	return domainsearch.QAResult{Answer: answer, Sources: sources}, nil
}

// toMeetingBriefs 取最近 qaCatalogMax 場會議轉成目錄項；日期以固定 UTC+8 呈現（容器可能缺 tzdata）。
func toMeetingBriefs(ms []domainmeeting.Meeting) []domainsearch.MeetingBrief {
	if len(ms) > qaCatalogMax {
		ms = ms[:qaCatalogMax]
	}
	out := make([]domainsearch.MeetingBrief, 0, len(ms))
	for _, m := range ms {
		out = append(out, domainsearch.MeetingBrief{
			Title:   m.Title,
			Date:    m.CreatedAt.UTC().Add(8 * time.Hour).Format("2006-01-02"),
			Summary: m.Summary,
		})
	}
	return out
}
