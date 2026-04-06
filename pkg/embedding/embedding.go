package embedding

package embedding

import "context"

type Client interface {
	Embed(ctx context.Context, text string) ([]float32, error)
	Dimension() int
	SetDimension(dim int)
	ProviderName() string
	ModelName() string
}