package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/shahid-io/inode/internal/model"
)

// OllamaAdapter implements LLMAdapter using a local Ollama server.
type OllamaAdapter struct {
	baseURL string
	model   string
	client  *http.Client
}

// NewOllamaAdapter creates an adapter pointed at the given Ollama base URL.
func NewOllamaAdapter(baseURL, modelName string) *OllamaAdapter {
	if baseURL == "" {
		baseURL = "http://localhost:11434"
	}
	if modelName == "" {
		modelName = "llama3.2"
	}
	return &OllamaAdapter{
		baseURL: strings.TrimRight(baseURL, "/"),
		model:   modelName,
		client:  &http.Client{Timeout: 120 * time.Second},
	}
}

type ollamaChatRequest struct {
	Model    string          `json:"model"`
	Messages []ollamaMessage `json:"messages"`
	Stream   bool            `json:"stream"`
	Format   string          `json:"format,omitempty"`
}

type ollamaMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type ollamaChatResponse struct {
	Message ollamaMessage `json:"message"`
}

func (o *OllamaAdapter) chat(ctx context.Context, systemPrompt, userPrompt string, jsonFormat bool) (string, error) {
	messages := []ollamaMessage{}
	if systemPrompt != "" {
		messages = append(messages, ollamaMessage{Role: "system", Content: systemPrompt})
	}
	messages = append(messages, ollamaMessage{Role: "user", Content: userPrompt})

	req := ollamaChatRequest{
		Model:    o.model,
		Messages: messages,
		Stream:   false,
	}
	if jsonFormat {
		req.Format = "json"
	}

	body, err := json.Marshal(req)
	if err != nil {
		return "", err
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, o.baseURL+"/api/chat", bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := o.client.Do(httpReq)
	if err != nil {
		return "", fmt.Errorf("ollama request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("ollama returned %d", resp.StatusCode)
	}

	var result ollamaChatResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("ollama decode: %w", err)
	}
	return result.Message.Content, nil
}

// Classify returns category, tags, sensitivity, and a one-line summary.
func (o *OllamaAdapter) Classify(ctx context.Context, content string, categories []model.Category, tags []string) (ClassifyResult, error) {
	system := `You are a note classifier. Always respond with valid JSON only. No explanation, no markdown.`
	user := fmt.Sprintf(`Classify the following note.

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

	raw, err := o.chat(ctx, system, user, true)
	if err != nil {
		return ClassifyResult{}, fmt.Errorf("ollama classify: %w", err)
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
func (o *OllamaAdapter) Answer(ctx context.Context, query string, notes []*model.Note) (AnswerResult, error) {
	if len(notes) == 0 {
		return AnswerResult{Answer: "No relevant notes found.", Matched: false}, nil
	}

	system := `You are a personal knowledge assistant. Always respond with valid JSON only. No markdown, no prose outside JSON.`
	user := fmt.Sprintf(`Answer the user's query using ONLY the notes below.

Return JSON exactly in this shape:
{"matched": <true|false>, "answer": "<your answer>", "used_note_ids": ["<short_id>", ...]}

Set "matched" to true only if at least one note actually contains the answer.
Set "used_note_ids" to the short IDs (as printed in the "--- Note N (id: XXXXXXXX, ...) ---" headers) of the notes you used. Empty array if none.
If "matched" is false, "answer" should briefly say the information is not in the notes.

Notes:
%s
Query: %s`, buildContext(notes), query)

	raw, err := o.chat(ctx, system, user, true)
	if err != nil {
		return AnswerResult{}, fmt.Errorf("ollama answer: %w", err)
	}
	return parseAnswerJSON(raw)
}

// Summarize returns a one-line description of note content.
func (o *OllamaAdapter) Summarize(ctx context.Context, content string) (string, error) {
	raw, err := o.chat(ctx, "", fmt.Sprintf("Summarize this note in one concise sentence (max 80 chars). Return only the sentence:\n\n%s", content), false)
	if err != nil {
		return "", fmt.Errorf("ollama summarize: %w", err)
	}
	return strings.TrimSpace(raw), nil
}
