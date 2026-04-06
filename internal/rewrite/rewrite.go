package rewrite

import (
	"context"

	"github.com/shanmeiliu/rag-infra-go/internal/memory"
)

type SimpleRewriter struct{}

func NewSimpleRewriter() *SimpleRewriter {
	return &SimpleRewriter{}
}

func (r *SimpleRewriter) Rewrite(ctx context.Context, q string, _ []memory.Message) (string, error) {
	return q, nil
}
