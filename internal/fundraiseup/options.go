package fundraiseup

import (
	"fmt"
	"net/http"
	"strings"
	"time"
)

// Option configures optional Client settings.
type Option func(*options) error

// options holds optional configuration for creating a Client.
type options struct {
	// baseURL is the base URL for API requests.
	baseURL string

	// httpClient is a custom HTTP client.
	httpClient *http.Client

	// timeout is the HTTP client timeout.
	timeout time.Duration
}

// WithBaseURL sets a custom base URL for the API.
func WithBaseURL(baseURL string) Option {
	return func(o *options) error {
		baseURL = strings.TrimSpace(baseURL)
		if baseURL == "" {
			return fmt.Errorf("base URL cannot be empty")
		}
		o.baseURL = baseURL
		return nil
	}
}

// WithHTTPClient sets a custom HTTP client. Overrides WithTimeout.
func WithHTTPClient(httpClient *http.Client) Option {
	return func(o *options) error {
		if httpClient == nil {
			return fmt.Errorf("HTTP client cannot be nil")
		}
		o.httpClient = httpClient
		return nil
	}
}

// WithTimeout sets the HTTP client timeout.
func WithTimeout(timeout time.Duration) Option {
	return func(o *options) error {
		if timeout <= 0 {
			return fmt.Errorf("timeout must be positive, got %v", timeout)
		}
		o.timeout = timeout
		return nil
	}
}

// defaultOptions returns options with sensible defaults.
func defaultOptions() *options {
	return &options{
		baseURL: "https://api.fundraiseup.com/v1",
		timeout: 30 * time.Second,
	}
}
