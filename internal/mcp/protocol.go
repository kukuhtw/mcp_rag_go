// mcp/protocol.go
// Definisi struktur dasar MCP protocol

package mcp

type ToolRequest struct {
    Tool   string      `json:"tool"`
    Params interface{} `json:"params"`
}

type ToolResponse struct {
	Success bool        `json:"success"`
	Data    interface{} `json:"data,omitempty"`
	Error   string      `json:"error,omitempty"`
}
