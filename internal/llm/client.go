package llm

import (
	"context"
	"fmt"
	"strings"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"
)

type Client struct {
	api   anthropic.Client
	model anthropic.Model
}

func New(apiKey, model string) *Client {
	return &Client{
		api:   anthropic.NewClient(option.WithAPIKey(apiKey)),
		model: anthropic.Model(model),
	}
}

type ChatMessage struct {
	Role    string
	Content string
}

const maxChatTokens = 1024

func (c *Client) StreamChat(ctx context.Context, history []ChatMessage, contextChunks []string, onDelta func(string) error) (string, error) {
	params := anthropic.MessageNewParams{
		Model:     c.model,
		MaxTokens: maxChatTokens,
		Messages:  toMessageParams(history),
	}
	if system := buildSystemPrompt(contextChunks); system != "" {
		params.System = []anthropic.TextBlockParam{{Text: system}}
	}

	stream := c.api.Messages.NewStreaming(ctx, params)

	var full strings.Builder
	for stream.Next() {
		event := stream.Current()
		if delta, ok := event.AsAny().(anthropic.ContentBlockDeltaEvent); ok {
			if text := delta.Delta.Text; text != "" {
				full.WriteString(text)
				if err := onDelta(text); err != nil {
					return full.String(), err
				}
			}
		}
	}
	if err := stream.Err(); err != nil {
		return full.String(), fmt.Errorf("streaming chat: %w", err)
	}

	return full.String(), nil
}

func toMessageParams(history []ChatMessage) []anthropic.MessageParam {
	params := make([]anthropic.MessageParam, 0, len(history))
	for _, m := range history {
		block := anthropic.NewTextBlock(m.Content)
		if m.Role == "assistant" {
			params = append(params, anthropic.NewAssistantMessage(block))
		} else {
			params = append(params, anthropic.NewUserMessage(block))
		}
	}
	return params
}

func buildSystemPrompt(chunks []string) string {
	if len(chunks) == 0 {
		return ""
	}
	var sb strings.Builder
	sb.WriteString("You are a helpful assistant. Use the following reference material to ground your answers when relevant. If the material does not cover the question, answer from your own knowledge.\n")
	for _, chunk := range chunks {
		sb.WriteString("---\n")
		sb.WriteString(chunk)
		sb.WriteString("\n")
	}
	return sb.String()
}
