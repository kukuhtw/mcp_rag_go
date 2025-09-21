// internal/repositories/search/rag_repo.go
// internal/repositories/search/rag_repo.go
package search

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"math"
	"strings"
	"time"

	"mcp-oilgas/pkg/vector" // ganti dari your/module → ke module kamu
)

type RAGHit struct {
	DocID   string
	Title   string
	URL     string
	Snippet string
	Page    int
	Score   float64
}

type RAGRepo interface {
	Retrieve(ctx context.Context, query string, topK int) ([]RAGHit, error)
}

type ragRepo struct {
	db           *sql.DB
	embedClient  *vector.OpenAIClient
	embedModel   string
	prefilterTop int
}

func NewRAGRepo(db *sql.DB, embedClient *vector.OpenAIClient, embedModel string, prefilterTop int) RAGRepo {
	if prefilterTop <= 0 {
		prefilterTop = 100
	}
	if embedModel == "" {
		embedModel = "text-embedding-3-small"
	}
	return &ragRepo{db: db, embedClient: embedClient, embedModel: embedModel, prefilterTop: prefilterTop}
}

type chunkRow struct {
	DocID   string
	Title   string
	URL     string
	PageNo  int
	Snippet string
	EmbJSON string
}

// 🔑 pindahkan ke sini biar bisa dipakai di partialSortTopK
type scored struct {
	h RAGHit
}

func (r *ragRepo) Retrieve(ctx context.Context, query string, topK int) ([]RAGHit, error) {
	q := strings.TrimSpace(query)
	if q == "" {
		return nil, errors.New("empty query")
	}
	if topK <= 0 || topK > 50 {
		topK = 10
	}

	embs, err := r.embedClient.CreateEmbeddings(ctx, r.embedModel, []string{q})
	if err != nil {
		return nil, err
	}
	if len(embs) == 0 {
		return nil, errors.New("no embedding for query")
	}
	qv := embs[0]
	norm(qv)

	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	rows, err := r.db.QueryContext(ctx, `
		SELECT doc_id, title, url, page_no, snippet, JSON_EXTRACT(embedding, '$') AS emb
		FROM doc_chunks
		WHERE MATCH(title, snippet) AGAINST (? IN NATURAL LANGUAGE MODE)
		ORDER BY MATCH(title, snippet) AGAINST (? IN NATURAL LANGUAGE MODE) DESC
		LIMIT ?`,
		q, q, r.prefilterTop,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var cands []chunkRow
	for rows.Next() {
		var cr chunkRow
		if err := rows.Scan(&cr.DocID, &cr.Title, &cr.URL, &cr.PageNo, &cr.Snippet, &cr.EmbJSON); err != nil {
			return nil, err
		}
		cands = append(cands, cr)
	}
	_ = rows.Close()

	scoredHits := make([]scored, 0, len(cands))
	for _, c := range cands {
		var ev []float64
		if err := json.Unmarshal([]byte(c.EmbJSON), &ev); err != nil || len(ev) == 0 {
			continue
		}
		norm(ev)
		s := dot(qv, ev)
		scoredHits = append(scoredHits, scored{
			h: RAGHit{
				DocID:   c.DocID,
				Title:   c.Title,
				URL:     c.URL,
				Snippet: c.Snippet,
				Page:    c.PageNo,
				Score:   s,
			},
		})
	}

	partialSortTopK(scoredHits, topK)

	out := make([]RAGHit, 0, min(topK, len(scoredHits)))
	for i := 0; i < len(scoredHits) && i < topK; i++ {
		out = append(out, scoredHits[i].h)
	}
	return out, nil
}

func norm(v []float64) {
	var s float64
	for _, x := range v {
		s += x * x
	}
	if s == 0 {
		return
	}
	inv := 1.0 / math.Sqrt(s)
	for i := range v {
		v[i] *= inv
	}
}

func dot(a, b []float64) float64 {
	n := len(a)
	if len(b) < n {
		n = len(b)
	}
	var s float64
	for i := 0; i < n; i++ {
		s += a[i] * b[i]
	}
	return s
}

func partialSortTopK(arr []scored, k int) {
	for i := 0; i < len(arr); i++ {
		maxIdx := i
		for j := i + 1; j < len(arr); j++ {
			if arr[j].h.Score > arr[maxIdx].h.Score {
				maxIdx = j
			}
		}
		arr[i], arr[maxIdx] = arr[maxIdx], arr[i]
		if i+1 >= k {
			break
		}
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
