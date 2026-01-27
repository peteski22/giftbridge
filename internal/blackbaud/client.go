package blackbaud

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
)

// Client is a Blackbaud SKY API client.
type Client struct {
	// baseURL is the base URL for API requests.
	baseURL string

	// config holds the client configuration.
	config Config

	// httpClient is the HTTP client for making requests.
	httpClient *http.Client

	// tokenManager handles OAuth token refresh.
	tokenManager *tokenManager
}

// CreateConstituent creates a new constituent and returns the new constituent ID.
func (c *Client) CreateConstituent(ctx context.Context, constituent *Constituent) (string, error) {
	reqURL := fmt.Sprintf("%s/constituent/v1/constituents", c.baseURL)

	var result createResponse
	if err := c.doRequest(ctx, http.MethodPost, reqURL, constituent, &result); err != nil {
		return "", fmt.Errorf("creating constituent: %w", err)
	}

	return result.ID, nil
}

// CreateGift creates a new gift and returns the new gift ID.
func (c *Client) CreateGift(ctx context.Context, gift *Gift) (string, error) {
	reqURL := fmt.Sprintf("%s/gift/v1/gifts", c.baseURL)

	var result createResponse
	if err := c.doRequest(ctx, http.MethodPost, reqURL, gift, &result); err != nil {
		return "", fmt.Errorf("creating gift: %w", err)
	}

	return result.ID, nil
}

// SearchConstituents searches for constituents matching the given email address.
func (c *Client) SearchConstituents(ctx context.Context, email string) ([]Constituent, error) {
	params := url.Values{}
	params.Set("search_text", email)

	reqURL := fmt.Sprintf("%s/constituent/v1/constituents/search?%s", c.baseURL, params.Encode())

	var result constituentSearchResponse
	if err := c.doRequest(ctx, http.MethodGet, reqURL, nil, &result); err != nil {
		return nil, fmt.Errorf("searching constituents: %w", err)
	}

	return result.Value, nil
}

// ListGiftsByConstituent returns all gifts for a constituent, optionally filtered by gift type.
// Handles pagination automatically to return all matching gifts.
func (c *Client) ListGiftsByConstituent(
	ctx context.Context,
	constituentID string,
	giftTypes []GiftType,
) ([]Gift, error) {
	params := url.Values{}
	params.Set("constituent_id", constituentID)
	for _, gt := range giftTypes {
		params.Add("gift_type", string(gt))
	}

	var allGifts []Gift
	reqURL := fmt.Sprintf("%s/gift/v1/gifts?%s", c.baseURL, params.Encode())

	for reqURL != "" {
		var result giftListResponse
		if err := c.doRequest(ctx, http.MethodGet, reqURL, nil, &result); err != nil {
			return nil, fmt.Errorf("listing gifts: %w", err)
		}

		allGifts = append(allGifts, result.Value...)
		reqURL = result.NextLink
	}

	return allGifts, nil
}

// UpdateGift updates an existing gift by ID.
func (c *Client) UpdateGift(ctx context.Context, giftID string, gift *Gift) error {
	reqURL := fmt.Sprintf("%s/gift/v1/gifts/%s", c.baseURL, giftID)

	if err := c.doRequest(ctx, http.MethodPatch, reqURL, gift, nil); err != nil {
		return fmt.Errorf("updating gift: %w", err)
	}

	return nil
}

// doRequest executes an HTTP request with authentication and JSON encoding.
func (c *Client) doRequest(ctx context.Context, method string, reqURL string, body any, result any) error {
	accessToken, err := c.tokenManager.AccessToken(ctx)
	if err != nil {
		return fmt.Errorf("getting access token: %w", err)
	}

	var reqBody io.Reader
	if body != nil {
		jsonBody, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("marshaling request body: %w", err)
		}
		reqBody = bytes.NewReader(jsonBody)
	}

	req, err := http.NewRequestWithContext(ctx, method, reqURL, reqBody)
	if err != nil {
		return fmt.Errorf("creating request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Bb-Api-Subscription-Key", c.config.SubscriptionKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("executing request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("unexpected status %d: %s", resp.StatusCode, string(respBody))
	}

	if result != nil {
		if err := json.NewDecoder(resp.Body).Decode(result); err != nil {
			return fmt.Errorf("decoding response: %w", err)
		}
	}

	return nil
}

// Config holds the required configuration for creating a Client.
type Config struct {
	// ClientID is the OAuth client identifier.
	ClientID string

	// ClientSecret is the OAuth client secret.
	ClientSecret string

	// SubscriptionKey is the SKY API subscription key.
	SubscriptionKey string

	// TokenStore provides access to OAuth tokens.
	TokenStore TokenStore
}

// validate checks that all required Config fields are set.
func (c *Config) validate() error {
	var errs []error
	if c.ClientID == "" {
		errs = append(errs, errors.New("client ID is required"))
	}
	if c.ClientSecret == "" {
		errs = append(errs, errors.New("client secret is required"))
	}
	if c.SubscriptionKey == "" {
		errs = append(errs, errors.New("subscription key is required"))
	}
	if c.TokenStore == nil {
		errs = append(errs, errors.New("token store is required"))
	}
	return errors.Join(errs...)
}

// NewClient creates a new Blackbaud SKY API client.
func NewClient(cfg Config, opts ...Option) (*Client, error) {
	if err := cfg.validate(); err != nil {
		return nil, fmt.Errorf("invalid config: %w", err)
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

	tm := newTokenManager(cfg.ClientID, cfg.ClientSecret, cfg.TokenStore, httpClient)

	return &Client{
		baseURL:      o.baseURL,
		config:       cfg,
		httpClient:   httpClient,
		tokenManager: tm,
	}, nil
}
