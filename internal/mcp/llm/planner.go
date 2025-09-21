// internal/mcp/llm/planner.go
package llm

import (
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// ToolLite: representasi tool dari schema (tanpa import mcp untuk hindari cycle)
type ToolLite struct {
	Name          string          `json:"name"`
	Description   string          `json:"description"`
	ParamsSchema  json.RawMessage `json:"params_schema,omitempty"` // JSON Schema utuh (untuk referensi LLM)
	Required      []string        `json:"required,omitempty"`
	ExampleParams map[string]any  `json:"example_params,omitempty"`
}

// NOTE: Interface Client sudah ada di openai.go (package llm).
// Di sini cukup menggunakannya saja tanpa mendeklar ulang.

// RoutePlanner bertumpu pada Client
type RoutePlanner struct{ client Client }

func NewRoutePlannerFromEnv() (*RoutePlanner, error) {
	c, err := NewFromEnv()
	if err != nil {
		return nil, err
	}
	return &RoutePlanner{client: c}, nil
}

// ====== Struktur schema JSON dasar ======

type schemaDoc struct {
	Title       string           `json:"title"`
	Description string           `json:"description"`
	Type        string           `json:"type"`
	Properties  map[string]any   `json:"properties"`
	Required    []string         `json:"required"`
	Examples    []map[string]any `json:"examples"`
}

// deriveToolName: "tool_get_po_status.schema.json" -> "get_po_status"
func deriveToolName(filename string) string {
	base := filepath.Base(filename)
	name := strings.TrimSuffix(base, ".schema.json")
	name = strings.TrimPrefix(name, "tool_")
	return name
}

// ===== Loader schema tools dari folder =====

func LoadToolsFromSchemaDir(dir string) ([]ToolLite, error) {
	var out []ToolLite

	walk := func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		if !strings.HasSuffix(d.Name(), ".schema.json") {
			return nil
		}
		if !strings.HasPrefix(d.Name(), "tool_") {
			return nil
		}

		b, err := os.ReadFile(path)
		if err != nil {
			log.Printf("[planner] skip schema read error: %s: %v", path, err)
			return nil
		}

		// Parse schema
		var sc schemaDoc
		if err := json.Unmarshal(b, &sc); err != nil {
			log.Printf("[planner] skip invalid schema: %s: %v", path, err)
			return nil // skip file ini
		}

		toolName := deriveToolName(path)
		desc := sc.Description
		if desc == "" {
			desc = sc.Title
		}

		// Ambil example params (kalau ada) untuk memandu LLM
		var ex map[string]any
		if len(sc.Examples) > 0 {
			ex = sc.Examples[0]
		}

		out = append(out, ToolLite{
			Name:          toolName,
			Description:   desc,
			ParamsSchema:  json.RawMessage(b),
			Required:      sc.Required,
			ExampleParams: ex,
		})
		return nil
	}

	if err := filepath.WalkDir(dir, walk); err != nil {
		return nil, err
	}

	// urutkan demi deterministik
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}

// ===== Loader tools dari manifest (api/mcp-tools.json) =====

type manifestEntry struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	InputSchema string `json:"input_schema"` // path ke schemas/mcp/*.schema.json
}

type manifestDoc struct {
	Tools []manifestEntry `json:"tools"`
}

// LoadToolsFromManifest membaca manifest (mis. "api/mcp-tools.json") lalu
// memuat schema yang dirujuk di setiap entry.
func LoadToolsFromManifest(manifestPath string) ([]ToolLite, error) {
	raw, err := os.ReadFile(manifestPath)
	if err != nil {
		return nil, fmt.Errorf("read manifest %s: %w", manifestPath, err)
	}

	var md manifestDoc
	if err := json.Unmarshal(raw, &md); err != nil {
		return nil, fmt.Errorf("unmarshal manifest %s: %w", manifestPath, err)
	}

	var out []ToolLite
	for _, ent := range md.Tools {
		if ent.Name == "" || ent.InputSchema == "" {
			log.Printf("[planner] skip manifest entry (missing name/input_schema): %+v", ent)
			continue
		}

		schemaBytes, err := os.ReadFile(ent.InputSchema)
		if err != nil {
			log.Printf("[planner] skip schema read: %s: %v", ent.InputSchema, err)
			continue
		}

		var sc schemaDoc
		if err := json.Unmarshal(schemaBytes, &sc); err != nil {
			log.Printf("[planner] skip invalid schema: %s: %v", ent.InputSchema, err)
			continue
		}

		desc := ent.Description
		if desc == "" {
			desc = sc.Description
			if desc == "" {
				desc = sc.Title
			}
		}

		var ex map[string]any
		if len(sc.Examples) > 0 {
			ex = sc.Examples[0]
		}

		out = append(out, ToolLite{
			Name:          ent.Name,
			Description:   desc,
			ParamsSchema:  json.RawMessage(schemaBytes),
			Required:      sc.Required,
			ExampleParams: ex,
		})
	}

	// urutkan demi deterministik
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}

// ====== Perencana ======

// PlanRawWithSchemas: ambil tools dari folder schema, lalu rencanakan rute dengan JSON mode
func (p *RoutePlanner) PlanRawWithSchemas(ctx context.Context, schemaDir string, question string) (string, error) {
	tools, err := LoadToolsFromSchemaDir(schemaDir)
	if err != nil {
		return "", fmt.Errorf("load tools from schema: %w", err)
	}
	return p.planRawInternal(ctx, tools, question)
}

// PlanRaw: kompatibilitas lama (kalau caller sudah menyiapkan tools)
func (p *RoutePlanner) PlanRaw(ctx context.Context, tools []ToolLite, question string) (string, error) {
	return p.planRawInternal(ctx, tools, question)
}

// planRawInternal: shared logic menyusun system prompt & call OpenAI JSON mode
func (p *RoutePlanner) planRawInternal(ctx context.Context, tools []ToolLite, question string) (string, error) {
	// ---- 1. Susun daftar tool untuk LLM ----
	type toolForLLM struct {
		Name        string                 `json:"name"`
		Description string                 `json:"description"`
		Fields      []string               `json:"fields,omitempty"`
		Required    []string               `json:"required,omitempty"`
		Example     map[string]any         `json:"example_params,omitempty"`
	}
	var llmTools []toolForLLM
	for _, t := range tools {
		var sc schemaDoc
		_ = json.Unmarshal(t.ParamsSchema, &sc)

		// ambil semua field dari schema.properties
		var fields []string
		for k := range sc.Properties {
			fields = append(fields, k)
		}
		sort.Strings(fields)

		llmTools = append(llmTools, toolForLLM{
			Name:        t.Name,
			Description: t.Description,
			Fields:      fields,
			Required:    sc.Required,
			Example:     t.ExampleParams,
		})
	}

	// ---- 2. Buat system prompt untuk ROUTER LLM ----
	sys := `Anda adalah ROUTER. Jawab HANYA dengan JSON VALID sesuai schema berikut.
Aturan:
- Pilih HANYA dari tools yang disediakan pada "tools".
- Jika pertanyaan jelas tentang Purchase Order (PO/vendor/ETA/amount/status), jangan pilih timeseries/production/drilling.
- Gunakan "rag" hanya bila tidak ada tool MCP yang cocok.
- Jangan mengarang nama tool atau field params yang tidak ada.
- Output HARUS valid object, TANPA teks lain, TANPA markdown.
- Jika pertanyaan menyebut tag/timeseries/signal (mis: OIL_*, GAS_*, *_D01) dan ada tanggal/range,
  PILIH tool "get_timeseries" (kind="mcp"), JANGAN pilih RAG.
  - Jika hanya satu tanggal (YYYY-MM-DD), gunakan:
    start_date = "YYYY-MM-DDT00:00:00Z"
    end_date   = "YYYY-MM-DDT23:59:59Z" (atau "YYYY-MM-DD+1 T00:00:00Z")
  - Sertakan: tag, start_date, end_date, opsional agg="raw".
- Jika pertanyaan tentang Purchase Order (PO/vendor/ETA/amount/status), pilih tool PO terkait. Jangan pilih timeseries.
- Gunakan "rag" hanya jika tidak ada tool MCP yang cocok.
- Output HARUS object JSON valid tanpa teks lain.
Skema keluaran:
{
  "mode": "mcp" | "rag" | "hybrid",
  "routes": [
    {
      "kind": "mcp" | "rag",
      "tool": "<nama tool jika kind=mcp>",
      "params": { },
      "query": "<string untuk rag>",
      "top_k": 10
    }
  ],
  "reason": "string singkat"
}`

	// ---- 3. Payload yang diberikan ke LLM ----
	payload := struct {
		Question string       `json:"question"`
		Tools    []toolForLLM `json:"tools"`
	}{
		Question: question,
		Tools:    llmTools,
	}
	ub, _ := json.Marshal(payload)

	// ---- 4. Timeout default (jika ctx belum punya deadline) ----
	if _, ok := ctx.Deadline(); !ok {
		var cancel context.CancelFunc
		// timeout 8 detik, karena jumlah schema bisa banyak
		ctx, cancel = context.WithTimeout(ctx, 8*time.Second)
		defer cancel()
	}

	// ---- 5. Panggil client AnswerJSON ----
	raw, err := p.client.AnswerJSON(ctx, string(ub), sys)
	if err != nil {
		return "", fmt.Errorf("planner AnswerJSON: %w", err)
	}
	return strings.TrimSpace(raw), nil
}
