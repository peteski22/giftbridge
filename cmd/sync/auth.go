package main

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"html"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"github.com/peteski22/giftbridge/internal/config"
	"github.com/peteski22/giftbridge/internal/storage"
)

const (
	authTimeout     = 5 * time.Minute
	authURL         = "https://app.blackbaud.com/oauth/authorize"
	callbackPath    = "/callback"
	callbackPort    = "8080"
	httpTimeout     = 30 * time.Second
	stateByteLength = 32
	tokenURL        = "https://oauth2.sky.blackbaud.com/token"
)

// oauthErrorResponse represents an OAuth error from the Blackbaud token endpoint.
//
//nolint:tagliatelle // External API uses snake_case.
type oauthErrorResponse struct {
	Description string `json:"error_description"`
	Error       string `json:"error"`
}

// tokenExchangeRequest contains the parameters for exchanging an authorization code.
type tokenExchangeRequest struct {
	ClientID     string
	ClientSecret string
	Code         string
	RedirectURI  string
	TokenURL     string
}

// tokenResponse represents the OAuth token response.
//
//nolint:tagliatelle // External API uses snake_case.
type tokenResponse struct {
	AccessToken  string `json:"access_token"`
	ExpiresIn    int    `json:"expires_in"`
	RefreshToken string `json:"refresh_token"`
	TokenType    string `json:"token_type"`
}

// buildBlackbaudAuthURL constructs the Blackbaud SKY API OAuth authorization URL.
func buildBlackbaudAuthURL(clientID string, redirectURI string, state string) string {
	params := url.Values{}
	params.Set("client_id", clientID)
	params.Set("redirect_uri", redirectURI)
	params.Set("response_type", "code")
	params.Set("state", state)

	return authURL + "?" + params.Encode()
}

// generateOAuthState generates a cryptographically secure random state for CSRF protection.
func generateOAuthState() (string, error) {
	b := make([]byte, stateByteLength)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("generating random bytes: %w", err)
	}
	return base64.URLEncoding.EncodeToString(b), nil
}

// buildBlackbaudTokenRequest constructs an HTTP request for the token exchange.
func buildBlackbaudTokenRequest(req tokenExchangeRequest) (*http.Request, error) {
	data := url.Values{}
	data.Set("client_id", req.ClientID)
	data.Set("client_secret", req.ClientSecret)
	data.Set("code", req.Code)
	data.Set("grant_type", "authorization_code")
	data.Set("redirect_uri", req.RedirectURI)

	httpReq, err := http.NewRequestWithContext(
		context.Background(),
		http.MethodPost,
		req.TokenURL,
		strings.NewReader(data.Encode()),
	)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	return httpReq, nil
}

// exchangeBlackbaudCode exchanges a Blackbaud authorization code for OAuth tokens.
func exchangeBlackbaudCode(req tokenExchangeRequest) (*tokenResponse, error) {
	httpReq, err := buildBlackbaudTokenRequest(req)
	if err != nil {
		return nil, err
	}

	client := &http.Client{Timeout: httpTimeout}
	resp, err := client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("executing request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		var errResp oauthErrorResponse
		if err := json.NewDecoder(resp.Body).Decode(&errResp); err == nil && errResp.Error != "" {
			return nil, fmt.Errorf("%s: %s", errResp.Error, errResp.Description)
		}
		return nil, fmt.Errorf("unexpected status: %d", resp.StatusCode)
	}

	var tokens tokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&tokens); err != nil {
		return nil, fmt.Errorf("decoding response: %w", err)
	}

	return &tokens, nil
}

// browserCommand returns the command and arguments to open a URL on the current OS.
func browserCommand(targetURL string) (string, []string) {
	switch runtime.GOOS {
	case "darwin":
		return "open", []string{targetURL}
	case "windows":
		return "rundll32", []string{"url.dll,FileProtocolHandler", targetURL}
	default:
		return "xdg-open", []string{targetURL}
	}
}

// openBrowser opens the default web browser to the specified URL.
func openBrowser(targetURL string) error {
	name, args := browserCommand(targetURL)
	cmd := exec.Command(name, args...)
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout

	return cmd.Start()
}

// runBlackbaudAuth performs the Blackbaud SKY API OAuth authorization flow.
// It starts a local server, opens the browser for user consent, and saves the refresh token.
func runBlackbaudAuth() error {
	fmt.Println("=== Blackbaud Authorization ===")
	fmt.Println()

	cfg, err := config.LoadLocal()
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	tokenPath, err := config.TokenFilePath()
	if err != nil {
		return fmt.Errorf("getting token path: %w", err)
	}

	// Generate state for CSRF protection.
	state, err := generateOAuthState()
	if err != nil {
		return fmt.Errorf("generating OAuth state: %w", err)
	}

	codeChan := make(chan string, 1)
	errChan := make(chan error, 1)

	server, err := startOAuthCallbackServer(codeChan, errChan, state)
	if err != nil {
		return fmt.Errorf("starting callback server: %w", err)
	}
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = server.Shutdown(ctx)
	}()

	redirectURI := fmt.Sprintf("http://localhost:%s%s", callbackPort, callbackPath)
	authURLWithParams := buildBlackbaudAuthURL(cfg.Blackbaud.ClientID, redirectURI, state)

	fmt.Println("Opening browser for Blackbaud authorization...")
	fmt.Println()
	fmt.Println("If the browser doesn't open, visit this URL:")
	fmt.Println(authURLWithParams)
	fmt.Println()

	if err := openBrowser(authURLWithParams); err != nil {
		fmt.Printf("Could not open browser: %s\n", err)
	}

	fmt.Println("Waiting for authorization...")

	var code string
	select {
	case code = <-codeChan:
		// Success.
	case err := <-errChan:
		return fmt.Errorf("authorization failed: %w", err)
	case <-time.After(authTimeout):
		return fmt.Errorf("authorization timed out after %s", authTimeout)
	}

	fmt.Println()
	fmt.Println("Authorization received, exchanging for tokens...")

	tokens, err := exchangeBlackbaudCode(tokenExchangeRequest{
		ClientID:     cfg.Blackbaud.ClientID,
		ClientSecret: cfg.Blackbaud.ClientSecret,
		Code:         code,
		RedirectURI:  redirectURI,
		TokenURL:     tokenURL,
	})
	if err != nil {
		return fmt.Errorf("exchanging code for tokens: %w", err)
	}

	tokenStore, err := storage.NewFileTokenStore(tokenPath)
	if err != nil {
		return fmt.Errorf("creating token store: %w", err)
	}

	if err := tokenStore.SaveRefreshToken(context.Background(), tokens.RefreshToken); err != nil {
		return fmt.Errorf("saving refresh token: %w", err)
	}

	fmt.Println()
	fmt.Println("Authorization successful!")
	fmt.Printf("Refresh token saved to: %s\n", tokenPath)
	fmt.Println()
	fmt.Println("You can now run:")
	fmt.Println("  giftbridge --dry-run --since=2024-01-01T00:00:00Z")

	return nil
}

// writeCallbackResponse writes an HTML response for the OAuth callback page.
// It escapes the title and message to prevent XSS attacks.
func writeCallbackResponse(w http.ResponseWriter, title string, message string) {
	w.Header().Set("Content-Type", "text/html")
	_, _ = fmt.Fprintf(
		w,
		`<html><body><h1>%s</h1><p>%s</p><p>You can close this window.</p></body></html>`,
		html.EscapeString(title),
		html.EscapeString(message),
	)
}

// startOAuthCallbackServer starts a local HTTP server to receive the Blackbaud OAuth callback.
// It sends the authorization code or error through the provided channels.
// The expectedState parameter is used for CSRF protection - the callback must include a matching state.
func startOAuthCallbackServer(
	codeChan chan<- string,
	errChan chan<- error,
	expectedState string,
) (*http.Server, error) {
	listener, err := net.Listen("tcp", ":"+callbackPort)
	if err != nil {
		return nil, fmt.Errorf("port %s is already in use", callbackPort)
	}

	mux := http.NewServeMux()
	mux.HandleFunc(callbackPath, func(w http.ResponseWriter, r *http.Request) {
		code := r.URL.Query().Get("code")
		errDesc := r.URL.Query().Get("error_description")
		errMsg := r.URL.Query().Get("error")
		state := r.URL.Query().Get("state")

		if errMsg != "" {
			errChan <- fmt.Errorf("%s: %s", errMsg, errDesc)
			writeCallbackResponse(w, "Authorization Failed", fmt.Sprintf("%s: %s", errMsg, errDesc))
			return
		}

		if code == "" {
			errChan <- fmt.Errorf("no authorization code received")
			writeCallbackResponse(w, "Authorization Failed", "No authorization code received.")
			return
		}

		// Verify state parameter for CSRF protection.
		if expectedState != "" && state != expectedState {
			errChan <- fmt.Errorf("state mismatch: possible CSRF attack")
			writeCallbackResponse(w, "Authorization Failed", "State validation failed.")
			return
		}

		codeChan <- code
		writeCallbackResponse(w, "Authorization Successful", "You can return to the terminal.")
	})

	server := &http.Server{
		Handler:           mux,
		ReadHeaderTimeout: 10 * time.Second,
	}

	go func() {
		if err := server.Serve(listener); err != nil && err != http.ErrServerClosed {
			errChan <- fmt.Errorf("server error: %w", err)
		}
	}()

	return server, nil
}
