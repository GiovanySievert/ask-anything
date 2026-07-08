package llm

import (
	"context"
	"encoding/json"
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

type Evaluation struct {
	Score         int      `json:"score"`
	Feedback      string   `json:"feedback"`
	MissingPoints []string `json:"missing_points"`
	WeakTopics    []string `json:"weak_topics"`
	NextQuestion  string   `json:"next_question"`
}

func (c *Client) GenerateQuestion(ctx context.Context, topic, level string, contextChunks []string) (string, error) {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("You are a technical interviewer. Generate ONE interview question for the topic %q at %q level.\n", topic, level))
	sb.WriteString("Return only the question text, with no preamble or numbering.\n")
	if len(contextChunks) > 0 {
		sb.WriteString("\nUse the following reference material to ground the question:\n")
		for _, chunk := range contextChunks {
			sb.WriteString("---\n")
			sb.WriteString(chunk)
			sb.WriteString("\n")
		}
	}

	text, err := c.complete(ctx, sb.String())
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(text), nil
}

func (c *Client) EvaluateAnswer(ctx context.Context, question, answer string) (Evaluation, error) {
	prompt := fmt.Sprintf(`You are a technical interviewer evaluating a candidate's answer.

Question: %s

Candidate's answer: %s

Respond with ONLY a JSON object (no markdown, no code fences) matching exactly this shape:
{
  "score": <integer 0-10>,
  "feedback": "<one or two sentences>",
  "missing_points": ["<point>", ...],
  "weak_topics": ["<topic>", ...],
  "next_question": "<a natural follow-up question>"
}`, question, answer)

	text, err := c.complete(ctx, prompt)
	if err != nil {
		return Evaluation{}, err
	}

	var eval Evaluation
	if err := json.Unmarshal([]byte(extractJSON(text)), &eval); err != nil {
		return Evaluation{}, fmt.Errorf("parsing evaluation JSON: %w", err)
	}
	return eval, nil
}

func (c *Client) complete(ctx context.Context, prompt string) (string, error) {
	resp, err := c.api.Messages.New(ctx, anthropic.MessageNewParams{
		Model:     c.model,
		MaxTokens: 1024,
		Messages: []anthropic.MessageParam{
			anthropic.NewUserMessage(anthropic.NewTextBlock(prompt)),
		},
	})
	if err != nil {
		return "", fmt.Errorf("calling claude: %w", err)
	}

	var out strings.Builder
	for _, block := range resp.Content {
		if text, ok := block.AsAny().(anthropic.TextBlock); ok {
			out.WriteString(text.Text)
		}
	}
	return out.String(), nil
}

func extractJSON(s string) string {
	start := strings.Index(s, "{")
	end := strings.LastIndex(s, "}")
	if start == -1 || end == -1 || end < start {
		return s
	}
	return s[start : end+1]
}
