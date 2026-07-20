package search

import (
	"context"
	"fmt"
	"log/slog"
	"sort"

	"github.com/google/uuid"

	domainmeeting "github.com/as130232/busy-bee/busy-bee-be/domain/meeting"
	domainsearch "github.com/as130232/busy-bee/busy-bee-be/domain/search"
)

const (
	searchTopK   = 10
	literalBoost = 0.5 // 字面命中的固定分（與 cosine 0~1 同量級）
)

// literalSearcher 字面搜尋來源（*db.MeetingRepo.ListForUser 滿足）。
type literalSearcher interface {
	ListForUser(ctx context.Context, userID uuid.UUID, search string) ([]domainmeeting.Meeting, error)
}

// ownerGetter owner 過濾取單一會議（*db.MeetingRepo.GetForUser 滿足），
// 供補齊只在語意命中、不在字面清單的會議。
type ownerGetter interface {
	GetForUser(ctx context.Context, id, userID uuid.UUID) (domainmeeting.Meeting, error)
}

// SearchUC 混合搜尋：字面（ILIKE）+ 語意（向量 top-K），合併去重排序。
type SearchUC struct {
	literal  literalSearcher
	embedder domainsearch.Embedder
	chunks   domainsearch.ChunkRepository
	owner    ownerGetter
}

func NewSearchUC(literal literalSearcher, embedder domainsearch.Embedder, chunks domainsearch.ChunkRepository, owner ownerGetter) *SearchUC {
	return &SearchUC{literal: literal, embedder: embedder, chunks: chunks, owner: owner}
}

// Execute 並行字面 + 語意，合併去重排序。查詢 embedding 失敗降級純字面。
// 回傳排序後會議，與每會議命中片段（semantic 才有 snippet）。
func (uc *SearchUC) Execute(ctx context.Context, userID uuid.UUID, query string) ([]domainmeeting.Meeting, map[uuid.UUID]domainsearch.SearchResult, error) {
	litMeetings, err := uc.literal.ListForUser(ctx, userID, query)
	if err != nil {
		return nil, nil, fmt.Errorf("search literal: %w", err)
	}

	scores := map[uuid.UUID]float64{}
	hits := map[uuid.UUID]domainsearch.SearchResult{}
	byID := map[uuid.UUID]domainmeeting.Meeting{}
	for _, m := range litMeetings {
		byID[m.ID] = m
		scores[m.ID] += literalBoost
	}

	// 語意：embedding 或向量檢索失敗則降級（只用字面結果，記 log 不記內容）
	if vec, eerr := uc.embedder.Embed(ctx, query); eerr != nil {
		slog.WarnContext(ctx, "search.semantic_degraded", "stage", "embed", "err", eerr)
	} else if sem, serr := uc.chunks.SearchSimilar(ctx, userID, vec, searchTopK); serr != nil {
		slog.WarnContext(ctx, "search.semantic_degraded", "stage", "vector", "err", serr)
	} else {
		for _, r := range sem {
			scores[r.MeetingID] += r.Score
			hits[r.MeetingID] = r // semantic snippet 優先
		}
	}

	// 補齊只在語意命中、不在字面清單的會議（owner 過濾取單筆）
	for id := range scores {
		if _, ok := byID[id]; ok {
			continue
		}
		if m, gerr := uc.owner.GetForUser(ctx, id, userID); gerr == nil {
			byID[id] = m
		} else {
			delete(scores, id) // 取不到（理論上不會，owner 已過濾）就丟棄
			delete(hits, id)
		}
	}

	// 依分數排序
	ids := make([]uuid.UUID, 0, len(scores))
	for id := range scores {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool { return scores[ids[i]] > scores[ids[j]] })

	out := make([]domainmeeting.Meeting, 0, len(ids))
	for _, id := range ids {
		out = append(out, byID[id])
	}
	return out, hits, nil
}
