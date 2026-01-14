package blackbaud

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

const (
	// defaultTokenDuration is used when the API doesn't return an expiry time.
	defaultTokenDuration = 60 * time.Minute

	// tokenExpiryBuffer is the time before expiry to trigger a refresh.
	tokenExpiryBuffer = 5 * time.Minute

	// tokenURL is the Blackbaud OAuth token endpoint.
	tokenURL = "https://oauth2.sky.blackbaud.com/token"
)

// TokenStore provides access to OAuth tokens.
type TokenStore interface {
	// RefreshToken returns the current refresh token.
	RefreshToken(ctx context.Context) (string, error)

	// SaveRefreshToken saves a new refresh token.
	SaveRefreshToken(ctx context.Context, token string) error
}

// tokenManager handles OAuth token refresh and caching.
type tokenManager struct {
	// accessToken is the current cached access token.
	accessToken string

	// clientID is the OAuth client identifier.
	clientID string

	// clientSecret is the OAuth client secret.
	clientSecret string

	// expiresAt is when the current access token expires.
	expiresAt time.Time

	// httpClient is the HTTP client for token requests.
	httpClient *http.Client

	// mu protects access token state.
	mu sync.RWMutex

	// tokenStore provides access to refresh tokens.
	tokenStore TokenStore
}

// AccessToken returns a valid access token, refreshing if necessary.
func (tm *tokenManager) AccessToken(ctx context.Context) (string, error) {
	if token, ok := tm.cachedToken(); ok {
		return token, nil
	}
	return tm.refreshAccessToken(ctx)
}

// cachedToken returns the cached access token if valid, or false if refresh is needed.
func (tm *tokenManager) cachedToken() (string, bool) {
	tm.mu.RLock()
	defer tm.mu.RUnlock()

	if tm.isTokenValid() {
		return tm.accessToken, true
	}
	return "", false
}

// isTokenValid checks if the current access token is valid and not near expiry.
// Must be called with at least a read lock held.
func (tm *tokenManager) isTokenValid() bool {
	return tm.accessToken != "" && time.Now().Before(tm.expiresAt.Add(-tokenExpiryBuffer))
}

// refreshAccessToken fetches a new access token using the refresh token.
func (tm *tokenManager) refreshAccessToken(ctx context.Context) (string, error) {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	// Double-check after acquiring write lock.
	if tm.isTokenValid() {
		return tm.accessToken, nil
	}

	refreshToken, err := tm.tokenStore.RefreshToken(ctx)
	if err != nil {
		return "", fmt.Errorf("getting refresh token: %w", err)
	}

	data := url.Values{}
	data.Set("grant_type", "refresh_token")
	data.Set("refresh_token", refreshToken)
	data.Set("client_id", tm.clientID)
	data.Set("client_secret", tm.clientSecret)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, tokenURL, strings.NewReader(data.Encode()))
	if err != nil {
		return "", fmt.Errorf("creating token request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := tm.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("executing token request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("token refresh failed with status %d: %s", resp.StatusCode, string(body))
	}

	var tokenResp tokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
		return "", fmt.Errorf("decoding token response: %w", err)
	}

	// Save new refresh token if provided.
	if tokenResp.RefreshToken != "" && tokenResp.RefreshToken != refreshToken {
		if err := tm.tokenStore.SaveRefreshToken(ctx, tokenResp.RefreshToken); err != nil {
			return "", fmt.Errorf("saving refresh token: %w", err)
		}
	}

	tm.accessToken = tokenResp.AccessToken
	if tokenResp.ExpiresIn > 0 {
		tm.expiresAt = time.Now().Add(time.Duration(tokenResp.ExpiresIn) * time.Second)
	} else {
		tm.expiresAt = time.Now().Add(defaultTokenDuration)
	}

	return tm.accessToken, nil
}

// newTokenManager creates a new token manager for handling OAuth authentication.
func newTokenManager(
	clientID string,
	clientSecret string,
	tokenStore TokenStore,
	httpClient *http.Client,
) *tokenManager {
	return &tokenManager{
		clientID:     clientID,
		clientSecret: clientSecret,
		httpClient:   httpClient,
		tokenStore:   tokenStore,
	}
}
