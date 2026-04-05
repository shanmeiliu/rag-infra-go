package providers

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
)

type OpenAIEmbeddingClient struct {
	apiKey  string
	baseURL string
	model   string
}

func NewOpenAIEmbeddingClient(cfg ProviderConfig) *OpenAIEmbeddingClient {
	return &OpenAIEmbeddingClient{
		apiKey:  cfg.OpenAIAPIKey,
		baseURL: cfg.OpenAIBaseURL,
		model:   cfg.OpenAIEmbeddingModel,
	}
}

func (c *OpenAIEmbeddingClient) Embed(ctx context.Context, text string) ([]float32, error) {
	reqBody := map[string]any{
		"model": c.model,
		"input": text,
	}

	data, err := json.Marshal(reqBody)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/embeddings", bytes.NewBuffer(data))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		return nil, fmt.Errorf("openai embeddings failed with status %s", resp.Status)
	}

	var result struct {
		Data []struct {
			Embedding []float32 `json:"embedding"`
		} `json:"data"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}
	if len(result.Data) == 0 {
		return nil, fmt.Errorf("no embedding returned")
	}

	return result.Data[0].Embedding, nil
}

func (c *OpenAIEmbeddingClient) Dimension() int {
	return 1536
}