// internal/mcp/llm/openai.go
package llm

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	openai "github.com/sashabaranov/go-openai"
)

// Client adalah kontrak minimal yang dipakai layer lain (planner/synthesizer/SSE).
type Client interface {
	// Jawaban naratif biasa (bebas format) — dipakai untuk tahap sintesis jawaban akhir.
	AnswerWithRAG(ctx context.Context, system, prompt string) (string, error)
	// Jawaban dalam format JSON object valid — dipakai oleh Planner agar output bisa di-unmarshal.
	AnswerJSON(ctx context.Context, user, system string) (string, error)
	// Streaming delta token untuk SSE — dipakai saat menyusun jawaban akhir secara bertahap.
	AnswerStream(ctx context.Context, system, prompt string, onDelta func(delta string) error) (string, error)
}

// OpenAIClient adalah implementasi Client berbasis go-openai.
type OpenAIClient struct {
	api   *openai.Client
	model string
}

// NewFromEnv mengembalikan OpenAI Client berbasis env var.
// Env minimal: OPENAI_API_KEY
// Opsional:    OPENAI_MODEL (default: gpt-4o-mini), OPENAI_BASE_URL (untuk proxy/self-hosted endpoint)
func NewFromEnv() (Client, error) {
	key := strings.TrimSpace(os.Getenv("OPENAI_API_KEY"))
	if key == "" {
		return nil, errors.New("OPENAI_API_KEY not set")
	}

	cfg := openai.DefaultConfig(key)
	if base := strings.TrimSpace(os.Getenv("OPENAI_BASE_URL")); base != "" {
		cfg.BaseURL = base
	}

	model := strings.TrimSpace(os.Getenv("OPENAI_MODEL"))
	if model == "" {
		model = "gpt-4o-mini" // default ringan, mendukung JSON mode & streaming
	}

	return &OpenAIClient{
		api:   openai.NewClientWithConfig(cfg),
		model: model,
	}, nil
}

// AnswerWithRAG meminta model menghasilkan jawaban naratif/final untuk user.
func (c *OpenAIClient) AnswerWithRAG(ctx context.Context, system, prompt string) (string, error) {
	req := openai.ChatCompletionRequest{
		Model: c.model,
		Messages: []openai.ChatCompletionMessage{
			{Role: openai.ChatMessageRoleSystem, Content: system},
			{Role: openai.ChatMessageRoleUser, Content: prompt},
		},
		Temperature: 0.2,
	}

	// Deadline defensif agar responsif
	var cancel context.CancelFunc
	if _, ok := ctx.Deadline(); !ok {
		ctx, cancel = context.WithTimeout(ctx, 18*time.Second)
		defer cancel()
	}

	resp, err := c.api.CreateChatCompletion(ctx, req)
	if err != nil {
		return "", fmt.Errorf("openai completion: %w", err)
	}
	if len(resp.Choices) == 0 {
		return "", errors.New("no completion choices")
	}
	out := strings.TrimSpace(resp.Choices[0].Message.Content)
	return out, nil
}

// AnswerJSON meminta model merespons JSON object valid (JSON mode).
func (c *OpenAIClient) AnswerJSON(ctx context.Context, user, system string) (string, error) {
	req := openai.ChatCompletionRequest{
		Model: c.model,
		Messages: []openai.ChatCompletionMessage{
			{Role: openai.ChatMessageRoleSystem, Content: system},
			{Role: openai.ChatMessageRoleUser, Content: user},
		},
		Temperature: 0.0,
		ResponseFormat: &openai.ChatCompletionResponseFormat{
			Type: openai.ChatCompletionResponseFormatTypeJSONObject, // jika SDK lama: Type: "json_object"
		},
	}

	// Deadline defensif lebih singkat untuk tahap routing/planning
	var cancel context.CancelFunc
	if _, ok := ctx.Deadline(); !ok {
		ctx, cancel = context.WithTimeout(ctx, 8*time.Second)
		defer cancel()
	}

	resp, err := c.api.CreateChatCompletion(ctx, req)
	if err != nil {
		return "", fmt.Errorf("openai completion (json): %w", err)
	}
	if len(resp.Choices) == 0 {
		return "", errors.New("no completion choices")
	}
	out := strings.TrimSpace(resp.Choices[0].Message.Content)

	// Bersihkan bila model menyelipkan ```json ... ```
	out = strings.TrimPrefix(out, "```json")
	out = strings.TrimPrefix(out, "```JSON")
	out = strings.TrimPrefix(out, "```")
	out = strings.TrimSuffix(out, "```")
	out = strings.TrimSpace(out)

	return out, nil
}

// AnswerStream melakukan chat completion secara streaming (token-by-token).
func (c *OpenAIClient) AnswerStream(ctx context.Context, system, prompt string, onDelta func(delta string) error) (string, error) {
	req := openai.ChatCompletionRequest{
		Model: c.model,
		Messages: []openai.ChatCompletionMessage{
			{Role: openai.ChatMessageRoleSystem, Content: system},
			{Role: openai.ChatMessageRoleUser, Content: prompt},
		},
		Temperature: 0.2,
		Stream:      true, // penting
	}

	// Deadline defensif
	var cancel context.CancelFunc
	if _, ok := ctx.Deadline(); !ok {
		ctx, cancel = context.WithTimeout(ctx, 60*time.Second)
		defer cancel()
	}

	stream, err := c.api.CreateChatCompletionStream(ctx, req)
	if err != nil {
		return "", fmt.Errorf("openai stream init: %w", err)
	}
	defer stream.Close()

	var final strings.Builder
	for {
		resp, err := stream.Recv()
		if err != nil {
			// io.EOF = selesai normal; SDK mengirim err.Error()=="EOF"
			if err.Error() == "EOF" {
				break
			}
			return final.String(), fmt.Errorf("openai stream recv: %w", err)
		}
		for _, ch := range resp.Choices {
			delta := ch.Delta.Content
			if delta == "" {
				continue
			}
			final.WriteString(delta)
			if onDelta != nil {
				if derr := onDelta(delta); derr != nil {
					return final.String(), derr
				}
			}
		}
	}
	return final.String(), nil
}
