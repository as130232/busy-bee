package db

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	pgv "github.com/pgvector/pgvector-go"

	"github.com/as130232/busy-bee/busy-bee-be/domain/search"
)

// ChunkRepo 以手寫 pgx + pgvector-go 實作 search.ChunkRepository。
// （pgvector 型別 sqlc 支援不完整，故不走 sqlc。）
type ChunkRepo struct {
	pool *pgxpool.Pool
}

func NewChunkRepo(pool *pgxpool.Pool) *ChunkRepo { return &ChunkRepo{pool: pool} }

var _ search.ChunkRepository = (*ChunkRepo)(nil)

// Upsert 冪等：先刪該會議舊 chunks 再批次插入（同一 tx）。
func (r *ChunkRepo) Upsert(ctx context.Context, chunks []search.Chunk) error {
	if len(chunks) == 0 {
		return nil
	}
	meetingID := chunks[0].MeetingID
	return pgx.BeginFunc(ctx, r.pool, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, `DELETE FROM transcript_chunks WHERE meeting_id = $1`, meetingID); err != nil {
			return fmt.Errorf("db.chunk delete: %w", err)
		}
		for _, c := range chunks {
			_, err := tx.Exec(ctx,
				`INSERT INTO transcript_chunks (meeting_id, user_id, chunk_index, content, embedding)
				 VALUES ($1,$2,$3,$4,$5)`,
				c.MeetingID, c.UserID, c.ChunkIndex, c.Content, pgv.NewVector(c.Embedding))
			if err != nil {
				return fmt.Errorf("db.chunk insert: %w", err)
			}
		}
		return nil
	})
}

func (r *ChunkRepo) DeleteByMeeting(ctx context.Context, meetingID uuid.UUID) error {
	if _, err := r.pool.Exec(ctx, `DELETE FROM transcript_chunks WHERE meeting_id = $1`, meetingID); err != nil {
		return fmt.Errorf("db.chunk deleteByMeeting: %w", err)
	}
	return nil
}

// SearchSimilar 每會議取最近的一塊（DISTINCT ON），cosine 距離排序，owner 過濾。
func (r *ChunkRepo) SearchSimilar(ctx context.Context, userID uuid.UUID, vec []float32, topK int) ([]search.SearchResult, error) {
	v := pgv.NewVector(vec)
	rows, err := r.pool.Query(ctx, `
		SELECT meeting_id, content, 1 - (embedding <=> $2) AS score
		FROM (
			SELECT DISTINCT ON (meeting_id) meeting_id, content, embedding
			FROM transcript_chunks
			WHERE user_id = $1
			ORDER BY meeting_id, embedding <=> $2
		) t
		ORDER BY embedding <=> $2
		LIMIT $3`,
		userID, v, topK)
	if err != nil {
		return nil, fmt.Errorf("db.chunk search: %w", err)
	}
	defer rows.Close()
	var out []search.SearchResult
	for rows.Next() {
		var res search.SearchResult
		if err := rows.Scan(&res.MeetingID, &res.Snippet, &res.Score); err != nil {
			return nil, fmt.Errorf("db.chunk scan: %w", err)
		}
		res.MatchType = search.MatchSemantic
		out = append(out, res)
	}
	return out, rows.Err()
}

// ExistingEmbeddings 回傳該會議現有 chunks 的 content → embedding 映射（重新索引複用未變動片段）。
func (r *ChunkRepo) ExistingEmbeddings(ctx context.Context, meetingID uuid.UUID) (map[string][]float32, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT content, embedding FROM transcript_chunks WHERE meeting_id = $1`, meetingID)
	if err != nil {
		return nil, fmt.Errorf("db.chunk existing: %w", err)
	}
	defer rows.Close()
	out := map[string][]float32{}
	for rows.Next() {
		var content string
		var vec pgv.Vector
		if err := rows.Scan(&content, &vec); err != nil {
			return nil, fmt.Errorf("db.chunk existing scan: %w", err)
		}
		out[content] = vec.Slice()
	}
	return out, rows.Err()
}

// MeetingsWithoutChunks 已 completed 且有逐字稿但尚無 chunks 的會議（回填掃描用）。
func (r *ChunkRepo) MeetingsWithoutChunks(ctx context.Context) ([]uuid.UUID, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT m.id FROM meetings m
		WHERE m.status = 'completed' AND m.transcript <> ''
		  AND NOT EXISTS (SELECT 1 FROM transcript_chunks c WHERE c.meeting_id = m.id)`)
	if err != nil {
		return nil, fmt.Errorf("db.chunk missing: %w", err)
	}
	defer rows.Close()
	var ids []uuid.UUID
	for rows.Next() {
		var id uuid.UUID
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("db.chunk missing scan: %w", err)
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}
