// internal/repositories/mysql/rag_repo.go
package mysql

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"math"
	"sort"
	"strings"
)

type Chunk struct {
	ID      int64
	DocID   sql.NullString
	Title   sql.NullString
	URL     sql.NullString
	Snippet sql.NullString
	PageNo  sql.NullInt64
	Score   sql.NullFloat64 // BM25 atau skor final hybrid
}

type RAGRepo struct {
	DB *sql.DB
}

// -------- BM25 (FULLTEXT) --------

func (r *RAGRepo) SearchBM25(ctx context.Context, query string, topK int) ([]Chunk, error) {
	if r == nil || r.DB == nil {
		return nil, errors.New("rag repo: DB is nil")
	}
	query = strings.TrimSpace(query)
	if query == "" {
		return nil, errors.New("rag repo: empty query")
	}
	if topK <= 0 || topK > 100 {
		topK = 10
	}

	const q = `
		SELECT id, doc_id, title, url, snippet, page_no,
		       MATCH(title, snippet) AGAINST (? IN NATURAL LANGUAGE MODE) AS score
		  FROM doc_chunks
		 WHERE MATCH(title, snippet) AGAINST (? IN NATURAL LANGUAGE MODE)
		 ORDER BY score DESC
		 LIMIT ?;
	`
	rows, err := r.DB.QueryContext(ctx, q, query, query, topK*5) // ambil lebih banyak utk hybrid
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []Chunk
	for rows.Next() {
		var c Chunk
		if err := rows.Scan(&c.ID, &c.DocID, &c.Title, &c.URL, &c.Snippet, &c.PageNo, &c.Score); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// -------- Embedding utils --------

func cosine(a, b []float32) float64 {
	if len(a) == 0 || len(b) == 0 || len(a) != len(b) {
		return 0
	}
	var dot, na, nb float64
	for i := 0; i < len(a); i++ {
		af := float64(a[i])
		bf := float64(b[i])
		dot += af * bf
		na += af * af
		nb += bf * bf
	}
	den := math.Sqrt(na) * math.Sqrt(nb)
	if den == 0 {
		return 0
	}
	return dot / den
}

func parseEmbeddingJSON(raw []byte) ([]float32, error) {
	var f64 []float64
	if err := json.Unmarshal(raw, &f64); err != nil {
		return nil, err
	}
	out := make([]float32, len(f64))
	for i, v := range f64 {
		out[i] = float32(v)
	}
	return out, nil
}

func (r *RAGRepo) loadEmbeddings(ctx context.Context, ids []int64) (map[int64][]float32, error) {
	if len(ids) == 0 {
		return map[int64][]float32{}, nil
	}
	var sb strings.Builder
	args := make([]any, 0, len(ids))
	sb.WriteString(`SELECT id, embedding FROM doc_chunks WHERE id IN (`)
	for i, id := range ids {
		if i > 0 {
			sb.WriteString(",")
		}
		sb.WriteString("?")
		args = append(args, id)
	}
	sb.WriteString(")")
	q := sb.String()

	rows, err := r.DB.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	m := make(map[int64][]float32, len(ids))
	for rows.Next() {
		var id int64
		var raw []byte // kolom JSON -> []byte
		if err := rows.Scan(&id, &raw); err != nil {
			return nil, err
		}
		if len(raw) == 0 {
			continue
		}
		vec, err := parseEmbeddingJSON(raw)
		if err != nil {
			continue
		}
		m[id] = vec
	}
	return m, rows.Err()
}

// -------- Hybrid (BM25 + cosine) --------

func (r *RAGRepo) SearchHybrid(ctx context.Context, query string, queryEmbedding []float32, alpha float64, topK int) ([]Chunk, error) {
	if r == nil || r.DB == nil {
		return nil, errors.New("rag repo: DB is nil")
	}
	if topK <= 0 || topK > 100 {
		topK = 10
	}
	if alpha < 0 || alpha > 1 {
		alpha = 0.5
	}

	var bm25Results []Chunk
	var err error
	if strings.TrimSpace(query) != "" {
		bm25Results, err = r.SearchBM25(ctx, query, topK)
		if err != nil {
			return nil, err
		}
	}

	candidateIDs := make([]int64, 0, len(bm25Results))
	idSet := map[int64]struct{}{}
	for _, c := range bm25Results {
		if _, ok := idSet[c.ID]; !ok {
			idSet[c.ID] = struct{}{}
			candidateIDs = append(candidateIDs, c.ID)
		}
	}

	// Kalau tidak ada teks, ambil sample dokumen ber-embedding
	if len(candidateIDs) == 0 && len(queryEmbedding) > 0 {
		rows, err := r.DB.QueryContext(ctx, `
			SELECT id FROM doc_chunks
			 WHERE embedding IS NOT NULL
			 ORDER BY id DESC
			 LIMIT 200
		`)
		if err != nil {
			return nil, err
		}
		for rows.Next() {
			var id int64
			if err := rows.Scan(&id); err != nil {
				rows.Close()
				return nil, err
			}
			if _, ok := idSet[id]; !ok {
				idSet[id] = struct{}{}
				candidateIDs = append(candidateIDs, id)
			}
		}
		rows.Close()
	}

	meta := map[int64]Chunk{}
	for _, c := range bm25Results {
		meta[c.ID] = c
	}

	// Lengkapi meta untuk kandidat lain
	if len(candidateIDs) > 0 {
		var sb strings.Builder
		args := make([]any, 0, len(candidateIDs))
		sb.WriteString(`SELECT id, doc_id, title, url, snippet, page_no FROM doc_chunks WHERE id IN (`)
		for i, id := range candidateIDs {
			if i > 0 {
				sb.WriteString(",")
			}
			sb.WriteString("?")
			args = append(args, id)
		}
		sb.WriteString(")")
		q := sb.String()

		rows, err := r.DB.QueryContext(ctx, q, args...)
		if err != nil {
			return nil, err
		}
		for rows.Next() {
			var c Chunk
			if err := rows.Scan(&c.ID, &c.DocID, &c.Title, &c.URL, &c.Snippet, &c.PageNo); err != nil {
				rows.Close()
				return nil, err
			}
			if _, exists := meta[c.ID]; !exists {
				meta[c.ID] = c
			}
		}
		rows.Close()
	}

	embMap, err := r.loadEmbeddings(ctx, candidateIDs)
	if err != nil {
		return nil, err
	}

	// Normalisasi BM25 ke [0,1]
	var maxBM25 float64
	for _, c := range meta {
		if c.Score.Valid && c.Score.Float64 > maxBM25 {
			maxBM25 = c.Score.Float64
		}
	}

	type scored struct {
		Chunk
		Final float64
	}
	out := make([]scored, 0, len(meta))
	for id, c := range meta {
		bm25Norm := 0.0
		if maxBM25 > 0 && c.Score.Valid {
			bm25Norm = c.Score.Float64 / maxBM25
			if bm25Norm > 1 {
				bm25Norm = 1
			}
		}
		cos := 0.0
		if len(queryEmbedding) > 0 {
			if vec, ok := embMap[id]; ok {
				cos = cosine(queryEmbedding, vec)
				if cos < -1 {
					cos = -1
				}
				if cos > 1 {
					cos = 1
				}
				cos = (cos + 1) / 2 // ke [0,1]
			}
		}
		final := alpha*bm25Norm + (1-alpha)*cos
		out = append(out, scored{Chunk: c, Final: final})
	}

	sort.Slice(out, func(i, j int) bool { return out[i].Final > out[j].Final })
	if len(out) > topK {
		out = out[:topK]
	}

	res := make([]Chunk, 0, len(out))
	for _, s := range out {
		c := s.Chunk
		c.Score.Valid = true
		c.Score.Float64 = s.Final
		res = append(res, c)
	}
	return res, nil
}
