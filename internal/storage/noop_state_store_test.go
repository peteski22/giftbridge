package storage

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestNewNoopStateStore(t *testing.T) {
	t.Parallel()

	since := time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC)

	store := NewNoopStateStore(since)

	require.NotNil(t, store)
	require.Equal(t, since, store.since)
}

func TestNoopStateStoreLastSyncTime(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		since time.Time
	}{
		"specific time": {
			since: time.Date(2024, 6, 15, 12, 0, 0, 0, time.UTC),
		},
		"zero time": {
			since: time.Time{},
		},
		"now": {
			since: time.Now(),
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			store := NewNoopStateStore(tc.since)

			result, err := store.LastSyncTime(context.Background())

			require.NoError(t, err)
			require.Equal(t, tc.since, result)
		})
	}
}

func TestNoopStateStoreSetLastSyncTime(t *testing.T) {
	t.Parallel()

	since := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	store := NewNoopStateStore(since)

	newTime := time.Date(2024, 12, 31, 23, 59, 59, 0, time.UTC)

	// SetLastSyncTime should do nothing and return nil.
	err := store.SetLastSyncTime(context.Background(), newTime)

	require.NoError(t, err)

	// LastSyncTime should still return the original since value.
	result, err := store.LastSyncTime(context.Background())
	require.NoError(t, err)
	require.Equal(t, since, result)
}

func TestNoopStateStoreMultipleCalls(t *testing.T) {
	t.Parallel()

	since := time.Date(2024, 3, 15, 8, 0, 0, 0, time.UTC)
	store := NewNoopStateStore(since)
	ctx := context.Background()

	// Multiple calls to LastSyncTime should return the same value.
	for i := 0; i < 5; i++ {
		result, err := store.LastSyncTime(ctx)
		require.NoError(t, err)
		require.Equal(t, since, result)
	}

	// Multiple calls to SetLastSyncTime should all succeed.
	for i := 0; i < 5; i++ {
		err := store.SetLastSyncTime(ctx, time.Now())
		require.NoError(t, err)
	}

	// Original value unchanged.
	result, err := store.LastSyncTime(ctx)
	require.NoError(t, err)
	require.Equal(t, since, result)
}
