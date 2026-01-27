package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

const (
	configDirName  = ".giftbridge"
	configFileName = "config.yaml"
	defaultType    = "Donation"
	tokenFileName  = "token"
)

// LocalConfig holds configuration loaded from a local file.
type LocalConfig struct {
	Blackbaud    localBlackbaudConfig
	FundraiseUp  localFundraiseUpConfig
	GiftDefaults GiftDefaults
}

// localBlackbaud represents the blackbaud section of the config file.
type localBlackbaud struct {
	ClientID        string `yaml:"client_id"`
	ClientSecret    string `yaml:"client_secret"`
	SubscriptionKey string `yaml:"subscription_key"`
}

// localBlackbaudConfig holds Blackbaud credentials from the config file.
type localBlackbaudConfig struct {
	ClientID        string
	ClientSecret    string
	SubscriptionKey string
}

// localConfig represents the local configuration file structure.
type localConfig struct {
	Blackbaud   localBlackbaud   `yaml:"blackbaud"`
	FundraiseUp localFundraiseUp `yaml:"fundraiseup"`
	Gift        localGift        `yaml:"gift"`
}

// localFundraiseUp represents the fundraiseup section of the config file.
type localFundraiseUp struct {
	APIKey string `yaml:"api_key"`
}

// localFundraiseUpConfig holds FundraiseUp credentials from the config file.
type localFundraiseUpConfig struct {
	APIKey string
}

// localGift represents the gift section of the config file.
type localGift struct {
	AppealID   string `yaml:"appeal_id"`
	CampaignID string `yaml:"campaign_id"`
	FundID     string `yaml:"fund_id"`
	Type       string `yaml:"type"`
}

// ConfigDir returns the giftbridge configuration directory path.
func ConfigDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("getting home directory: %w", err)
	}
	return filepath.Join(home, configDirName), nil
}

// ConfigFilePath returns the path to the local config file.
func ConfigFilePath() (string, error) {
	dir, err := ConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, configFileName), nil
}

// LoadLocal loads configuration from the local config file.
func LoadLocal() (*LocalConfig, error) {
	configPath, err := ConfigFilePath()
	if err != nil {
		return nil, err
	}

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
		cfg.GiftDefaults.Type = defaultType
	}

	if err := cfg.validate(); err != nil {
		return nil, fmt.Errorf("invalid config: %w", err)
	}

	return cfg, nil
}

// LocalConfigExists checks if a local config file exists.
func LocalConfigExists() bool {
	configPath, err := ConfigFilePath()
	if err != nil {
		return false
	}
	_, err = os.Stat(configPath)
	return err == nil
}

// TokenFilePath returns the path to the local token file.
func TokenFilePath() (string, error) {
	dir, err := ConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, tokenFileName), nil
}

// validate checks that required fields are set.
func (c *LocalConfig) validate() error {
	var errs []error

	if c.Blackbaud.ClientID == "" {
		errs = append(errs, errors.New("blackbaud.client_id is required"))
	}
	if c.Blackbaud.ClientSecret == "" {
		errs = append(errs, errors.New("blackbaud.client_secret is required"))
	}
	if c.Blackbaud.SubscriptionKey == "" {
		errs = append(errs, errors.New("blackbaud.subscription_key is required"))
	}
	if c.FundraiseUp.APIKey == "" {
		errs = append(errs, errors.New("fundraiseup.api_key is required"))
	}
	if c.GiftDefaults.FundID == "" {
		errs = append(errs, errors.New("gift.fund_id is required"))
	}

	return errors.Join(errs...)
}
