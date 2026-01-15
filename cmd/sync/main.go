// Package main provides the Lambda handler entry point for giftbridge.
package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"

	"github.com/aws/aws-lambda-go/lambda"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/secretsmanager"
	"github.com/aws/aws-sdk-go-v2/service/ssm"

	"github.com/peteski22/giftbridge/internal/blackbaud"
	"github.com/peteski22/giftbridge/internal/config"
	"github.com/peteski22/giftbridge/internal/fundraiseup"
	"github.com/peteski22/giftbridge/internal/storage"
	"github.com/peteski22/giftbridge/internal/sync"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
	slog.SetDefault(logger)

	lambda.Start(handler)
}

func handler(ctx context.Context) error {
	slog.InfoContext(ctx, "starting sync")

	// Load configuration.
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	// Initialize AWS SDK.
	awsCfg, err := awsconfig.LoadDefaultConfig(ctx)
	if err != nil {
		return fmt.Errorf("loading AWS config: %w", err)
	}

	// Create AWS service clients.
	dynamoClient := dynamodb.NewFromConfig(awsCfg)
	secretsClient := secretsmanager.NewFromConfig(awsCfg)
	ssmClient := ssm.NewFromConfig(awsCfg)

	// Create storage implementations.
	donationTracker, err := storage.NewDonationTracker(dynamoClient, cfg.DynamoDB.TableName)
	if err != nil {
		return fmt.Errorf("creating donation tracker: %w", err)
	}

	stateStore, err := storage.NewStateStore(ssmClient, cfg.SSM.ParameterName)
	if err != nil {
		return fmt.Errorf("creating state store: %w", err)
	}

	tokenStore, err := storage.NewTokenStore(secretsClient, cfg.Blackbaud.RefreshTokenSecretARN)
	if err != nil {
		return fmt.Errorf("creating token store: %w", err)
	}

	// Create API clients.
	fundraiseupClient, err := fundraiseup.NewClient(
		cfg.FundraiseUp.APIKey,
		fundraiseup.WithBaseURL(cfg.FundraiseUp.BaseURL),
	)
	if err != nil {
		return fmt.Errorf("creating FundraiseUp client: %w", err)
	}

	blackbaudClient, err := blackbaud.NewClient(
		blackbaud.Config{
			ClientID:        cfg.Blackbaud.ClientID,
			ClientSecret:    cfg.Blackbaud.ClientSecret,
			SubscriptionKey: cfg.Blackbaud.SubscriptionKey,
			TokenStore:      tokenStore,
		},
		blackbaud.WithBaseURL(cfg.Blackbaud.APIBaseURL),
	)
	if err != nil {
		return fmt.Errorf("creating Blackbaud client: %w", err)
	}

	// Create and run sync service.
	syncService, err := sync.NewService(sync.Config{
		Blackbaud:       blackbaudClient,
		DonationTracker: donationTracker,
		FundraiseUp:     fundraiseupClient,
		Logger:          slog.Default(),
		GiftDefaults:    cfg.GiftDefaults,
		StateStore:      stateStore,
	})
	if err != nil {
		return fmt.Errorf("creating sync service: %w", err)
	}

	result, err := syncService.Run(ctx)
	if err != nil {
		return fmt.Errorf("running sync: %w", err)
	}

	slog.InfoContext(ctx, "sync complete",
		"donations_processed", result.DonationsProcessed,
		"constituents_created", result.ConstituentsCreated,
		"gifts_created", result.GiftsCreated,
		"gifts_updated", result.GiftsUpdated,
		"errors", len(result.Errors),
	)

	// Return error if any donations failed.
	if len(result.Errors) > 0 {
		return fmt.Errorf("sync completed with %d errors", len(result.Errors))
	}

	return nil
}
