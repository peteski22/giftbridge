package fundraiseup

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestNewClient(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		apiKey  string
		errMsg  string
		wantErr bool
	}{
		"valid API key": {
			apiKey:  "test-api-key",
			wantErr: false,
		},
		"empty API key": {
			apiKey:  "",
			wantErr: true,
			errMsg:  "API key is required",
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			client, err := NewClient(tc.apiKey)

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

	tests := map[string]struct {
		errMsg      string
		expectedURL string
		opts        []Option
		wantErr     bool
	}{
		"default base URL": {
			opts:        nil,
			expectedURL: "https://api.fundraiseup.com/v1",
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

			client, err := NewClient("test-api-key", tc.opts...)

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

func TestClient_Donations(t *testing.T) {
	t.Parallel()

	t.Run("fetches single page of donations", func(t *testing.T) {
		t.Parallel()

		donations := []Donation{
			{ID: "don_1", Amount: 1000, Currency: "USD"},
			{ID: "don_2", Amount: 2000, Currency: "USD"},
		}

		server := newMockDonationsServer(t, []donationsResponse{
			{Data: donations, HasMore: false, NextCursor: ""},
		})
		defer server.Close()

		client, err := NewClient("test-key", WithBaseURL(server.URL))
		require.NoError(t, err)

		result, err := client.Donations(context.Background(), time.Now().Add(-24*time.Hour))

		require.NoError(t, err)
		require.Len(t, result, 2)
		require.Equal(t, "don_1", result[0].ID)
		require.Equal(t, "don_2", result[1].ID)
	})

	t.Run("fetches multiple pages of donations", func(t *testing.T) {
		t.Parallel()

		page1 := []Donation{{ID: "don_1", Amount: 1000}}
		page2 := []Donation{{ID: "don_2", Amount: 2000}}

		server := newMockDonationsServer(t, []donationsResponse{
			{Data: page1, HasMore: true, NextCursor: "cursor_1"},
			{Data: page2, HasMore: false, NextCursor: ""},
		})
		defer server.Close()

		client, err := NewClient("test-key", WithBaseURL(server.URL))
		require.NoError(t, err)

		result, err := client.Donations(context.Background(), time.Now().Add(-24*time.Hour))

		require.NoError(t, err)
		require.Len(t, result, 2)
		require.Equal(t, "don_1", result[0].ID)
		require.Equal(t, "don_2", result[1].ID)
	})

	t.Run("returns error on non-200 response", func(t *testing.T) {
		t.Parallel()

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = w.Write([]byte(`{"error": "unauthorized"}`))
		}))
		defer server.Close()

		client, err := NewClient("bad-key", WithBaseURL(server.URL))
		require.NoError(t, err)

		_, err = client.Donations(context.Background(), time.Now())

		require.Error(t, err)
		require.Contains(t, err.Error(), "unexpected status 401")
	})
}

func TestClient_Supporter(t *testing.T) {
	t.Parallel()

	t.Run("fetches supporter by ID", func(t *testing.T) {
		t.Parallel()

		supporter := Supporter{
			ID:        "sup_123",
			Email:     "test@example.com",
			FirstName: "John",
			LastName:  "Doe",
		}

		server := newMockSupporterServer(t, supporter)
		defer server.Close()

		client, err := NewClient("test-key", WithBaseURL(server.URL))
		require.NoError(t, err)

		result, err := client.Supporter(context.Background(), "sup_123")

		require.NoError(t, err)
		require.NotNil(t, result)
		require.Equal(t, "sup_123", result.ID)
		require.Equal(t, "test@example.com", result.Email)
		require.Equal(t, "John", result.FirstName)
		require.Equal(t, "Doe", result.LastName)
	})

	t.Run("returns error on non-200 response", func(t *testing.T) {
		t.Parallel()

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`{"error": "not found"}`))
		}))
		defer server.Close()

		client, err := NewClient("test-key", WithBaseURL(server.URL))
		require.NoError(t, err)

		_, err = client.Supporter(context.Background(), "nonexistent")

		require.Error(t, err)
		require.Contains(t, err.Error(), "unexpected status 404")
	})
}

// newMockDonationsServer creates a test server that returns paginated donation responses.
func newMockDonationsServer(t *testing.T, pages []donationsResponse) *httptest.Server {
	t.Helper()

	pageIndex := 0
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}

		if pageIndex >= len(pages) {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(pages[pageIndex])
		pageIndex++
	}))
}

// newMockSupporterServer creates a test server that returns a supporter.
func newMockSupporterServer(t *testing.T, supporter Supporter) *httptest.Server {
	t.Helper()

	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(supporter)
	}))
}
