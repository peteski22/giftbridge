package blackbaud

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

// mockTokenStore implements TokenStore for testing.
type mockTokenStore struct {
	getErr       error
	refreshToken string
	saveErr      error
}

// RefreshToken returns the current refresh token.
func (m *mockTokenStore) RefreshToken(_ context.Context) (string, error) {
	if m.getErr != nil {
		return "", m.getErr
	}
	return m.refreshToken, nil
}

// SaveRefreshToken saves a new refresh token.
func (m *mockTokenStore) SaveRefreshToken(_ context.Context, token string) error {
	if m.saveErr != nil {
		return m.saveErr
	}
	m.refreshToken = token
	return nil
}

func TestNewClient(t *testing.T) {
	t.Parallel()

	validTokenStore := &mockTokenStore{refreshToken: "test-token"}

	tests := map[string]struct {
		config  Config
		wantErr bool
		errMsg  string
	}{
		"valid config": {
			config: Config{
				ClientID:        "client-id",
				ClientSecret:    "client-secret",
				SubscriptionKey: "sub-key",
				TokenStore:      validTokenStore,
			},
			wantErr: false,
		},
		"missing client ID": {
			config: Config{
				ClientSecret:    "client-secret",
				SubscriptionKey: "sub-key",
				TokenStore:      validTokenStore,
			},
			wantErr: true,
			errMsg:  "client ID is required",
		},
		"missing client secret": {
			config: Config{
				ClientID:        "client-id",
				SubscriptionKey: "sub-key",
				TokenStore:      validTokenStore,
			},
			wantErr: true,
			errMsg:  "client secret is required",
		},
		"missing subscription key": {
			config: Config{
				ClientID:     "client-id",
				ClientSecret: "client-secret",
				TokenStore:   validTokenStore,
			},
			wantErr: true,
			errMsg:  "subscription key is required",
		},
		"missing token store": {
			config: Config{
				ClientID:        "client-id",
				ClientSecret:    "client-secret",
				SubscriptionKey: "sub-key",
			},
			wantErr: true,
			errMsg:  "token store is required",
		},
		"multiple missing fields": {
			config:  Config{},
			wantErr: true,
			errMsg:  "client ID is required",
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			client, err := NewClient(tc.config)

			if tc.wantErr {
				require.Error(t, err)
				require.Contains(t, err.Error(), tc.errMsg)
				require.Nil(t, client)
			} else {
				require.NoError(t, err)
				require.NotNil(t, client)
			}
		})
	}
}

func TestNewClientWithOptions(t *testing.T) {
	t.Parallel()

	validConfig := Config{
		ClientID:        "client-id",
		ClientSecret:    "client-secret",
		SubscriptionKey: "sub-key",
		TokenStore:      &mockTokenStore{refreshToken: "test-token"},
	}

	tests := map[string]struct {
		errMsg      string
		expectedURL string
		opts        []Option
		wantErr     bool
	}{
		"default base URL": {
			opts:        nil,
			expectedURL: "https://api.sky.blackbaud.com",
			wantErr:     false,
		},
		"custom base URL": {
			opts:        []Option{WithBaseURL("https://custom.api.com")},
			expectedURL: "https://custom.api.com",
			wantErr:     false,
		},
		"invalid option - empty base URL": {
			opts:    []Option{WithBaseURL("")},
			wantErr: true,
			errMsg:  "base URL cannot be empty",
		},
		"invalid option - nil HTTP client": {
			opts:    []Option{WithHTTPClient(nil)},
			wantErr: true,
			errMsg:  "HTTP client cannot be nil",
		},
		"invalid option - zero timeout": {
			opts:    []Option{WithTimeout(0)},
			wantErr: true,
			errMsg:  "timeout must be positive",
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			client, err := NewClient(validConfig, tc.opts...)

			if tc.wantErr {
				require.Error(t, err)
				require.Contains(t, err.Error(), tc.errMsg)
				require.Nil(t, client)
			} else {
				require.NoError(t, err)
				require.NotNil(t, client)
				require.Equal(t, tc.expectedURL, client.baseURL)
			}
		})
	}
}

func TestConfigValidate(t *testing.T) {
	t.Parallel()

	validTokenStore := &mockTokenStore{refreshToken: "test-token"}

	tests := map[string]struct {
		config       Config
		wantErr      bool
		errCount     int
		errFragments []string
	}{
		"valid config": {
			config: Config{
				ClientID:        "id",
				ClientSecret:    "secret",
				SubscriptionKey: "key",
				TokenStore:      validTokenStore,
			},
			wantErr: false,
		},
		"all fields missing": {
			config:   Config{},
			wantErr:  true,
			errCount: 4,
			errFragments: []string{
				"client ID is required",
				"client secret is required",
				"subscription key is required",
				"token store is required",
			},
		},
		"only token store missing": {
			config: Config{
				ClientID:        "id",
				ClientSecret:    "secret",
				SubscriptionKey: "key",
			},
			wantErr:      true,
			errCount:     1,
			errFragments: []string{"token store is required"},
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			err := tc.config.validate()

			if tc.wantErr {
				require.Error(t, err)
				for _, fragment := range tc.errFragments {
					require.Contains(t, err.Error(), fragment)
				}
			} else {
				require.NoError(t, err)
			}
		})
	}
}
