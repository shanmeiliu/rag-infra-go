package ingestion

import "context"

type Node interface {
	Name() string
	Run(ctx context.Context, state map[string]any) error
}

type Pipeline struct {
	nodes []Node
}

func NewPipeline(nodes ...Node) *Pipeline {
	return &Pipeline{nodes: nodes}
}

func (p *Pipeline) Run(ctx context.Context, state map[string]any) error {
	for _, node := range p.nodes {
		if err := node.Run(ctx, state); err != nil {
			return err
		}
	}
	return nil
}
