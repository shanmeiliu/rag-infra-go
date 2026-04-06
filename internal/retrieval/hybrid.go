package retrieval

import (
	"context"
	"database/sql"
	"sort"

	"github.com/shanmeiliu/rag-infra-go/pkg/vectorstore"
)

type HybridRetriever struct {
	vectorStore vectorstore.Store
	db          *sql.DB
	topK        int
	alpha       float64
}

func NewHybridRetriever(store vectorstore.Store, db *sql.DB, topK int, alpha float64) *HybridRetriever {
	return &HybridRetriever{
		vectorStore: store,
		db:          db,
		topK:        topK,
		alpha:       alpha,
	}
}

func (r *HybridRetriever) Retrieve(ctx context.Context, query string, embedding []float32) ([]Document, error) {
	vecResults, err := r.vectorStore.Search(ctx, embedding, r.topK, nil)
	if err != nil {
		return nil, err
	}

	kwResults, err := KeywordSearch(ctx, r.db, query, r.topK)
	if err != nil {
		return nil, err
	}

	scoreMap := map[string]float64{}
	docMap := map[string]Document{}

	// vector scores (lower distance = better → invert)
	for _, v := range vecResults {
		score := 1.0 / (1.0 + v.Score)
		scoreMap[v.ChunkID] += r.alpha * score

		docMap[v.ChunkID] = Document{
			ID:      v.ChunkID,
			Content: v.Content,
			Source:  v.DocID,
		}
	}

	// keyword scores
	for _, k := range kwResults {
		scoreMap[k.ChunkID] += (1 - r.alpha) * k.Score

		if _, exists := docMap[k.ChunkID]; !exists {
			docMap[k.ChunkID] = Document{
				ID:      k.ChunkID,
				Content: k.Content,
				Source:  k.DocID,
			}
		}
	}

	type pair struct {
		id    string
		score float64
	}

	var pairs []pair
	for id, score := range scoreMap {
		pairs = append(pairs, pair{id, score})
	}

	sort.Slice(pairs, func(i, j int) bool {
		return pairs[i].score > pairs[j].score
	})

	var results []Document
	for i := 0; i < len(pairs) && i < r.topK; i++ {
		results = append(results, docMap[pairs[i].id])
	}

	return results, nil
}