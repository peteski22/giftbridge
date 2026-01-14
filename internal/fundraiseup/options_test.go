package fundraiseup

import (
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestDefaultOptions(t *testing.T) {
	t.Parallel()

	opts := defaultOptions()

	require.Equal(t, "https://api.fundraiseup.com/v1", opts.baseURL)
	require.Equal(t, 30*time.Second, opts.timeout)
	require.Nil(t, opts.httpClient)
}

func TestWithBaseURL(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		baseURL  string
		expected string
		wantErr  bool
	}{
		"valid URL": {
			baseURL:  "https://custom.api.com",
			expected: "https://custom.api.com",
			wantErr:  false,
		},
		"URL with whitespace": {
			baseURL:  "  https://trimmed.api.com  ",
			expected: "https://trimmed.api.com",
			wantErr:  false,
		},
		"empty URL": {
			baseURL: "",
			wantErr: true,
		},
		"whitespace only": {
			baseURL: "   ",
			wantErr: true,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			opts := defaultOptions()
			err := WithBaseURL(tc.baseURL)(opts)

			if tc.wantErr {
				require.Error(t, err)
				require.Contains(t, err.Error(), "base URL cannot be empty")
			} else {
				require.NoError(t, err)
				require.Equal(t, tc.expected, opts.baseURL)
			}
		})
	}
}

func TestWithHTTPClient(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		client  *http.Client
		wantErr bool
	}{
		"valid client": {
			client:  &http.Client{Timeout: 60 * time.Second},
			wantErr: false,
		},
		"nil client": {
			client:  nil,
			wantErr: true,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			opts := defaultOptions()
			err := WithHTTPClient(tc.client)(opts)

			if tc.wantErr {
				require.Error(t, err)
				require.Contains(t, err.Error(), "HTTP client cannot be nil")
			} else {
				require.NoError(t, err)
				require.Equal(t, tc.client, opts.httpClient)
			}
		})
	}
}

func TestWithTimeout(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		expected time.Duration
		timeout  time.Duration
		wantErr  bool
	}{
		"valid timeout": {
			timeout:  60 * time.Second,
			expected: 60 * time.Second,
			wantErr:  false,
		},
		"zero timeout": {
			timeout: 0,
			wantErr: true,
		},
		"negative timeout": {
			timeout: -1 * time.Second,
			wantErr: true,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			opts := defaultOptions()
			err := WithTimeout(tc.timeout)(opts)

			if tc.wantErr {
				require.Error(t, err)
				require.Contains(t, err.Error(), "timeout must be positive")
			} else {
				require.NoError(t, err)
				require.Equal(t, tc.expected, opts.timeout)
			}
		})
	}
}
