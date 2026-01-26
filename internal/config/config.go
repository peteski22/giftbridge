// Package config provides configuration loading from environment variables.
package config

import (
	"errors"
	"fmt"
	"os"
	"strings"
)

const (
	// EnvBlackbaudAPIBaseURL is the base URL for the Blackbaud SKY API.
	EnvBlackbaudAPIBaseURL = "BLACKBAUD_API_BASE_URL"

	// EnvBlackbaudClientID is the OAuth client ID for Blackbaud.
	EnvBlackbaudClientID = "BLACKBAUD_CLIENT_ID"

	// EnvBlackbaudClientSecret is the OAuth client secret for Blackbaud.
	EnvBlackbaudClientSecret = "BLACKBAUD_CLIENT_SECRET"

	// EnvBlackbaudEnvironmentID is the Blackbaud environment identifier.
	EnvBlackbaudEnvironmentID = "BLACKBAUD_ENVIRONMENT_ID"

	// EnvBlackbaudRefreshTokenSecretARN is the Secrets Manager ARN for the refresh token.
	EnvBlackbaudRefreshTokenSecretARN = "BLACKBAUD_REFRESH_TOKEN_SECRET_ARN"

	// EnvBlackbaudSubscriptionKey is the SKY API subscription key.
	EnvBlackbaudSubscriptionKey = "BLACKBAUD_SUBSCRIPTION_KEY"

	// EnvBlackbaudTokenURL is the OAuth token endpoint URL.
	EnvBlackbaudTokenURL = "BLACKBAUD_TOKEN_URL"

	// EnvDynamoDBIndexName is the DynamoDB Global Secondary Index for recurring donation queries.
	EnvDynamoDBIndexName = "DYNAMODB_INDEX_NAME"

	// EnvDynamoDBTableName is the DynamoDB table for tracking donations.
	EnvDynamoDBTableName = "DYNAMODB_TABLE_NAME"

	// EnvFundraiseUpAPIKey is the API key for FundraiseUp.
	EnvFundraiseUpAPIKey = "FUNDRAISEUP_API_KEY"

	// EnvFundraiseUpBaseURL is the base URL for the FundraiseUp API.
	EnvFundraiseUpBaseURL = "FUNDRAISEUP_BASE_URL"

	// EnvGiftAppealID is the Raiser's Edge Appeal ID for gifts.
	EnvGiftAppealID = "GIFT_APPEAL_ID"

	// EnvGiftCampaignID is the Raiser's Edge Campaign ID for gifts.
	EnvGiftCampaignID = "GIFT_CAMPAIGN_ID"

	// EnvGiftFundID is the Raiser's Edge Fund ID for gifts.
	EnvGiftFundID = "GIFT_FUND_ID"

	// EnvGiftType is the gift type in Raiser's Edge (default: Donation).
	EnvGiftType = "GIFT_TYPE"

	// EnvSSMParameterName is the SSM parameter storing the last sync timestamp.
	EnvSSMParameterName = "SSM_PARAMETER_NAME"
)

// Blackbaud holds Blackbaud SKY API configuration.
type Blackbaud struct {
	// APIBaseURL is the base URL for API requests.
	APIBaseURL string

	// ClientID is the OAuth client identifier.
	ClientID string

	// ClientSecret is the OAuth client secret.
	ClientSecret string

	// EnvironmentID is the Blackbaud environment identifier.
	EnvironmentID string

	// RefreshTokenSecretARN is the Secrets Manager ARN storing the OAuth refresh token.
	RefreshTokenSecretARN string

	// SubscriptionKey is the SKY API subscription key.
	SubscriptionKey string

	// TokenURL is the OAuth token endpoint.
	TokenURL string
}

// DynamoDB holds AWS DynamoDB configuration.
type DynamoDB struct {
	// IndexName is the Global Secondary Index name for querying donations by recurring ID.
	IndexName string

	// TableName is the name of the DynamoDB table for tracking donations.
	TableName string
}

// FundraiseUp holds FundraiseUp API configuration.
type FundraiseUp struct {
	// APIKey is the API key for authentication.
	APIKey string

	// BaseURL is the base URL for API requests.
	BaseURL string
}

// GiftDefaults holds default values applied to all gifts in Raiser's Edge.
type GiftDefaults struct {
	// AppealID is the Raiser's Edge Appeal to attribute gifts to (optional).
	AppealID string

	// CampaignID is the Raiser's Edge Campaign to attribute gifts to (optional).
	CampaignID string

	// FundID is the Raiser's Edge Fund where gifts are recorded (required).
	FundID string

	// Type is the type of gift in Raiser's Edge (default: Donation).
	Type string
}

// SSM holds AWS Systems Manager Parameter Store configuration.
type SSM struct {
	// ParameterName is the SSM parameter storing the last sync timestamp.
	ParameterName string
}

// Settings holds all configuration for the application.
type Settings struct {
	// Blackbaud contains Blackbaud SKY API settings.
	Blackbaud Blackbaud

	// DynamoDB contains AWS DynamoDB settings.
	DynamoDB DynamoDB

	// FundraiseUp contains FundraiseUp API settings.
	FundraiseUp FundraiseUp

	// GiftDefaults contains default values for gifts in Raiser's Edge.
	GiftDefaults GiftDefaults

	// SSM contains AWS Systems Manager Parameter Store settings.
	SSM SSM
}

func (s *Settings) validate() error {
	var errs []error

	if s.Blackbaud.ClientID == "" {
		errs = append(errs, requiredError(EnvBlackbaudClientID))
	}
	if s.Blackbaud.ClientSecret == "" {
		errs = append(errs, requiredError(EnvBlackbaudClientSecret))
	}
	if s.Blackbaud.EnvironmentID == "" {
		errs = append(errs, requiredError(EnvBlackbaudEnvironmentID))
	}
	if s.Blackbaud.RefreshTokenSecretARN == "" {
		errs = append(errs, requiredError(EnvBlackbaudRefreshTokenSecretARN))
	}
	if s.Blackbaud.SubscriptionKey == "" {
		errs = append(errs, requiredError(EnvBlackbaudSubscriptionKey))
	}
	if s.DynamoDB.TableName == "" {
		errs = append(errs, requiredError(EnvDynamoDBTableName))
	}
	if s.FundraiseUp.APIKey == "" {
		errs = append(errs, requiredError(EnvFundraiseUpAPIKey))
	}
	if s.GiftDefaults.FundID == "" {
		errs = append(errs, requiredError(EnvGiftFundID))
	}
	if s.SSM.ParameterName == "" {
		errs = append(errs, requiredError(EnvSSMParameterName))
	}

	return errors.Join(errs...)
}

// Load reads configuration from environment variables.
func Load() (*Settings, error) {
	cfg := &Settings{
		Blackbaud: Blackbaud{
			APIBaseURL:            envOrDefault(EnvBlackbaudAPIBaseURL, "https://api.sky.blackbaud.com"),
			ClientID:              strings.TrimSpace(os.Getenv(EnvBlackbaudClientID)),
			ClientSecret:          strings.TrimSpace(os.Getenv(EnvBlackbaudClientSecret)),
			EnvironmentID:         strings.TrimSpace(os.Getenv(EnvBlackbaudEnvironmentID)),
			RefreshTokenSecretARN: strings.TrimSpace(os.Getenv(EnvBlackbaudRefreshTokenSecretARN)),
			SubscriptionKey:       strings.TrimSpace(os.Getenv(EnvBlackbaudSubscriptionKey)),
			TokenURL:              envOrDefault(EnvBlackbaudTokenURL, "https://oauth2.sky.blackbaud.com/token"),
		},
		DynamoDB: DynamoDB{
			IndexName: envOrDefault(EnvDynamoDBIndexName, "RecurringIdIndex"),
			TableName: strings.TrimSpace(os.Getenv(EnvDynamoDBTableName)),
		},
		FundraiseUp: FundraiseUp{
			APIKey:  strings.TrimSpace(os.Getenv(EnvFundraiseUpAPIKey)),
			BaseURL: envOrDefault(EnvFundraiseUpBaseURL, "https://api.fundraiseup.com/v1"),
		},
		GiftDefaults: GiftDefaults{
			AppealID:   strings.TrimSpace(os.Getenv(EnvGiftAppealID)),
			CampaignID: strings.TrimSpace(os.Getenv(EnvGiftCampaignID)),
			FundID:     strings.TrimSpace(os.Getenv(EnvGiftFundID)),
			Type:       envOrDefault(EnvGiftType, "Donation"),
		},
		SSM: SSM{
			ParameterName: strings.TrimSpace(os.Getenv(EnvSSMParameterName)),
		},
	}

	if err := cfg.validate(); err != nil {
		return nil, err
	}

	return cfg, nil
}

func envOrDefault(key string, defaultValue string) string {
	if value := strings.TrimSpace(os.Getenv(key)); value != "" {
		return value
	}
	return defaultValue
}

func requiredError(envVar string) error {
	return fmt.Errorf("%s is required", envVar)
}
