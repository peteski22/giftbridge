package storage

import (
	"context"
	"time"
)

// NoopStateStore is a state store that does nothing.
// Used for dry-run mode where we don't persist state.
type NoopStateStore struct {
	since time.Time
}

// NewNoopStateStore creates a new NoopStateStore with the given initial time.
func NewNoopStateStore(since time.Time) *NoopStateStore {
	return &NoopStateStore{since: since}
}

// LastSyncTime returns the configured time.
func (s *NoopStateStore) LastSyncTime(_ context.Context) (time.Time, error) {
	return s.since, nil
}

// SetLastSyncTime does nothing.
func (s *NoopStateStore) SetLastSyncTime(_ context.Context, _ time.Time) error {
	return nil
}
