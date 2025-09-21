// cmd/ingest-docs/main.go
// cmd/ingest-docs/main.go
package main

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	_ "github.com/go-sql-driver/mysql"
)

type embeddingReq struct {
	Model string   `json:"model"`
	Input []string `json:"input"`
}
type embeddingRes struct {
	Data []struct {
		Embedding []float32 `json:"embedding"`
	} `json:"data"`
}

func main() {
	var dsn, model, where string
	var batch int
	flag.StringVar(&dsn, "dsn", "mcpuser:secret@tcp(mysql:3306)/mcp?parseTime=true&multiStatements=true", "MySQL DSN")
	flag.StringVar(&model, "model", "text-embedding-3-small", "OpenAI embeddings model")
	flag.IntVar(&batch, "batch", 128, "batch size")
	flag.StringVar(&where, "where", "embedding IS NULL", "extra WHERE filter for selection")
	flag.Parse()

	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		fmt.Println("OPENAI_API_KEY empty")
		os.Exit(1)
	}

	db, err := sql.Open("mysql", dsn)
	must(err)
	defer db.Close()

	ctx := context.Background()
	for {
		rows, err := db.QueryContext(ctx, `
			SELECT id, COALESCE(title,''), COALESCE(url,''), COALESCE(snippet,'')
			FROM doc_chunks
			WHERE `+where+` 
			LIMIT ?`, batch)
		must(err)

		type rowT struct{ id int64; text string }
		batchRows := make([]rowT, 0, batch)
		for rows.Next() {
			var id int64
			var title, url, snip string
			must(rows.Scan(&id, &title, &url, &snip))
			txt := strings.TrimSpace(title+"\n"+snip+"\nURL: "+url)
			if len(txt) > 8000 { // jaga-jaga
				txt = txt[:8000]
			}
			batchRows = append(batchRows, rowT{id: id, text: txt})
		}
		rows.Close()

		if len(batchRows) == 0 {
			fmt.Println("done: no more rows")
			return
		}

		// call OpenAI
		inputs := make([]string, len(batchRows))
		for i, r := range batchRows {
			inputs[i] = r.text
		}
		embeds, err := fetchEmbeddings(ctx, apiKey, model, inputs)
		must(err)
		if len(embeds) != len(batchRows) {
			must(fmt.Errorf("embedding count mismatch %d != %d", len(embeds), len(batchRows)))
		}

		// save
		tx, err := db.BeginTx(ctx, nil); must(err)
		stmt, err := tx.PrepareContext(ctx, `UPDATE doc_chunks SET embedding = ? WHERE id = ?`); must(err)
		for i, r := range batchRows {
			js, _ := json.Marshal(embeds[i])
			if _, err := stmt.ExecContext(ctx, string(js), r.id); err != nil {
				_ = tx.Rollback(); must(err)
			}
		}
		stmt.Close()
		must(tx.Commit())
		fmt.Printf("ok: embedded %d rows\n", len(batchRows))

		// kecilkan tempo biar aman
		time.Sleep(300 * time.Millisecond)
	}
}

func fetchEmbeddings(ctx context.Context, apiKey, model string, input []string) ([][]float32, error) {
	reqBody, _ := json.Marshal(embeddingReq{Model: model, Input: input})
	req, _ := http.NewRequestWithContext(ctx, "POST", "https://api.openai.com/v1/embeddings", bytes.NewReader(reqBody))
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode/100 != 2 {
		b, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("openai error: %s", string(b))
	}
	var out embeddingRes
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, err
	}
	vecs := make([][]float32, len(out.Data))
	for i := range out.Data {
		vecs[i] = out.Data[i].Embedding
	}
	return vecs, nil
}

func must(err error) {
	if err != nil {
		fmt.Fprintln(os.Stderr, "ERR:", err)
		os.Exit(1)
	}
}
