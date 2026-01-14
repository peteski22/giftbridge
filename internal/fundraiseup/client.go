package fundraiseup

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

// Client is a FundraiseUp API client.
type Client struct {
	// apiKey is the API key for authentication.
	apiKey string

	// baseURL is the base URL for API requests.
	baseURL string

	// httpClient is the HTTP client for making requests.
	httpClient *http.Client
}

// Donations fetches donations created or updated after the given time.
func (c *Client) Donations(ctx context.Context, since time.Time) ([]Donation, error) {
	var allDonations []Donation
	var cursor string

	for {
		donations, nextCursor, err := c.fetchDonationsPage(ctx, since, cursor)
		if err != nil {
			return nil, err
		}
		allDonations = append(allDonations, donations...)

		if nextCursor == "" {
			break
		}
		cursor = nextCursor
	}

	return allDonations, nil
}

// Supporter fetches a supporter by ID.
func (c *Client) Supporter(ctx context.Context, supporterID string) (*Supporter, error) {
	reqURL := fmt.Sprintf("%s/supporters/%s", c.baseURL, supporterID)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("executing request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("unexpected status %d: %s", resp.StatusCode, string(body))
	}

	var supporter Supporter
	if err := json.NewDecoder(resp.Body).Decode(&supporter); err != nil {
		return nil, fmt.Errorf("decoding response: %w", err)
	}

	return &supporter, nil
}

// fetchDonationsPage fetches a single page of donations from the API.
func (c *Client) fetchDonationsPage(ctx context.Context, since time.Time, cursor string) ([]Donation, string, error) {
	params := url.Values{}
	params.Set("createdAfter", since.UTC().Format(time.RFC3339))
	params.Set("limit", "50")
	if cursor != "" {
		params.Set("cursor", cursor)
	}

	reqURL := fmt.Sprintf("%s/donations?%s", c.baseURL, params.Encode())

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, "", fmt.Errorf("creating request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, "", fmt.Errorf("executing request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, "", fmt.Errorf("unexpected status %d: %s", resp.StatusCode, string(body))
	}

	var result donationsResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, "", fmt.Errorf("decoding response: %w", err)
	}

	return result.Data, result.NextCursor, nil
}

// NewClient creates a new FundraiseUp API client.
func NewClient(apiKey string, opts ...Option) (*Client, error) {
	if apiKey == "" {
		return nil, errors.New("API key is required")
	}

	o := defaultOptions()
	for _, opt := range opts {
		if err := opt(o); err != nil {
			return nil, fmt.Errorf("applying option: %w", err)
		}
	}

	httpClient := o.httpClient
	if httpClient == nil {
		httpClient = &http.Client{Timeout: o.timeout}
	}

	return &Client{
		apiKey:     apiKey,
		baseURL:    o.baseURL,
		httpClient: httpClient,
	}, nil
}
