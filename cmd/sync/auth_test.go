package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestBuildBlackbaudAuthURL(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		clientID       string
		redirectURI    string
		state          string
		wantContains   []string
		wantNotContain []string
	}{
		"standard values": {
			clientID:    "my-client-id",
			redirectURI: "http://localhost:8080/callback",
			state:       "test-state-123",
			wantContains: []string{
				"https://app.blackbaud.com/oauth/authorize",
				"client_id=my-client-id",
				"redirect_uri=http",
				"response_type=code",
				"state=test-state-123",
			},
		},
		"special characters in client ID": {
			clientID:    "client+id/special",
			redirectURI: "http://localhost:8080/callback",
			state:       "state-abc",
			wantContains: []string{
				"client_id=client",
			},
		},
		"empty values": {
			clientID:    "",
			redirectURI: "",
			state:       "",
			wantContains: []string{
				"client_id=",
				"redirect_uri=",
				"response_type=code",
				"state=",
			},
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			result := buildBlackbaudAuthURL(tc.clientID, tc.redirectURI, tc.state)

			for _, want := range tc.wantContains {
				require.Contains(t, result, want)
			}
			for _, notWant := range tc.wantNotContain {
				require.NotContains(t, result, notWant)
			}
		})
	}
}

func TestBuildBlackbaudAuthURLParseable(t *testing.T) {
	t.Parallel()

	result := buildBlackbaudAuthURL("test-client", "http://localhost:8080/callback", "test-state-xyz")

	parsed, err := url.Parse(result)
	require.NoError(t, err)
	require.Equal(t, "https", parsed.Scheme)
	require.Equal(t, "app.blackbaud.com", parsed.Host)
	require.Equal(t, "/oauth/authorize", parsed.Path)

	query := parsed.Query()
	require.Equal(t, "test-client", query.Get("client_id"))
	require.Equal(t, "http://localhost:8080/callback", query.Get("redirect_uri"))
	require.Equal(t, "code", query.Get("response_type"))
	require.Equal(t, "test-state-xyz", query.Get("state"))
}

func TestGenerateOAuthState(t *testing.T) {
	t.Parallel()

	state1, err := generateOAuthState()
	require.NoError(t, err)
	require.NotEmpty(t, state1)

	state2, err := generateOAuthState()
	require.NoError(t, err)
	require.NotEmpty(t, state2)

	// States should be different (cryptographically random).
	require.NotEqual(t, state1, state2)

	// State should be base64 URL encoded (no + or /).
	require.NotContains(t, state1, "+")
	require.NotContains(t, state1, "/")
}

func TestBuildBlackbaudTokenRequest(t *testing.T) {
	t.Parallel()

	req := tokenExchangeRequest{
		ClientID:     "test-client-id",
		ClientSecret: "test-secret",
		Code:         "auth-code-123",
		RedirectURI:  "http://localhost:8080/callback",
		TokenURL:     "https://oauth2.sky.blackbaud.com/token",
	}

	httpReq, err := buildBlackbaudTokenRequest(req)

	require.NoError(t, err)
	require.Equal(t, http.MethodPost, httpReq.Method)
	require.Equal(t, "https://oauth2.sky.blackbaud.com/token", httpReq.URL.String())
	require.Equal(t, "application/x-www-form-urlencoded", httpReq.Header.Get("Content-Type"))

	// Parse the body to verify form values.
	require.NoError(t, httpReq.ParseForm())
	require.Equal(t, "test-client-id", httpReq.FormValue("client_id"))
	require.Equal(t, "test-secret", httpReq.FormValue("client_secret"))
	require.Equal(t, "auth-code-123", httpReq.FormValue("code"))
	require.Equal(t, "authorization_code", httpReq.FormValue("grant_type"))
	require.Equal(t, "http://localhost:8080/callback", httpReq.FormValue("redirect_uri"))
}

func TestExchangeBlackbaudCode(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		serverHandler func(w http.ResponseWriter, r *http.Request)
		wantErr       bool
		errContains   string
		validateResp  func(t *testing.T, resp *tokenResponse)
	}{
		"successful token exchange": {
			serverHandler: func(w http.ResponseWriter, r *http.Request) {
				require.Equal(t, http.MethodPost, r.Method)
				require.Equal(t, "application/x-www-form-urlencoded", r.Header.Get("Content-Type"))

				err := r.ParseForm()
				require.NoError(t, err)
				require.Equal(t, "test-client-id", r.FormValue("client_id"))
				require.Equal(t, "test-client-secret", r.FormValue("client_secret"))
				require.Equal(t, "auth-code-123", r.FormValue("code"))
				require.Equal(t, "authorization_code", r.FormValue("grant_type"))
				require.Equal(t, "http://localhost:8080/callback", r.FormValue("redirect_uri"))

				w.Header().Set("Content-Type", "application/json")
				_ = json.NewEncoder(w).Encode(map[string]any{
					"access_token":  "access-token-xyz",
					"expires_in":    3600,
					"refresh_token": "refresh-token-abc",
					"token_type":    "Bearer",
				})
			},
			wantErr: false,
			validateResp: func(t *testing.T, resp *tokenResponse) {
				t.Helper()
				require.Equal(t, "access-token-xyz", resp.AccessToken)
				require.Equal(t, 3600, resp.ExpiresIn)
				require.Equal(t, "refresh-token-abc", resp.RefreshToken)
				require.Equal(t, "Bearer", resp.TokenType)
			},
		},
		"error response from server": {
			serverHandler: func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusBadRequest)
				w.Header().Set("Content-Type", "application/json")
				_ = json.NewEncoder(w).Encode(map[string]string{
					"error":             "invalid_grant",
					"error_description": "The authorization code has expired",
				})
			},
			wantErr:     true,
			errContains: "invalid_grant",
		},
		"non-JSON error response": {
			serverHandler: func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusInternalServerError)
				_, _ = w.Write([]byte("Internal Server Error"))
			},
			wantErr:     true,
			errContains: "unexpected status: 500",
		},
		"invalid JSON response": {
			serverHandler: func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte("{invalid json"))
			},
			wantErr:     true,
			errContains: "decoding response",
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			server := httptest.NewServer(http.HandlerFunc(tc.serverHandler))
			defer server.Close()

			resp, err := exchangeBlackbaudCode(tokenExchangeRequest{
				ClientID:     "test-client-id",
				ClientSecret: "test-client-secret",
				Code:         "auth-code-123",
				RedirectURI:  "http://localhost:8080/callback",
				TokenURL:     server.URL,
			})

			if tc.wantErr {
				require.Error(t, err)
				if tc.errContains != "" {
					require.Contains(t, err.Error(), tc.errContains)
				}
			} else {
				require.NoError(t, err)
				require.NotNil(t, resp)
				if tc.validateResp != nil {
					tc.validateResp(t, resp)
				}
			}
		})
	}
}

func TestWriteCallbackResponse(t *testing.T) {
	t.Parallel()

	t.Run("normal content", func(t *testing.T) {
		t.Parallel()

		w := httptest.NewRecorder()

		writeCallbackResponse(w, "Test Title", "Test message here.")

		resp := w.Result()
		defer func() { _ = resp.Body.Close() }()

		require.Equal(t, "text/html", resp.Header.Get("Content-Type"))

		body := w.Body.String()
		require.Contains(t, body, "<h1>Test Title</h1>")
		require.Contains(t, body, "<p>Test message here.</p>")
		require.Contains(t, body, "You can close this window.")
	})

	t.Run("escapes HTML in title and message", func(t *testing.T) {
		t.Parallel()

		w := httptest.NewRecorder()

		writeCallbackResponse(w, "<script>alert('xss')</script>", "Test <b>bold</b> & \"quoted\"")

		body := w.Body.String()
		// Should be escaped, not raw HTML.
		require.Contains(t, body, "&lt;script&gt;alert(&#39;xss&#39;)&lt;/script&gt;")
		require.Contains(t, body, "&lt;b&gt;bold&lt;/b&gt;")
		require.Contains(t, body, "&amp;")
		require.Contains(t, body, "&#34;quoted&#34;")
		// Should NOT contain raw script tags.
		require.NotContains(t, body, "<script>")
	})
}

func TestBrowserCommand(t *testing.T) {
	t.Parallel()

	testURL := "https://example.com/auth"
	name, args := browserCommand(testURL)

	// Verify command is set based on OS.
	require.NotEmpty(t, name)
	require.NotEmpty(t, args)

	// URL should be in the args.
	found := false
	for _, arg := range args {
		if arg == testURL {
			found = true
			break
		}
	}
	require.True(t, found, "URL should be in command arguments")
}

func TestStartOAuthCallbackServer(t *testing.T) {
	// Cannot use t.Parallel() because subtests share the same port 8080.

	t.Run("successful authorization callback with state", func(t *testing.T) {
		// Cannot use t.Parallel() - port 8080 conflict.

		codeChan := make(chan string, 1)
		errChan := make(chan error, 1)
		expectedState := "test-state-123"

		server, err := startOAuthCallbackServer(codeChan, errChan, expectedState)
		require.NoError(t, err)
		defer func() {
			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()
			_ = server.Shutdown(ctx)
		}()

		// Make a request with a valid code and matching state.
		resp, err := http.Get("http://localhost:8080/callback?code=test-auth-code&state=test-state-123")
		require.NoError(t, err)
		defer func() { _ = resp.Body.Close() }()

		require.Equal(t, http.StatusOK, resp.StatusCode)

		select {
		case code := <-codeChan:
			require.Equal(t, "test-auth-code", code)
		case err := <-errChan:
			t.Fatalf("unexpected error: %v", err)
		case <-time.After(time.Second):
			t.Fatal("timeout waiting for code")
		}
	})

	t.Run("state mismatch rejected", func(t *testing.T) {
		// Cannot use t.Parallel() - port 8080 conflict.

		codeChan := make(chan string, 1)
		errChan := make(chan error, 1)
		expectedState := "expected-state"

		server, err := startOAuthCallbackServer(codeChan, errChan, expectedState)
		require.NoError(t, err)
		defer func() {
			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()
			_ = server.Shutdown(ctx)
		}()

		// Make a request with wrong state.
		resp, err := http.Get("http://localhost:8080/callback?code=test-auth-code&state=wrong-state")
		require.NoError(t, err)
		defer func() { _ = resp.Body.Close() }()

		require.Equal(t, http.StatusOK, resp.StatusCode)

		select {
		case <-codeChan:
			t.Fatal("unexpected code received")
		case err := <-errChan:
			require.Contains(t, err.Error(), "state mismatch")
		case <-time.After(time.Second):
			t.Fatal("timeout waiting for error")
		}
	})

	t.Run("error callback", func(t *testing.T) {
		// Cannot use t.Parallel() - port 8080 conflict.

		codeChan := make(chan string, 1)
		errChan := make(chan error, 1)

		server, err := startOAuthCallbackServer(codeChan, errChan, "")
		require.NoError(t, err)
		defer func() {
			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()
			_ = server.Shutdown(ctx)
		}()

		// Make a request with an error.
		resp, err := http.Get(
			"http://localhost:8080/callback?error=access_denied&error_description=User%20denied%20access",
		)
		require.NoError(t, err)
		defer func() { _ = resp.Body.Close() }()

		require.Equal(t, http.StatusOK, resp.StatusCode)

		select {
		case <-codeChan:
			t.Fatal("unexpected code received")
		case err := <-errChan:
			require.Contains(t, err.Error(), "access_denied")
			require.Contains(t, err.Error(), "User denied access")
		case <-time.After(time.Second):
			t.Fatal("timeout waiting for error")
		}
	})

	t.Run("missing code callback", func(t *testing.T) {
		// Cannot use t.Parallel() - port 8080 conflict.

		codeChan := make(chan string, 1)
		errChan := make(chan error, 1)

		server, err := startOAuthCallbackServer(codeChan, errChan, "")
		require.NoError(t, err)
		defer func() {
			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()
			_ = server.Shutdown(ctx)
		}()

		// Make a request without code or error.
		resp, err := http.Get("http://localhost:8080/callback")
		require.NoError(t, err)
		defer func() { _ = resp.Body.Close() }()

		require.Equal(t, http.StatusOK, resp.StatusCode)

		select {
		case <-codeChan:
			t.Fatal("unexpected code received")
		case err := <-errChan:
			require.Contains(t, err.Error(), "no authorization code")
		case <-time.After(time.Second):
			t.Fatal("timeout waiting for error")
		}
	})
}
