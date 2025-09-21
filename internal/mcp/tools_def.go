// internal/mcp/tools_def.go
// internal/mcp/tools_def.go
// internal/mcp/tools_def.go
package mcp

import (
	_ "embed"
	"encoding/json"
	"sync"
)

// letakkan file di paket ini (tanpa "..")
//go:embed mcp-tools.json
var toolsJSON []byte

type ToolDef struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	InputSchema string `json:"input_schema"`
}
type ToolCatalog struct{ Tools []ToolDef `json:"tools"` }

var (
	toolDefs     []ToolDef
	toolDefsOnce sync.Once
	toolDefsErr  error
)

func LoadToolDefs() ([]ToolDef, error) {
	toolDefsOnce.Do(func() {
		var cat ToolCatalog
		if err := json.Unmarshal(toolsJSON, &cat); err != nil {
			toolDefsErr = err
			return
		}
		toolDefs = cat.Tools
	})
	return toolDefs, toolDefsErr
}
