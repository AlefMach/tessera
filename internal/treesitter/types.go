package treesitter

import (
	"context"
	"time"

	"github.com/alef-mach/tessera/internal/memory"
	"github.com/alef-mach/tessera/internal/treesitter/model"
)

const maxIndexedFileBytes = 1 << 20

type Store interface {
	ClearIndex(ctx context.Context, sessionID string) error
	SaveFileSummary(ctx context.Context, summary memory.FileSummary) error
	SaveSymbol(ctx context.Context, symbol memory.Symbol) error
}

type Indexer struct {
	root  string
	store Store
	now   func() time.Time
}

type Option func(*Indexer)

func WithClock(now func() time.Time) Option {
	return func(i *Indexer) {
		i.now = now
	}
}

func New(root string, store Store, opts ...Option) *Indexer {
	idx := &Indexer{root: root, store: store, now: func() time.Time { return time.Now().UTC() }}
	for _, opt := range opts {
		opt(idx)
	}
	return idx
}

type Result struct {
	Files      int
	Symbols    int
	RepoMap    string
	Summaries  []model.FileIndex
	StartedAt  time.Time
	FinishedAt time.Time
}

type FileIndex = model.FileIndex
type SymbolIndex = model.SymbolIndex
