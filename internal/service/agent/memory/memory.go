package memory

import "context"

type RetrievalOpts struct {
	Limit  int
	UserID uint
}

// Category — 语义记忆分类。
type Category string

const (
	CategoryPreference Category = "preference"
	CategoryLesson     Category = "lesson"
)

type MemoryItem struct {
	Content    string
	Importance float64
	Source     string // "working" | "semantic"
	Category   Category
	UserID     uint
}

type Memory interface {
	Add(ctx context.Context, item *MemoryItem) error
	Retrieve(ctx context.Context, query string, opts RetrievalOpts) ([]*MemoryItem, error)
	Clear(ctx context.Context) error
}
