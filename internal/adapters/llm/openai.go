package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/BiSemaphore/inode/internal/model"
)

// OpenAIAdapter implements the LLM Adapter using the OpenAI Chat Completions
// API. It talks plain HTTP (no SDK dependency) so the binary stays lean and
// `go install` needs no extra modules, mirroring the Ollama adapter's style,
// but pointed at OpenAI with Bearer auth and JSON-mode responses.
type OpenAIAdapter struct {
	baseURL string
	apiKey  string
	model   string
	client  *http.Client
}

// NewOpenAIAdapter creates an adapter for the OpenAI Chat Completions API.
// baseURL defaults to the public API; override it to target Azure OpenAI, a
// gateway, or a compatible endpoint. modelName defaults to a fast, cheap model.
func NewOpenAIAdapter(apiKey, modelName, baseURL string) *OpenAIAdapter {
	if baseURL == "" {
		baseURL = OpenAIDefaultURL
	}
	if modelName == "" {
		modelName = OpenAIDefaultModel
	}
	return &OpenAIAdapter{
		baseURL: strings.TrimRight(baseURL, "/"),
		apiKey:  apiKey,
		model:   modelName,
		client:  &http.Client{Timeout: 120 * time.Second},
	}
}

type openAIChatRequest struct {
	Model          string             `json:"model"`
	Messages       []openAIMessage    `json:"messages"`
	MaxTokens      int                `json:"max_tokens,omitempty"`
	ResponseFormat *openAIResponseFmt `json:"response_format,omitempty"`
}

type openAIResponseFmt struct {
	Type string `json:"type"` // "json_object" enables JSON mode
}

type openAIMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type openAIChatResponse struct {
	Choices []struct {
		Message openAIMessage `json:"message"`
	} `json:"choices"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error"`
}

// chat sends a system+user turn to the Chat Completions endpoint. When
// jsonFormat is true it enables JSON mode; the OpenAI API requires the word
// "json" to appear in the prompt for that mode, which our prompts satisfy.
func (o *OpenAIAdapter) chat(ctx context.Context, systemPrompt, userPrompt string, maxTokens int, jsonFormat bool) (string, error) {
	messages := []openAIMessage{}
	if systemPrompt != "" {
		messages = append(messages, openAIMessage{Role: "system", Content: systemPrompt})
	}
	messages = append(messages, openAIMessage{Role: "user", Content: userPrompt})

	reqBody := openAIChatRequest{
		Model:     o.model,
		Messages:  messages,
		MaxTokens: maxTokens,
	}
	if jsonFormat {
		reqBody.ResponseFormat = &openAIResponseFmt{Type: "json_object"}
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return "", err
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, o.baseURL+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+o.apiKey)

	resp, err := o.client.Do(httpReq)
	if err != nil {
		return "", fmt.Errorf("openai request: %w", err)
	}
	defer resp.Body.Close()

	var result openAIChatResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("openai decode: %w", err)
	}
	if result.Error != nil {
		return "", fmt.Errorf("openai error (%d): %s", resp.StatusCode, result.Error.Message)
	}
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("openai returned %d", resp.StatusCode)
	}
	if len(result.Choices) == 0 {
		return "", fmt.Errorf("openai returned no choices")
	}
	return result.Choices[0].Message.Content, nil
}

// Classify returns category, tags, sensitivity, and a one-line summary.
func (o *OpenAIAdapter) Classify(ctx context.Context, content string, categories []model.Category, tags []string) (ClassifyResult, error) {
	system := `You are a note classifier. Always respond with valid JSON only. No explanation, no markdown.`
	user := fmt.Sprintf(`Classify the following note.

Predefined categories (choose the best match):
%s
Suggested tags (use relevant ones, add new ones if needed): %s

Note content:
%s

Return JSON:
{"category":"<name>","tags":["tag1","tag2"],"is_sensitive":<bool>,"summary":"<one line, max 80 chars>"}

The summary describes what the note is for. Never copy secret values (keys, tokens, passwords, connection strings) into it.`,
		formatCategories(categories),
		strings.Join(tags, ", "),
		content,
	)

	raw, err := o.chat(ctx, system, user, 256, true)
	if err != nil {
		return ClassifyResult{}, fmt.Errorf("openai classify: %w", err)
	}

	raw = extractJSON(raw)
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

// Answer performs RAG generation using retrieved notes as context.
// Returns a structured result so the caller knows whether the notes were
// actually useful and which ones the model relied on.
func (o *OpenAIAdapter) Answer(ctx context.Context, query string, notes []*model.Note) (AnswerResult, error) {
	if len(notes) == 0 {
		return AnswerResult{Answer: "No relevant notes found.", Matched: false}, nil
	}

	system := `You are a personal knowledge assistant. Always respond with valid JSON only. No markdown, no prose outside JSON.`
	user := fmt.Sprintf(`Answer the user's query using ONLY the notes below.

Return JSON exactly in this shape:
{"matched": <true|false>, "answer": "<your answer>", "used_note_ids": ["<short_id>", ...]}

- matched: true if any note answers the query.
- answer: your natural-language reply, using the actual values from the notes when the answer is there.
- used_note_ids: only the notes you actually drew from to write your answer. Skip every note you did not reference.

Use the 8-char id from each "--- Note N (id: XXXXXXXX, ...) ---" header.

Notes:
%s
Query: %s`, buildContext(notes), query)

	raw, err := o.chat(ctx, system, user, 512, true)
	if err != nil {
		return AnswerResult{}, fmt.Errorf("openai answer: %w", err)
	}
	return parseAnswerJSON(raw)
}

// Summarize returns a one-line description of note content.
func (o *OpenAIAdapter) Summarize(ctx context.Context, content string) (string, error) {
	raw, err := o.chat(ctx, "", fmt.Sprintf("Summarize what this note is for in one concise sentence (max 80 chars). Never copy secret values (keys, tokens, passwords) into it. Return only the sentence:\n\n%s", content), 100, false)
	if err != nil {
		return "", fmt.Errorf("openai summarize: %w", err)
	}
	return strings.TrimSpace(raw), nil
}
