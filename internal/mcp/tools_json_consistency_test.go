// internal/mcp/tools_json_consistency_test.go

package mcp_test

import (
	"testing"

	"mcp-oilgas/internal/mcp"
)

// Pastikan semua tool di api/mcp-tools.json SUDAH diregister.
// (Boleh ada tool terdaftar yang tidak tercantum di JSON; fokus kita adalah file JSON tidak menyebut tool yang belum ada.)
func TestToolsJsonOnlyContainsRegisteredTools(t *testing.T) {
	defs, err := mcp.LoadToolDefs()
	if err != nil {
		t.Fatalf("LoadToolDefs error: %v", err)
	}
	if len(defs) == 0 {
		t.Fatalf("no tools found in api/mcp-tools.json")
	}

	// daftar nama tool yang terdaftar di registry
	reg := map[string]struct{}{}
	for _, name := range mcp.List() {
		reg[name] = struct{}{}
	}

	for _, d := range defs {
		if _, ok := reg[d.Name]; !ok {
			t.Fatalf("tool %q exists in api/mcp-tools.json but NOT registered in MCP registry", d.Name)
		}
	}
}
