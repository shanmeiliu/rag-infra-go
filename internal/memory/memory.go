package memory

import (
	"context"
	"sync"
)

type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type InMemoryStore struct {
	mu    sync.RWMutex
	store map[string][]Message
}

func NewInMemoryStore() *InMemoryStore {
	return &InMemoryStore{
		store: make(map[string][]Message),
	}
}

func (s *InMemoryStore) Load(ctx context.Context, sessionID string) ([]Message, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	msgs := s.store[sessionID]
	out := make([]Message, len(msgs))
	copy(out, msgs)
	return out, nil
}

func (s *InMemoryStore) Save(ctx context.Context, sessionID string, msg Message) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.store[sessionID] = append(s.store[sessionID], msg)
	return nil
}
