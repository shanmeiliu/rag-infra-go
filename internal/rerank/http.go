package rerank

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/shanmeiliu/rag-infra-go/internal/providers"
	"github.com/shanmeiliu/rag-infra-go/pkg/reranker"
)

type HTTPClient struct {
	url     string
	apiKey  string
	model   string
	client  *http.Client
	cb      *providers.CircuitBreaker
	httpCfg providers.HTTPSettings
}

func NewHTTPClient(cfg Config) *HTTPClient {
	httpCfg := providers.DefaultHTTPSettings()

	return &HTTPClient{
		url:     cfg.URL,
		apiKey:  cfg.APIKey,
		model:   cfg.Model,
		client:  &http.Client{Timeout: httpCfg.Timeout},
		cb:      providers.NewCircuitBreaker(httpCfg.CircuitThreshold, httpCfg.CircuitCooldown),
		httpCfg: httpCfg,
	}
}

func (c *HTTPClient) Rerank(ctx context.Context, query string, docs []reranker.Candidate, topK int) ([]reranker.Candidate, error) {
	if len(docs) == 0 {
		return docs, nil
	}

	if err := c.cb.Allow(); err != nil {
		return nil, err
	}

	inputDocs := make([]string, 0, len(docs))
	for _, d := range docs {
		inputDocs = append(inputDocs, d.Content)
	}

	reqBody := map[string]any{
		"model":     c.model,
		"query":     query,
		"documents": inputDocs,
		"top_n":     topK,
	}

	data, err := json.Marshal(reqBody)
	if err != nil {
		return nil, err
	}

	var lastErr error

	for attempt := 0; attempt <= c.httpCfg.MaxRetries; attempt++ {
		out, err := c.doRequest(ctx, data, docs)
		if err == nil {
			c.cb.RecordSuccess()
			return out, nil
		}

		lastErr = err
		if attempt == c.httpCfg.MaxRetries || !providersShouldRetry(err) {
			break
		}

		delay := c.httpCfg.RetryBaseDelay * time.Duration(attempt+1)
		if sleepErr := providersSleep(ctx, delay); sleepErr != nil {
			c.cb.RecordFailure()
			return nil, sleepErr
		}
	}

	c.cb.RecordFailure()
	return nil, lastErr
}

func (c *HTTPClient) doRequest(ctx context.Context, data []byte, docs []reranker.Candidate) ([]reranker.Candidate, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.url, bytes.NewBuffer(data))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	if c.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.apiKey)
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 500 || resp.StatusCode == http.StatusTooManyRequests {
		return nil, fmt.Errorf("retryable reranker failure: %s", resp.Status)
	}
	if resp.StatusCode >= 300 {
		var raw bytes.Buffer
		_, _ = raw.ReadFrom(resp.Body)
		return nil, fmt.Errorf("reranker failed: %s - %s", resp.Status, raw.String())
	}

	var result struct {
		Results []struct {
			Index          int     `json:"index"`
			RelevanceScore float64 `json:"relevance_score"`
		} `json:"results"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	if len(result.Results) == 0 {
		return docs, nil
	}

	out := make([]reranker.Candidate, 0, len(result.Results))
	for _, item := range result.Results {
		if item.Index < 0 || item.Index >= len(docs) {
			continue
		}
		doc := docs[item.Index]
		doc.Score = item.RelevanceScore
		out = append(out, doc)
	}

	if len(out) == 0 {
		return docs, nil
	}

	return out, nil
}
