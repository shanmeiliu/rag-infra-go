package providers

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/shanmeiliu/rag-infra-go/pkg/llm"
)

type OpenAIClient struct {
	apiKey  string
	baseURL string
	model   string
	client  *http.Client
	cb      *CircuitBreaker
	httpCfg HTTPSettings
}

func NewOpenAIClient() *OpenAIClient {
	cfg := LoadProviderConfig()
	httpCfg := cfg.HTTPSettings()

	return &OpenAIClient{
		apiKey:  os.Getenv("OPENAI_API_KEY"),
		baseURL: getEnv("OPENAI_BASE_URL", "https://api.openai.com/v1"),
		model:   getEnv("OPENAI_CHAT_MODEL", "gpt-4o-mini"),
		client:  &http.Client{Timeout: httpCfg.Timeout},
		cb:      NewCircuitBreaker(httpCfg.CircuitThreshold, httpCfg.CircuitCooldown),
		httpCfg: httpCfg,
	}
}

func (c *OpenAIClient) Generate(ctx context.Context, prompt string) (string, error) {
	if err := c.cb.Allow(); err != nil {
		return "", err
	}

	reqBody := map[string]any{
		"model": c.model,
		"messages": []map[string]string{
			{"role": "system", "content": "You are a helpful assistant."},
			{"role": "user", "content": prompt},
		},
	}

	data, err := json.Marshal(reqBody)
	if err != nil {
		return "", err
	}

	var lastErr error
	for attempt := 0; attempt <= c.httpCfg.MaxRetries; attempt++ {
		out, err := c.doGenerateRequest(ctx, data)
		if err == nil {
			c.cb.RecordSuccess()
			return out, nil
		}

		lastErr = err
		if attempt == c.httpCfg.MaxRetries || !shouldRetryError(err) {
			break
		}

		delay := c.httpCfg.RetryBaseDelay * time.Duration(attempt+1)
		if sleepErr := sleepWithContext(ctx, delay); sleepErr != nil {
			c.cb.RecordFailure()
			return "", sleepErr
		}
	}

	c.cb.RecordFailure()
	return "", lastErr
}

func (c *OpenAIClient) doGenerateRequest(ctx context.Context, data []byte) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/chat/completions", bytes.NewBuffer(data))
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if shouldRetryStatus(resp.StatusCode) {
		return "", fmt.Errorf("retryable llm request failure: %s", resp.Status)
	}
	if resp.StatusCode >= 300 {
		var raw bytes.Buffer
		_, _ = raw.ReadFrom(resp.Body)
		return "", fmt.Errorf("openai chat failed: %s - %s", resp.Status, raw.String())
	}

	var result struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", err
	}
	if len(result.Choices) == 0 {
		return "", fmt.Errorf("no response from llm")
	}

	return result.Choices[0].Message.Content, nil
}

func (c *OpenAIClient) Stream(ctx context.Context, prompt string) (<-chan string, error) {
	if err := c.cb.Allow(); err != nil {
		return nil, err
	}

	reqBody := map[string]any{
		"model":  c.model,
		"stream": true,
		"messages": []map[string]string{
			{"role": "system", "content": "You are a helpful assistant."},
			{"role": "user", "content": prompt},
		},
	}

	data, err := json.Marshal(reqBody)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/chat/completions", bytes.NewBuffer(data))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.client.Do(req)
	if err != nil {
		c.cb.RecordFailure()
		return nil, err
	}

	if shouldRetryStatus(resp.StatusCode) {
		resp.Body.Close()
		c.cb.RecordFailure()
		return nil, fmt.Errorf("retryable llm stream failure: %s", resp.Status)
	}
	if resp.StatusCode >= 300 {
		var raw bytes.Buffer
		_, _ = raw.ReadFrom(resp.Body)
		resp.Body.Close()
		c.cb.RecordFailure()
		return nil, fmt.Errorf("openai stream failed: %s - %s", resp.Status, raw.String())
	}

	c.cb.RecordSuccess()

	out := make(chan string)

	go func() {
		defer close(out)
		defer resp.Body.Close()

		scanner := bufio.NewScanner(resp.Body)
		buf := make([]byte, 0, 64*1024)
		scanner.Buffer(buf, 1024*1024)

		for scanner.Scan() {
			line := strings.TrimSpace(scanner.Text())
			if line == "" {
				continue
			}
			if !strings.HasPrefix(line, "data: ") {
				continue
			}

			payload := strings.TrimPrefix(line, "data: ")
			if payload == "[DONE]" {
				return
			}

			var chunk struct {
				Choices []struct {
					Delta struct {
						Content string `json:"content"`
					} `json:"delta"`
				} `json:"choices"`
			}

			if err := json.Unmarshal([]byte(payload), &chunk); err != nil {
				continue
			}
			if len(chunk.Choices) == 0 {
				continue
			}

			text := chunk.Choices[0].Delta.Content
			if text == "" {
				continue
			}

			select {
			case <-ctx.Done():
				return
			case out <- text:
			}
		}
	}()

	return out, nil
}

var _ llm.Client = (*OpenAIClient)(nil)
