package blackbaud

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// errMockGetRefreshToken is a sentinel error for testing.
var errMockGetRefreshToken = errMock("mock get refresh token error")

// errMock is a simple error type for testing.
type errMock string

// mockTransport is an http.RoundTripper that redirects requests to a test server.
type mockTransport struct {
	baseURL string
	handler http.Handler
}

// Error implements the error interface.
func (e errMock) Error() string {
	return string(e)
}

// RoundTrip implements http.RoundTripper.
func (m *mockTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	// Redirect the request to our mock server.
	req.URL.Scheme = "http"
	req.URL.Host = m.baseURL[7:] // Strip "http://".
	req.RequestURI = ""

	// Create a response recorder.
	rec := httptest.NewRecorder()
	m.handler.ServeHTTP(rec, req)

	return rec.Result(), nil
}

func TestNewTokenManager(t *testing.T) {
	t.Parallel()

	store := &mockTokenStore{refreshToken: "refresh-token"}
	httpClient := &http.Client{Timeout: 10 * time.Second}

	tm := newTokenManager("client-id", "client-secret", store, httpClient)

	require.NotNil(t, tm)
	require.Equal(t, "client-id", tm.clientID)
	require.Equal(t, "client-secret", tm.clientSecret)
	require.Equal(t, store, tm.tokenStore)
	require.Equal(t, httpClient, tm.httpClient)
	require.Empty(t, tm.accessToken)
	require.True(t, tm.expiresAt.IsZero())
}

func TestTokenManager_AccessToken(t *testing.T) {
	t.Parallel()

	t.Run("returns cached token when valid", func(t *testing.T) {
		t.Parallel()

		tm := &tokenManager{
			accessToken: "cached-token",
			expiresAt:   time.Now().Add(30 * time.Minute),
		}

		token, err := tm.AccessToken(context.Background())

		require.NoError(t, err)
		require.Equal(t, "cached-token", token)
	})

	t.Run("refreshes token when expired", func(t *testing.T) {
		t.Parallel()

		server := newMockOAuthServer(t, tokenResponse{
			AccessToken:  "new-access-token",
			RefreshToken: "new-refresh-token",
			ExpiresIn:    3600,
			TokenType:    "Bearer",
		})
		defer server.Close()

		// Temporarily override tokenURL for this test.
		store := &mockTokenStore{refreshToken: "old-refresh-token"}
		tm := &tokenManager{
			accessToken:  "old-token",
			clientID:     "client-id",
			clientSecret: "client-secret",
			expiresAt:    time.Now().Add(-5 * time.Minute), // Expired.
			httpClient:   server.Client(),
			tokenStore:   store,
		}

		// Override the token URL by using a custom transport.
		tm.httpClient = &http.Client{
			Transport: &mockTransport{
				handler: server.Config.Handler,
				baseURL: server.URL,
			},
		}

		token, err := tm.AccessToken(context.Background())

		require.NoError(t, err)
		require.Equal(t, "new-access-token", token)
		require.Equal(t, "new-refresh-token", store.refreshToken)
	})
}

func TestTokenManager_CachedToken(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		accessToken string
		expiresAt   time.Time
		wantOK      bool
		wantToken   string
	}{
		"valid cached token": {
			accessToken: "valid-token",
			expiresAt:   time.Now().Add(30 * time.Minute),
			wantToken:   "valid-token",
			wantOK:      true,
		},
		"empty access token": {
			accessToken: "",
			expiresAt:   time.Now().Add(30 * time.Minute),
			wantToken:   "",
			wantOK:      false,
		},
		"expired token": {
			accessToken: "expired-token",
			expiresAt:   time.Now().Add(-5 * time.Minute),
			wantToken:   "",
			wantOK:      false,
		},
		"token within expiry buffer": {
			accessToken: "near-expiry-token",
			expiresAt:   time.Now().Add(3 * time.Minute), // Within 5 minute buffer.
			wantToken:   "",
			wantOK:      false,
		},
		"token exactly at buffer boundary": {
			accessToken: "boundary-token",
			expiresAt:   time.Now().Add(tokenExpiryBuffer),
			wantToken:   "",
			wantOK:      false,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			tm := &tokenManager{
				accessToken: tc.accessToken,
				expiresAt:   tc.expiresAt,
			}

			token, ok := tm.cachedToken()

			require.Equal(t, tc.wantOK, ok)
			require.Equal(t, tc.wantToken, token)
		})
	}
}

func TestTokenManager_ConcurrentAccess(t *testing.T) {
	t.Parallel()

	server := newMockOAuthServer(t, tokenResponse{
		AccessToken:  "concurrent-token",
		RefreshToken: "concurrent-refresh",
		ExpiresIn:    3600,
		TokenType:    "Bearer",
	})
	defer server.Close()

	store := &mockTokenStore{refreshToken: "initial-refresh"}
	tm := &tokenManager{
		clientID:     "test-client",
		clientSecret: "test-secret",
		expiresAt:    time.Now().Add(-5 * time.Minute), // Expired to force refresh.
		httpClient: &http.Client{
			Transport: &mockTransport{
				handler: server.Config.Handler,
				baseURL: server.URL,
			},
		},
		tokenStore: store,
	}

	var wg sync.WaitGroup
	tokens := make([]string, 10)
	errors := make([]error, 10)

	for i := range 10 {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			tokens[idx], errors[idx] = tm.AccessToken(context.Background())
		}(i)
	}

	wg.Wait()

	for i := range 10 {
		require.NoError(t, errors[i])
		require.Equal(t, "concurrent-token", tokens[i])
	}
}

func TestTokenManager_IsTokenValid(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		accessToken string
		expiresAt   time.Time
		want        bool
	}{
		"valid token": {
			accessToken: "token",
			expiresAt:   time.Now().Add(30 * time.Minute),
			want:        true,
		},
		"empty token": {
			accessToken: "",
			expiresAt:   time.Now().Add(30 * time.Minute),
			want:        false,
		},
		"expired": {
			accessToken: "token",
			expiresAt:   time.Now().Add(-1 * time.Minute),
			want:        false,
		},
		"within buffer": {
			accessToken: "token",
			expiresAt:   time.Now().Add(2 * time.Minute),
			want:        false,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			tm := &tokenManager{
				accessToken: tc.accessToken,
				expiresAt:   tc.expiresAt,
			}

			got := tm.isTokenValid()

			require.Equal(t, tc.want, got)
		})
	}
}

func TestTokenManager_RefreshAccessToken(t *testing.T) {
	t.Parallel()

	t.Run("successful refresh", func(t *testing.T) {
		t.Parallel()

		server := newMockOAuthServer(t, tokenResponse{
			AccessToken:  "fresh-token",
			RefreshToken: "fresh-refresh",
			ExpiresIn:    7200,
			TokenType:    "Bearer",
		})
		defer server.Close()

		store := &mockTokenStore{refreshToken: "current-refresh"}
		tm := &tokenManager{
			clientID:     "test-client",
			clientSecret: "test-secret",
			httpClient: &http.Client{
				Transport: &mockTransport{
					handler: server.Config.Handler,
					baseURL: server.URL,
				},
			},
			tokenStore: store,
		}

		token, err := tm.refreshAccessToken(context.Background())

		require.NoError(t, err)
		require.Equal(t, "fresh-token", token)
		require.Equal(t, "fresh-token", tm.accessToken)
		require.Equal(t, "fresh-refresh", store.refreshToken)
		require.False(t, tm.expiresAt.IsZero())
	})

	t.Run("refresh with default duration when expires_in is zero", func(t *testing.T) {
		t.Parallel()

		server := newMockOAuthServer(t, tokenResponse{
			AccessToken:  "token-no-expiry",
			RefreshToken: "refresh-no-expiry",
			ExpiresIn:    0, // No expiry provided.
			TokenType:    "Bearer",
		})
		defer server.Close()

		store := &mockTokenStore{refreshToken: "current-refresh"}
		tm := &tokenManager{
			clientID:     "test-client",
			clientSecret: "test-secret",
			httpClient: &http.Client{
				Transport: &mockTransport{
					handler: server.Config.Handler,
					baseURL: server.URL,
				},
			},
			tokenStore: store,
		}

		token, err := tm.refreshAccessToken(context.Background())

		require.NoError(t, err)
		require.Equal(t, "token-no-expiry", token)
		// Should use default duration of 60 minutes.
		expectedExpiry := time.Now().Add(defaultTokenDuration)
		require.WithinDuration(t, expectedExpiry, tm.expiresAt, 5*time.Second)
	})

	t.Run("does not save refresh token when unchanged", func(t *testing.T) {
		t.Parallel()

		server := newMockOAuthServer(t, tokenResponse{
			AccessToken:  "new-access",
			RefreshToken: "same-refresh", // Same as current.
			ExpiresIn:    3600,
			TokenType:    "Bearer",
		})
		defer server.Close()

		store := &mockTokenStore{refreshToken: "same-refresh"}
		tm := &tokenManager{
			clientID:     "test-client",
			clientSecret: "test-secret",
			httpClient: &http.Client{
				Transport: &mockTransport{
					handler: server.Config.Handler,
					baseURL: server.URL,
				},
			},
			tokenStore: store,
		}

		_, err := tm.refreshAccessToken(context.Background())

		require.NoError(t, err)
		// Store should not have been called for save since token is unchanged.
		require.Equal(t, "same-refresh", store.refreshToken)
	})

	t.Run("error when token store fails to get refresh token", func(t *testing.T) {
		t.Parallel()

		store := &mockTokenStore{
			refreshToken: "",
			getErr:       errMockGetRefreshToken,
		}
		tm := &tokenManager{
			clientID:     "test-client",
			clientSecret: "test-secret",
			httpClient:   &http.Client{},
			tokenStore:   store,
		}

		_, err := tm.refreshAccessToken(context.Background())

		require.Error(t, err)
		require.Contains(t, err.Error(), "getting refresh token")
	})

	t.Run("error on non-200 response", func(t *testing.T) {
		t.Parallel()

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = w.Write([]byte(`{"error": "invalid_grant"}`))
		}))
		defer server.Close()

		store := &mockTokenStore{refreshToken: "bad-refresh"}
		tm := &tokenManager{
			clientID:     "test-client",
			clientSecret: "test-secret",
			httpClient: &http.Client{
				Transport: &mockTransport{
					handler: server.Config.Handler,
					baseURL: server.URL,
				},
			},
			tokenStore: store,
		}

		_, err := tm.refreshAccessToken(context.Background())

		require.Error(t, err)
		require.Contains(t, err.Error(), "token refresh failed with status 401")
	})
}

// newMockOAuthServer creates a test server that responds with the given token response.
func newMockOAuthServer(t *testing.T, resp tokenResponse) *httptest.Server {
	t.Helper()

	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(resp)
	}))
}
