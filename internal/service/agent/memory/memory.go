package memory

import "context"

type RetrievalOpts struct {
	Limit  int
	UserID uint
}

type MemoryItem struct {
	Content    string
	Importance float64
	Source     string // "working" | "semantic"
}

type Memory interface {
	Add(ctx context.Context, item *MemoryItem) error
	Retrieve(ctx context.Context, query string, opts RetrievalOpts) ([]*MemoryItem, error)
	Clear(ctx context.Context) error
}
