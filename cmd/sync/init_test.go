package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestConfigTemplate(t *testing.T) {
	t.Parallel()

	// Verify the config template contains expected sections.
	require.Contains(t, configTemplate, "blackbaud:")
	require.Contains(t, configTemplate, "client_id:")
	require.Contains(t, configTemplate, "client_secret:")
	require.Contains(t, configTemplate, "subscription_key:")
	require.Contains(t, configTemplate, "fundraiseup:")
	require.Contains(t, configTemplate, "api_key:")
	require.Contains(t, configTemplate, "gift:")
	require.Contains(t, configTemplate, "fund_id:")
	require.Contains(t, configTemplate, "campaign_id:")
	require.Contains(t, configTemplate, "appeal_id:")
	require.Contains(t, configTemplate, "type:")
}

func TestRunInitCreatesConfig(t *testing.T) {
	// Cannot use t.Parallel() with t.Setenv().

	// Create a temp directory to act as home.
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)

	err := runInit()
	require.NoError(t, err)

	// Check config file was created.
	configPath := filepath.Join(tmpHome, ".giftbridge", "config.yaml")
	data, err := os.ReadFile(configPath)
	require.NoError(t, err)
	require.Equal(t, configTemplate, string(data))

	// Check file permissions (0600).
	info, err := os.Stat(configPath)
	require.NoError(t, err)
	require.Equal(t, os.FileMode(0o600), info.Mode().Perm())

	// Check directory permissions (0700).
	dirInfo, err := os.Stat(filepath.Join(tmpHome, ".giftbridge"))
	require.NoError(t, err)
	require.Equal(t, os.FileMode(0o700), dirInfo.Mode().Perm())
}

func TestRunInitFailsIfConfigExists(t *testing.T) {
	// Cannot use t.Parallel() with t.Setenv().

	// Create a temp directory with existing config.
	tmpHome := t.TempDir()
	configDir := filepath.Join(tmpHome, ".giftbridge")
	require.NoError(t, os.MkdirAll(configDir, 0o700))
	configPath := filepath.Join(configDir, "config.yaml")
	require.NoError(t, os.WriteFile(configPath, []byte("existing config"), 0o600))

	t.Setenv("HOME", tmpHome)

	err := runInit()

	require.Error(t, err)
	require.Contains(t, err.Error(), "config file already exists")
}

func TestRunInitCreatesDirectory(t *testing.T) {
	// Cannot use t.Parallel() with t.Setenv().

	// Create a temp home without .giftbridge directory.
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)

	// Verify directory doesn't exist yet.
	configDir := filepath.Join(tmpHome, ".giftbridge")
	_, err := os.Stat(configDir)
	require.True(t, os.IsNotExist(err))

	err = runInit()
	require.NoError(t, err)

	// Verify directory was created.
	info, err := os.Stat(configDir)
	require.NoError(t, err)
	require.True(t, info.IsDir())
}
