package llm

import "context"

type Client interface {
	Generate(ctx context.Context, prompt string) (string, error)
	Stream(ctx context.Context, prompt string) (<-chan string, error)
}
