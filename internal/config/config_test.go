package config

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestLoad(t *testing.T) {
	// Cannot use t.Parallel() with t.Setenv().
	tests := map[string]struct {
		envVars      map[string]string
		errFragments []string
		wantSettings *Settings
		wantErr      bool
	}{
		"all required vars set": {
			envVars: map[string]string{
				EnvBlackbaudClientID:              "client-id",
				EnvBlackbaudClientSecret:          "client-secret",
				EnvBlackbaudEnvironmentID:         "env-id",
				EnvBlackbaudRefreshTokenSecretARN: "arn:aws:secretsmanager:us-east-1:123456789012:secret:token",
				EnvBlackbaudSubscriptionKey:       "sub-key",
				EnvDynamoDBTableName:              "donations-table",
				EnvFundraiseUpAPIKey:              "fru-key",
				EnvGiftFundID:                     "fund-123",
				EnvSSMParameterName:               "/app/last-sync",
			},
			wantErr: false,
			wantSettings: &Settings{
				Blackbaud: Blackbaud{
					APIBaseURL:            "https://api.sky.blackbaud.com",
					ClientID:              "client-id",
					ClientSecret:          "client-secret",
					EnvironmentID:         "env-id",
					RefreshTokenSecretARN: "arn:aws:secretsmanager:us-east-1:123456789012:secret:token",
					SubscriptionKey:       "sub-key",
					TokenURL:              "https://oauth2.sky.blackbaud.com/token",
				},
				DynamoDB: DynamoDB{
					IndexName: "RecurringIdIndex",
					TableName: "donations-table",
				},
				FundraiseUp: FundraiseUp{
					APIKey:  "fru-key",
					BaseURL: "https://api.fundraiseup.com/v1",
				},
				GiftDefaults: GiftDefaults{
					FundID: "fund-123",
					Type:   "Donation",
				},
				SSM: SSM{
					ParameterName: "/app/last-sync",
				},
			},
		},
		"custom URLs and gift defaults": {
			envVars: map[string]string{
				EnvBlackbaudAPIBaseURL:            "https://custom.api.com",
				EnvBlackbaudClientID:              "client-id",
				EnvBlackbaudClientSecret:          "client-secret",
				EnvBlackbaudEnvironmentID:         "env-id",
				EnvBlackbaudRefreshTokenSecretARN: "arn:aws:secretsmanager:us-east-1:123456789012:secret:token",
				EnvBlackbaudSubscriptionKey:       "sub-key",
				EnvBlackbaudTokenURL:              "https://custom.token.com",
				EnvDynamoDBTableName:              "donations-table",
				EnvFundraiseUpAPIKey:              "fru-key",
				EnvFundraiseUpBaseURL:             "https://custom.fru.com",
				EnvGiftAppealID:                   "appeal-456",
				EnvGiftCampaignID:                 "campaign-789",
				EnvGiftFundID:                     "fund-123",
				EnvGiftType:                       "Grant",
				EnvSSMParameterName:               "/app/last-sync",
			},
			wantErr: false,
			wantSettings: &Settings{
				Blackbaud: Blackbaud{
					APIBaseURL:            "https://custom.api.com",
					ClientID:              "client-id",
					ClientSecret:          "client-secret",
					EnvironmentID:         "env-id",
					RefreshTokenSecretARN: "arn:aws:secretsmanager:us-east-1:123456789012:secret:token",
					SubscriptionKey:       "sub-key",
					TokenURL:              "https://custom.token.com",
				},
				DynamoDB: DynamoDB{
					IndexName: "RecurringIdIndex",
					TableName: "donations-table",
				},
				FundraiseUp: FundraiseUp{
					APIKey:  "fru-key",
					BaseURL: "https://custom.fru.com",
				},
				GiftDefaults: GiftDefaults{
					AppealID:   "appeal-456",
					CampaignID: "campaign-789",
					FundID:     "fund-123",
					Type:       "Grant",
				},
				SSM: SSM{
					ParameterName: "/app/last-sync",
				},
			},
		},
		"whitespace only values treated as empty": {
			envVars: map[string]string{
				EnvBlackbaudClientID:              "   ",
				EnvBlackbaudClientSecret:          "client-secret",
				EnvBlackbaudEnvironmentID:         "env-id",
				EnvBlackbaudRefreshTokenSecretARN: "arn:aws:secretsmanager:us-east-1:123456789012:secret:token",
				EnvBlackbaudSubscriptionKey:       "sub-key",
				EnvDynamoDBTableName:              "donations-table",
				EnvFundraiseUpAPIKey:              "fru-key",
				EnvGiftFundID:                     "fund-123",
				EnvSSMParameterName:               "/app/last-sync",
			},
			wantErr:      true,
			errFragments: []string{EnvBlackbaudClientID + " is required"},
		},
		"missing all required vars": {
			envVars: map[string]string{},
			wantErr: true,
			errFragments: []string{
				EnvBlackbaudClientID + " is required",
				EnvBlackbaudClientSecret + " is required",
				EnvBlackbaudEnvironmentID + " is required",
				EnvBlackbaudRefreshTokenSecretARN + " is required",
				EnvBlackbaudSubscriptionKey + " is required",
				EnvDynamoDBTableName + " is required",
				EnvFundraiseUpAPIKey + " is required",
				EnvGiftFundID + " is required",
				EnvSSMParameterName + " is required",
			},
		},
		"missing blackbaud client id": {
			envVars: map[string]string{
				EnvBlackbaudClientSecret:          "client-secret",
				EnvBlackbaudEnvironmentID:         "env-id",
				EnvBlackbaudRefreshTokenSecretARN: "arn:aws:secretsmanager:us-east-1:123456789012:secret:token",
				EnvBlackbaudSubscriptionKey:       "sub-key",
				EnvDynamoDBTableName:              "donations-table",
				EnvFundraiseUpAPIKey:              "fru-key",
				EnvGiftFundID:                     "fund-123",
				EnvSSMParameterName:               "/app/last-sync",
			},
			wantErr:      true,
			errFragments: []string{EnvBlackbaudClientID + " is required"},
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			// Set environment variables for this test.
			for k, v := range tc.envVars {
				t.Setenv(k, v)
			}

			settings, err := Load()

			if tc.wantErr {
				require.Error(t, err)
				for _, fragment := range tc.errFragments {
					require.Contains(t, err.Error(), fragment)
				}
				require.Nil(t, settings)
			} else {
				require.NoError(t, err)
				require.Equal(t, tc.wantSettings, settings)
			}
		})
	}
}

func TestEnvOrDefault(t *testing.T) {
	// Cannot use t.Parallel() with t.Setenv().
	tests := map[string]struct {
		defaultVal string
		envKey     string
		envVal     string
		setEnv     bool
		want       string
	}{
		"returns env value when set": {
			envKey:     "TEST_VAR",
			envVal:     "custom-value",
			setEnv:     true,
			defaultVal: "default-value",
			want:       "custom-value",
		},
		"returns default when not set": {
			envKey:     "TEST_VAR_UNSET",
			setEnv:     false,
			defaultVal: "default-value",
			want:       "default-value",
		},
		"returns default when empty": {
			envKey:     "TEST_VAR_EMPTY",
			envVal:     "",
			setEnv:     true,
			defaultVal: "default-value",
			want:       "default-value",
		},
		"trims whitespace": {
			envKey:     "TEST_VAR_WHITESPACE",
			envVal:     "  trimmed  ",
			setEnv:     true,
			defaultVal: "default-value",
			want:       "trimmed",
		},
		"returns default when only whitespace": {
			envKey:     "TEST_VAR_ONLY_WHITESPACE",
			envVal:     "   ",
			setEnv:     true,
			defaultVal: "default-value",
			want:       "default-value",
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			if tc.setEnv {
				t.Setenv(tc.envKey, tc.envVal)
			}

			got := envOrDefault(tc.envKey, tc.defaultVal)

			require.Equal(t, tc.want, got)
		})
	}
}
