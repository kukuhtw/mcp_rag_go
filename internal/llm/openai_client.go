// [FILE] internal/mcp/llm/openai.go
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

// ======================
// Interface (kontrak umum)
// ======================
type Client interface {
	// Jawaban naratif (non-JSON, non-stream)
	AnswerWithRAG(ctx context.Context, system, prompt string) (string, error)

	// Jawaban dalam format JSON valid
	AnswerJSON(ctx context.Context, user, system string) (string, error)

	// Streaming delta token untuk SSE
	AnswerStream(ctx context.Context, system, prompt string, onDelta func(delta string) error) (string, error)

	// Streaming sederhana (tanpa system role)
	StreamChat(ctx context.Context, prompt string, onToken func(token string) error) error

	// Ambil nama model aktif
	Model() string
}

// ======================
// Implementasi OpenAIClient
// ======================
type OpenAIClient struct {
	api   *openai.Client
	model string
}

// NewFromEnv membaca API key dan model dari environment
// - OPENAI_API_KEY (wajib)
// - OPENAI_MODEL (opsional, default gpt-4o-mini)
// - OPENAI_BASE_URL (opsional, untuk proxy/self-hosted endpoint)
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
		model = "gpt-4o-mini"
	}

	return &OpenAIClient{
		api:   openai.NewClientWithConfig(cfg),
		model: model,
	}, nil
}

func (c *OpenAIClient) Model() string { return c.model }

// ======================
// Implementasi method
// ======================

// Jawaban naratif
func (c *OpenAIClient) AnswerWithRAG(ctx context.Context, system, prompt string) (string, error) {
	req := openai.ChatCompletionRequest{
		Model: c.model,
		Messages: []openai.ChatCompletionMessage{
			{Role: openai.ChatMessageRoleSystem, Content: system},
			{Role: openai.ChatMessageRoleUser, Content: prompt},
		},
		Temperature: 0.2,
	}

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
	return strings.TrimSpace(resp.Choices[0].Message.Content), nil
}

// Jawaban JSON
func (c *OpenAIClient) AnswerJSON(ctx context.Context, user, system string) (string, error) {
	req := openai.ChatCompletionRequest{
		Model: c.model,
		Messages: []openai.ChatCompletionMessage{
			{Role: openai.ChatMessageRoleSystem, Content: system},
			{Role: openai.ChatMessageRoleUser, Content: user},
		},
		Temperature: 0.0,
		ResponseFormat: &openai.ChatCompletionResponseFormat{
			Type: openai.ChatCompletionResponseFormatTypeJSONObject,
		},
	}

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
	return strings.TrimSpace(out), nil
}

// Streaming dengan system + user
func (c *OpenAIClient) AnswerStream(ctx context.Context, system, prompt string, onDelta func(delta string) error) (string, error) {
	req := openai.ChatCompletionRequest{
		Model: c.model,
		Messages: []openai.ChatCompletionMessage{
			{Role: openai.ChatMessageRoleSystem, Content: system},
			{Role: openai.ChatMessageRoleUser, Content: prompt},
		},
		Temperature: 0.2,
		Stream:      true,
	}

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
		if errors.Is(err, openai.ErrCompletionStreamEOF) {
			break
		}
		if err != nil {
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

// Streaming sederhana (hanya user role, mirip versi PoC lama)
func (c *OpenAIClient) StreamChat(ctx context.Context, prompt string, onToken func(token string) error) error {
	req := openai.ChatCompletionRequest{
		Model: c.model,
		Messages: []openai.ChatCompletionMessage{
			{Role: openai.ChatMessageRoleUser, Content: prompt},
		},
		Stream: true,
	}
	stream, err := c.api.CreateChatCompletionStream(ctx, req)
	if err != nil {
		return err
	}
	defer stream.Close()

	for {
		resp, err := stream.Recv()
		if errors.Is(err, openai.ErrCompletionStreamEOF) {
			return nil
		}
		if err != nil {
			return err
		}
		for _, ch := range resp.Choices {
			frag := ch.Delta.Content
			if frag != "" {
				if err := onToken(frag); err != nil {
					return err
				}
			}
		}
	}
}
