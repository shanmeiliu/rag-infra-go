package embedding

import "context"

type Client interface {
	Embed(ctx context.Context, text string) ([]float32, error)
	Dimension() int
	ProviderName() string
	ModelName() string
}