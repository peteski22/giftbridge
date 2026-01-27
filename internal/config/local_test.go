package config

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

func TestConfigDir(t *testing.T) {
	t.Parallel()

	dir, err := ConfigDir()

	require.NoError(t, err)
	require.Contains(t, dir, ".giftbridge")
}

func TestConfigFilePath(t *testing.T) {
	t.Parallel()

	path, err := ConfigFilePath()

	require.NoError(t, err)
	require.Contains(t, path, ".giftbridge")
	require.Contains(t, path, "config.yaml")
}

func TestTokenFilePath(t *testing.T) {
	t.Parallel()

	path, err := TokenFilePath()

	require.NoError(t, err)
	require.Contains(t, path, ".giftbridge")
	require.Contains(t, path, "token")
}

func TestLocalConfigValidate(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		config       LocalConfig
		wantErr      bool
		errFragments []string
	}{
		"valid config": {
			config: LocalConfig{
				Blackbaud: localBlackbaudConfig{
					ClientID:        "client-id",
					ClientSecret:    "client-secret",
					SubscriptionKey: "sub-key",
				},
				FundraiseUp: localFundraiseUpConfig{
					APIKey: "api-key",
				},
				GiftDefaults: GiftDefaults{
					FundID: "fund-123",
					Type:   "Donation",
				},
			},
			wantErr: false,
		},
		"missing all required fields": {
			config:  LocalConfig{},
			wantErr: true,
			errFragments: []string{
				"blackbaud.client_id is required",
				"blackbaud.client_secret is required",
				"blackbaud.subscription_key is required",
				"fundraiseup.api_key is required",
				"gift.fund_id is required",
			},
		},
		"missing only fund_id": {
			config: LocalConfig{
				Blackbaud: localBlackbaudConfig{
					ClientID:        "client-id",
					ClientSecret:    "client-secret",
					SubscriptionKey: "sub-key",
				},
				FundraiseUp: localFundraiseUpConfig{
					APIKey: "api-key",
				},
			},
			wantErr:      true,
			errFragments: []string{"gift.fund_id is required"},
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

func TestLoadLocalFromFile(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		content     string
		wantErr     bool
		errContains string
		validateCfg func(t *testing.T, cfg *LocalConfig)
	}{
		"valid config file": {
			content: `
blackbaud:
  client_id: "test-client-id"
  client_secret: "test-client-secret"
  subscription_key: "test-sub-key"
fundraiseup:
  api_key: "test-api-key"
gift:
  fund_id: "fund-123"
  campaign_id: "campaign-456"
  appeal_id: "appeal-789"
  type: "Donation"
`,
			wantErr: false,
			validateCfg: func(t *testing.T, cfg *LocalConfig) {
				t.Helper()
				require.Equal(t, "test-client-id", cfg.Blackbaud.ClientID)
				require.Equal(t, "test-client-secret", cfg.Blackbaud.ClientSecret)
				require.Equal(t, "test-sub-key", cfg.Blackbaud.SubscriptionKey)
				require.Equal(t, "test-api-key", cfg.FundraiseUp.APIKey)
				require.Equal(t, "fund-123", cfg.GiftDefaults.FundID)
				require.Equal(t, "campaign-456", cfg.GiftDefaults.CampaignID)
				require.Equal(t, "appeal-789", cfg.GiftDefaults.AppealID)
				require.Equal(t, "Donation", cfg.GiftDefaults.Type)
			},
		},
		"defaults type to Donation when empty": {
			content: `
blackbaud:
  client_id: "test-client-id"
  client_secret: "test-client-secret"
  subscription_key: "test-sub-key"
fundraiseup:
  api_key: "test-api-key"
gift:
  fund_id: "fund-123"
`,
			wantErr: false,
			validateCfg: func(t *testing.T, cfg *LocalConfig) {
				t.Helper()
				require.Equal(t, "Donation", cfg.GiftDefaults.Type)
			},
		},
		"invalid yaml": {
			content:     `invalid: yaml: content: [}`,
			wantErr:     true,
			errContains: "parsing config",
		},
		"missing required fields": {
			content: `
blackbaud:
  client_id: "test-client-id"
`,
			wantErr:     true,
			errContains: "invalid config",
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			dir := t.TempDir()
			configPath := filepath.Join(dir, "config.yaml")
			require.NoError(t, os.WriteFile(configPath, []byte(tc.content), 0o600))

			cfg, err := loadLocalFromPath(configPath)

			if tc.wantErr {
				require.Error(t, err)
				if tc.errContains != "" {
					require.Contains(t, err.Error(), tc.errContains)
				}
			} else {
				require.NoError(t, err)
				require.NotNil(t, cfg)
				if tc.validateCfg != nil {
					tc.validateCfg(t, cfg)
				}
			}
		})
	}
}

func TestLoadLocalFileNotFound(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	configPath := filepath.Join(dir, "nonexistent.yaml")

	_, err := loadLocalFromPath(configPath)

	require.Error(t, err)
	require.Contains(t, err.Error(), "config file not found")
}

func TestLocalConfigExists(t *testing.T) {
	t.Parallel()

	// This test verifies the function doesn't panic.
	// Actual result depends on whether ~/.giftbridge/config.yaml exists.
	_ = LocalConfigExists()
}

// loadLocalFromPath loads config from a specific path for testing.
func loadLocalFromPath(configPath string) (*LocalConfig, error) {
	data, err := os.ReadFile(configPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("config file not found: %s (run 'giftbridge init' to create)", configPath)
		}
		return nil, fmt.Errorf("reading config file: %w", err)
	}

	var local localConfig
	if err := yaml.Unmarshal(data, &local); err != nil {
		return nil, fmt.Errorf("parsing config file: %w", err)
	}

	cfg := &LocalConfig{}
	cfg.Blackbaud.ClientID = local.Blackbaud.ClientID
	cfg.Blackbaud.ClientSecret = local.Blackbaud.ClientSecret
	cfg.Blackbaud.SubscriptionKey = local.Blackbaud.SubscriptionKey
	cfg.FundraiseUp.APIKey = local.FundraiseUp.APIKey
	cfg.GiftDefaults.AppealID = local.Gift.AppealID
	cfg.GiftDefaults.CampaignID = local.Gift.CampaignID
	cfg.GiftDefaults.FundID = local.Gift.FundID
	cfg.GiftDefaults.Type = local.Gift.Type

	if cfg.GiftDefaults.Type == "" {
		cfg.GiftDefaults.Type = "Donation"
	}

	if err := cfg.validate(); err != nil {
		return nil, fmt.Errorf("invalid config: %w", err)
	}

	return cfg, nil
}
