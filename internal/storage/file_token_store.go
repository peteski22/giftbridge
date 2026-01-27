package storage

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// FileTokenStore stores OAuth tokens in a local file.
type FileTokenStore struct {
	path string
}

// NewFileTokenStore creates a new FileTokenStore that reads/writes to the given path.
func NewFileTokenStore(path string) (*FileTokenStore, error) {
	if path == "" {
		return nil, fmt.Errorf("token file path is required")
	}
	return &FileTokenStore{path: path}, nil
}

// RefreshToken returns the current refresh token from the file.
func (s *FileTokenStore) RefreshToken(_ context.Context) (string, error) {
	data, err := os.ReadFile(s.path)
	if err != nil {
		if os.IsNotExist(err) {
			return "", fmt.Errorf("token file not found: %s (run 'giftbridge auth' to authenticate)", s.path)
		}
		return "", fmt.Errorf("reading token file: %w", err)
	}

	token := strings.TrimSpace(string(data))
	if token == "" {
		return "", fmt.Errorf("token file is empty: %s", s.path)
	}

	return token, nil
}

// SaveRefreshToken saves the refresh token to the file.
func (s *FileTokenStore) SaveRefreshToken(_ context.Context, token string) error {
	dir := filepath.Dir(s.path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("creating token directory: %w", err)
	}

	if err := os.WriteFile(s.path, []byte(token+"\n"), 0o600); err != nil {
		return fmt.Errorf("writing token file: %w", err)
	}

	return nil
}
