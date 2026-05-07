package embedding

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

const (
	voyageAPIURL = "https://api.voyageai.com/v1/embeddings"
	defaultModel = "voyage-3"
)

// VoyageAdapter implements EmbeddingAdapter using the Voyage AI REST API.
type VoyageAdapter struct {
	apiKey string
	model  string
	client *http.Client
}

// NewVoyageAdapter creates a new Voyage AI embedding adapter.
func NewVoyageAdapter(apiKey, model string) *VoyageAdapter {
	if model == "" {
		model = defaultModel
	}
	return &VoyageAdapter{
		apiKey: apiKey,
		model:  model,
		client: &http.Client{Timeout: 30 * time.Second},
	}
}

type voyageRequest struct {
	Input []string `json:"input"`
	Model string   `json:"model"`
}

type voyageResponse struct {
	Data []struct {
		Embedding []float32 `json:"embedding"`
		Index     int       `json:"index"`
	} `json:"data"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

// Embed converts text to a float32 vector using Voyage AI.
// voyage-3 produces 1024-dimensional vectors.
func (v *VoyageAdapter) Embed(ctx context.Context, text string) ([]float32, error) {
	body, err := json.Marshal(voyageRequest{
		Input: []string{text},
		Model: v.model,
	})
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, voyageAPIURL, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+v.apiKey)

	resp, err := v.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("voyage api request: %w", err)
	}
	defer resp.Body.Close()

	var result voyageResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode voyage response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		msg := "unknown error"
		if result.Error != nil {
			msg = result.Error.Message
		}
		return nil, fmt.Errorf("voyage api error %d: %s", resp.StatusCode, msg)
	}

	if len(result.Data) == 0 {
		return nil, fmt.Errorf("voyage api returned empty embedding")
	}

	return result.Data[0].Embedding, nil
}
