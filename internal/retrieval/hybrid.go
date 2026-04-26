package retrieval

import (
	"context"
	"database/sql"
	"sort"

	"github.com/shanmeiliu/rag-infra-go/internal/chat"
	"github.com/shanmeiliu/rag-infra-go/pkg/reranker"
	"github.com/shanmeiliu/rag-infra-go/pkg/vectorstore"
)

type HybridRetriever struct {
	vectorStore vectorstore.Store
	db          *sql.DB
	topK        int
	alpha       float64
	reranker    reranker.Client
	rerankTopK  int
}

func NewHybridRetriever(store vectorstore.Store, db *sql.DB, topK int, alpha float64, rr reranker.Client, rerankTopK int) *HybridRetriever {
	if topK <= 0 {
		topK = 8
	}
	if alpha <= 0 || alpha >= 1 {
		alpha = 0.55
	}
	if rerankTopK <= 0 {
		rerankTopK = topK
	}

	return &HybridRetriever{
		vectorStore: store,
		db:          db,
		topK:        topK,
		alpha:       alpha,
		reranker:    rr,
		rerankTopK:  rerankTopK,
	}
}

func (r *HybridRetriever) Retrieve(ctx context.Context, query string, embedding []float32, filters map[string]any) ([]chat.Document, error) {
	candidateK := r.topK * 4
	if candidateK < 20 {
		candidateK = 20
	}

	vecResults, err := r.vectorStore.Search(ctx, embedding, candidateK, filters)
	if err != nil {
		return nil, err
	}

	kwResults, err := KeywordSearch(ctx, r.db, query, candidateK, filters)
	if err != nil {
		return nil, err
	}

	scoreMap := map[string]float64{}
	docMap := map[string]chat.Document{}

	for _, v := range vecResults {
		score := 1.0 / (1.0 + v.Score)
		scoreMap[v.ChunkID] += r.alpha * score
		docMap[v.ChunkID] = chat.Document{
			ID:      v.ChunkID,
			Content: v.Content,
			Source:  v.DocID,
		}
	}

	for _, k := range kwResults {
		scoreMap[k.ChunkID] += (1 - r.alpha) * k.Score
		if _, exists := docMap[k.ChunkID]; !exists {
			docMap[k.ChunkID] = chat.Document{
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
		pairs = append(pairs, pair{id: id, score: score})
	}

	sort.Slice(pairs, func(i, j int) bool {
		return pairs[i].score > pairs[j].score
	})

	candidates := make([]reranker.Candidate, 0, min(len(pairs), candidateK))
	for i := 0; i < len(pairs) && i < candidateK; i++ {
		doc := docMap[pairs[i].id]
		candidates = append(candidates, reranker.Candidate{
			ID:      doc.ID,
			DocID:   doc.Source,
			Content: doc.Content,
			Score:   pairs[i].score,
		})
	}

	if r.reranker != nil && len(candidates) > 0 {
		reranked, err := r.reranker.Rerank(ctx, query, candidates, r.rerankTopK)
		if err == nil && len(reranked) > 0 {
			out := make([]chat.Document, 0, min(r.topK, len(reranked)))
			for i := 0; i < len(reranked) && i < r.topK; i++ {
				out = append(out, chat.Document{
					ID:      reranked[i].ID,
					Content: reranked[i].Content,
					Source:  reranked[i].DocID,
				})
			}
			return out, nil
		}
	}

	results := make([]chat.Document, 0, min(r.topK, len(candidates)))
	for i := 0; i < len(candidates) && i < r.topK; i++ {
		results = append(results, chat.Document{
			ID:      candidates[i].ID,
			Content: candidates[i].Content,
			Source:  candidates[i].DocID,
		})
	}

	return results, nil
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
