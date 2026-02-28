package source

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/priyanshujain/openbotkit/store"
)

// Status represents the current state of a data source.
type Status struct {
	Connected    bool
	Accounts     []string
	ItemCount    int64
	LastSyncedAt *time.Time
}

// Source is the minimal interface that all data sources implement.
type Source interface {
	Name() string
	Status(ctx context.Context, db *store.DB) (*Status, error)
}

// registry holds all registered sources.
var (
	registryMu sync.RWMutex
	registry   = map[string]Source{}
)

// Register adds a source to the global registry.
func Register(s Source) {
	registryMu.Lock()
	defer registryMu.Unlock()
	registry[s.Name()] = s
}

// Get returns a registered source by name.
func Get(name string) (Source, error) {
	registryMu.RLock()
	defer registryMu.RUnlock()
	s, ok := registry[name]
	if !ok {
		return nil, fmt.Errorf("source %q not registered", name)
	}
	return s, nil
}

// All returns all registered sources.
func All() []Source {
	registryMu.RLock()
	defer registryMu.RUnlock()
	sources := make([]Source, 0, len(registry))
	for _, s := range registry {
		sources = append(sources, s)
	}
	return sources
}
