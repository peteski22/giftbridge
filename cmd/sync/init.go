package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/peteski22/giftbridge/internal/config"
)

const configTemplate = `# GiftBridge Configuration
# See docs/authentication.md for setup instructions.

blackbaud:
  # From Blackbaud Developer Portal -> My Applications.
  client_id: ""
  client_secret: ""
  # From Blackbaud Developer Portal -> My Subscriptions.
  subscription_key: ""

fundraiseup:
  # From FundraiseUp Dashboard -> Settings -> API keys.
  api_key: ""

gift:
  # Required: Raiser's Edge Fund ID.
  fund_id: ""
  # Optional: Campaign and Appeal IDs.
  campaign_id: ""
  appeal_id: ""
  # Gift type (default: Donation).
  type: "Donation"
`

// runInit creates a sample configuration file.
func runInit() error {
	configDir, err := config.ConfigDir()
	if err != nil {
		return fmt.Errorf("getting config directory: %w", err)
	}

	configPath, err := config.ConfigFilePath()
	if err != nil {
		return fmt.Errorf("getting config path: %w", err)
	}

	if _, err := os.Stat(configPath); err == nil {
		return fmt.Errorf("config file already exists: %s", configPath)
	}

	if err := os.MkdirAll(configDir, 0o700); err != nil {
		return fmt.Errorf("creating config directory: %w", err)
	}

	if err := os.WriteFile(configPath, []byte(configTemplate), 0o600); err != nil {
		return fmt.Errorf("writing config file: %w", err)
	}

	fmt.Println("Created config file:", configPath)
	fmt.Println()
	fmt.Println("Next steps:")
	fmt.Println("  1. Edit the config file with your credentials")
	fmt.Println("  2. Run 'giftbridge auth' to authorize with Blackbaud")
	fmt.Println("  3. Run 'giftbridge --dry-run --since=2024-01-01T00:00:00Z' to test")

	tokenPath := filepath.Join(configDir, "token")
	fmt.Println()
	fmt.Printf("Token will be stored at: %s\n", tokenPath)

	return nil
}
