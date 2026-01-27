package storage

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNewFileTokenStore(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		path    string
		wantErr bool
		errMsg  string
	}{
		"valid path": {
			path:    "/path/to/token",
			wantErr: false,
		},
		"empty path": {
			path:    "",
			wantErr: true,
			errMsg:  "token file path is required",
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			store, err := NewFileTokenStore(tc.path)

			if tc.wantErr {
				require.Error(t, err)
				require.Contains(t, err.Error(), tc.errMsg)
				require.Nil(t, store)
			} else {
				require.NoError(t, err)
				require.NotNil(t, store)
			}
		})
	}
}

func TestFileTokenStoreRefreshToken(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		setup       func(t *testing.T, dir string) string
		wantToken   string
		wantErr     bool
		errContains string
	}{
		"valid token file": {
			setup: func(t *testing.T, dir string) string {
				t.Helper()
				path := filepath.Join(dir, "token")
				require.NoError(t, os.WriteFile(path, []byte("my-refresh-token\n"), 0o600))
				return path
			},
			wantToken: "my-refresh-token",
			wantErr:   false,
		},
		"token file with whitespace": {
			setup: func(t *testing.T, dir string) string {
				t.Helper()
				path := filepath.Join(dir, "token")
				require.NoError(t, os.WriteFile(path, []byte("  token-with-spaces  \n"), 0o600))
				return path
			},
			wantToken: "token-with-spaces",
			wantErr:   false,
		},
		"file not found": {
			setup: func(t *testing.T, dir string) string {
				t.Helper()
				return filepath.Join(dir, "nonexistent")
			},
			wantErr:     true,
			errContains: "token file not found",
		},
		"empty file": {
			setup: func(t *testing.T, dir string) string {
				t.Helper()
				path := filepath.Join(dir, "token")
				require.NoError(t, os.WriteFile(path, []byte(""), 0o600))
				return path
			},
			wantErr:     true,
			errContains: "token file is empty",
		},
		"whitespace only file": {
			setup: func(t *testing.T, dir string) string {
				t.Helper()
				path := filepath.Join(dir, "token")
				require.NoError(t, os.WriteFile(path, []byte("   \n\t  "), 0o600))
				return path
			},
			wantErr:     true,
			errContains: "token file is empty",
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			dir := t.TempDir()
			path := tc.setup(t, dir)

			store, err := NewFileTokenStore(path)
			require.NoError(t, err)

			token, err := store.RefreshToken(context.Background())

			if tc.wantErr {
				require.Error(t, err)
				require.Contains(t, err.Error(), tc.errContains)
			} else {
				require.NoError(t, err)
				require.Equal(t, tc.wantToken, token)
			}
		})
	}
}

func TestFileTokenStoreSaveRefreshToken(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		setup     func(t *testing.T) string
		token     string
		wantErr   bool
		checkFile func(t *testing.T, path string)
	}{
		"save to new file": {
			setup: func(t *testing.T) string {
				t.Helper()
				dir := t.TempDir()
				return filepath.Join(dir, "token")
			},
			token:   "new-token",
			wantErr: false,
			checkFile: func(t *testing.T, path string) {
				t.Helper()
				data, err := os.ReadFile(path)
				require.NoError(t, err)
				require.Equal(t, "new-token\n", string(data))
			},
		},
		"overwrite existing file": {
			setup: func(t *testing.T) string {
				t.Helper()
				dir := t.TempDir()
				path := filepath.Join(dir, "token")
				require.NoError(t, os.WriteFile(path, []byte("old-token"), 0o600))
				return path
			},
			token:   "new-token",
			wantErr: false,
			checkFile: func(t *testing.T, path string) {
				t.Helper()
				data, err := os.ReadFile(path)
				require.NoError(t, err)
				require.Equal(t, "new-token\n", string(data))
			},
		},
		"creates parent directory": {
			setup: func(t *testing.T) string {
				t.Helper()
				dir := t.TempDir()
				return filepath.Join(dir, "subdir", "token")
			},
			token:   "my-token",
			wantErr: false,
			checkFile: func(t *testing.T, path string) {
				t.Helper()
				data, err := os.ReadFile(path)
				require.NoError(t, err)
				require.Equal(t, "my-token\n", string(data))
			},
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			path := tc.setup(t)

			store, err := NewFileTokenStore(path)
			require.NoError(t, err)

			err = store.SaveRefreshToken(context.Background(), tc.token)

			if tc.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				if tc.checkFile != nil {
					tc.checkFile(t, path)
				}
			}
		})
	}
}

func TestFileTokenStoreRoundTrip(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "token")

	store, err := NewFileTokenStore(path)
	require.NoError(t, err)

	ctx := context.Background()

	// Save a token.
	err = store.SaveRefreshToken(ctx, "round-trip-token")
	require.NoError(t, err)

	// Read it back.
	token, err := store.RefreshToken(ctx)
	require.NoError(t, err)
	require.Equal(t, "round-trip-token", token)

	// Overwrite with new token.
	err = store.SaveRefreshToken(ctx, "updated-token")
	require.NoError(t, err)

	// Read updated token.
	token, err = store.RefreshToken(ctx)
	require.NoError(t, err)
	require.Equal(t, "updated-token", token)
}
