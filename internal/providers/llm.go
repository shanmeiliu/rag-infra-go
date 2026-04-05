package providers

import (
	"context"
	"strings"
	"time"

	"github.com/shanmeiliu/rag-infra-go/pkg/llm"
)

type MockLLMClient struct{}

func NewMockLLMClient() *MockLLMClient {
	return &MockLLMClient{}
}

func (c *MockLLMClient) Generate(ctx context.Context, prompt string) (string, error) {
	return "This is a mock answer generated from the retrieved context.", nil
}

func (c *MockLLMClient) Stream(ctx context.Context, prompt string) (<-chan string, error) {
	out := make(chan string)

	go func() {
		defer close(out)

		parts := strings.Split("This is a mock streamed answer generated from the retrieved context.", " ")
		for _, part := range parts {
			select {
			case <-ctx.Done():
				return
			case out <- part + " ":
				time.Sleep(60 * time.Millisecond)
			}
		}
	}()

	return out, nil
}

var _ llm.Client = (*MockLLMClient)(nil)
