package llm

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"
	"github.com/shahid-io/inode/internal/model"
)

// ClaudeAdapter implements LLMAdapter using the Anthropic Claude API.
type ClaudeAdapter struct {
	client anthropic.Client
	model  anthropic.Model
}

// NewClaudeAdapter creates a Claude API adapter.
func NewClaudeAdapter(apiKey, modelName string) *ClaudeAdapter {
	if modelName == "" {
		modelName = string(anthropic.ModelClaudeSonnet4_5)
	}
	return &ClaudeAdapter{
		client: anthropic.NewClient(option.WithAPIKey(apiKey)),
		model:  anthropic.Model(modelName),
	}
}

// Classify sends note content to Claude and returns structured metadata.
func (c *ClaudeAdapter) Classify(ctx context.Context, content string, categories []model.Category, tags []string) (ClassifyResult, error) {
	prompt := fmt.Sprintf(`Classify the following note. Return ONLY a JSON object, no markdown, no explanation.

Predefined categories (choose the best match):
%s
Suggested tags (use relevant ones, add new ones if needed): %s

Note content:
%s

Return JSON:
{"category":"<name>","tags":["tag1","tag2"],"is_sensitive":<bool>,"summary":"<one line, max 80 chars>"}`,
		formatCategories(categories),
		strings.Join(tags, ", "),
		content,
	)

	msg, err := c.client.Messages.New(ctx, anthropic.MessageNewParams{
		Model:     c.model,
		MaxTokens: 256,
		Messages: []anthropic.MessageParam{
			anthropic.NewUserMessage(anthropic.NewTextBlock(prompt)),
		},
	})
	if err != nil {
		return ClassifyResult{}, fmt.Errorf("claude classify: %w", err)
	}

	raw := extractJSON(textFromMessage(msg))

	var parsed struct {
		Category    string   `json:"category"`
		Tags        []string `json:"tags"`
		IsSensitive bool     `json:"is_sensitive"`
		Summary     string   `json:"summary"`
	}
	if err := json.Unmarshal([]byte(raw), &parsed); err != nil {
		return ClassifyResult{}, fmt.Errorf("parse classify response %q: %w", raw, err)
	}

	return ClassifyResult{
		Category:    parsed.Category,
		Tags:        parsed.Tags,
		IsSensitive: parsed.IsSensitive,
		Summary:     parsed.Summary,
	}, nil
}

// Answer performs RAG generation — retrieved notes as context, Claude answers.
// Returns a structured result so the caller knows whether the notes were
// actually useful and which ones the model relied on.
func (c *ClaudeAdapter) Answer(ctx context.Context, query string, notes []*model.Note) (AnswerResult, error) {
	if len(notes) == 0 {
		return AnswerResult{Answer: "No relevant notes found.", Matched: false}, nil
	}

	prompt := fmt.Sprintf(`You are a personal knowledge assistant. Answer the user's query using ONLY the notes provided below. For sensitive values, include them as-is — the CLI handles masking.

Return ONLY a JSON object, no markdown:
{"matched": <true|false>, "answer": "<your answer>", "used_note_ids": ["<short_id>", ...]}

- matched: true if any note answers the query.
- answer: your natural-language reply, using the actual values from the notes when the answer is there.
- used_note_ids: only the notes you actually drew from to write your answer. Skip every note you did not reference.

Use the 8-char id from each "--- Note N (id: XXXXXXXX, ...) ---" header.

Notes:
%s
Query: %s`, buildContext(notes), query)

	msg, err := c.client.Messages.New(ctx, anthropic.MessageNewParams{
		Model:     c.model,
		MaxTokens: 512,
		Messages: []anthropic.MessageParam{
			anthropic.NewUserMessage(anthropic.NewTextBlock(prompt)),
		},
	})
	if err != nil {
		return AnswerResult{}, fmt.Errorf("claude answer: %w", err)
	}

	return parseAnswerJSON(textFromMessage(msg))
}

// Summarize returns a one-line description of note content.
func (c *ClaudeAdapter) Summarize(ctx context.Context, content string) (string, error) {
	prompt := fmt.Sprintf("Summarize this note in one concise sentence (max 80 chars). Return only the sentence:\n\n%s", content)

	msg, err := c.client.Messages.New(ctx, anthropic.MessageNewParams{
		Model:     c.model,
		MaxTokens: 100,
		Messages: []anthropic.MessageParam{
			anthropic.NewUserMessage(anthropic.NewTextBlock(prompt)),
		},
	})
	if err != nil {
		return "", fmt.Errorf("claude summarize: %w", err)
	}

	return strings.TrimSpace(textFromMessage(msg)), nil
}

// textFromMessage extracts the first text block from a Claude response.
func textFromMessage(msg *anthropic.Message) string {
	for _, block := range msg.Content {
		if block.Type == "text" {
			return block.Text
		}
	}
	return ""
}

// parseAnswerJSON parses the structured answer JSON returned by an LLM.
// Defensive: validates internal consistency (matched=true must come with
// at least one used_note_id; otherwise the result is coerced to not-matched).
func parseAnswerJSON(raw string) (AnswerResult, error) {
	body := extractJSON(raw)
	var parsed struct {
		Matched     bool     `json:"matched"`
		Answer      string   `json:"answer"`
		UsedNoteIDs []string `json:"used_note_ids"`
	}
	if err := json.Unmarshal([]byte(body), &parsed); err != nil {
		return AnswerResult{}, fmt.Errorf("parse answer response %q: %w", body, err)
	}

	answer := strings.TrimSpace(parsed.Answer)
	if answer == "" {
		answer = "No relevant notes found."
	}

	matched := parsed.Matched && len(parsed.UsedNoteIDs) > 0
	if !matched {
		return AnswerResult{Answer: answer, Matched: false}, nil
	}
	return AnswerResult{Answer: answer, Matched: true, UsedNoteIDs: parsed.UsedNoteIDs}, nil
}

// extractJSON strips markdown fences and extracts the first JSON object.
func extractJSON(s string) string {
	s = strings.TrimSpace(s)
	if strings.HasPrefix(s, "```") {
		lines := strings.Split(s, "\n")
		var inner []string
		for _, l := range lines[1:] {
			if strings.HasPrefix(l, "```") {
				break
			}
			inner = append(inner, l)
		}
		s = strings.Join(inner, "\n")
	}
	start := strings.Index(s, "{")
	end := strings.LastIndex(s, "}")
	if start >= 0 && end > start {
		return s[start : end+1]
	}
	return s
}

func formatCategories(cats []model.Category) string {
	var sb strings.Builder
	for _, c := range cats {
		fmt.Fprintf(&sb, "- %s: %s\n", c.Name, c.Description)
	}
	return sb.String()
}

func buildContext(notes []*model.Note) string {
	var sb strings.Builder
	for i, n := range notes {
		content := n.ContentPlain
		if content == "" && len(n.ContentEnc) > 0 {
			content = "[encrypted]"
		}
		shortID := n.ID
		if len(shortID) > 8 {
			shortID = shortID[:8]
		}
		fmt.Fprintf(&sb, "--- Note %d (id: %s, category: %s, tags: %s) ---\n%s\n\n",
			i+1, shortID, n.Category, strings.Join(n.Tags, ", "), content)
	}
	return sb.String()
}
