// Package main provides the Lambda handler entry point for giftbridge.
package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"time"

	"github.com/aws/aws-lambda-go/lambda"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/secretsmanager"
	"github.com/aws/aws-sdk-go-v2/service/ssm"

	"github.com/peteski22/giftbridge/internal/blackbaud"
	"github.com/peteski22/giftbridge/internal/config"
	"github.com/peteski22/giftbridge/internal/fundraiseup"
	"github.com/peteski22/giftbridge/internal/storage"
	"github.com/peteski22/giftbridge/internal/sync"
)

func main() {
	// Check for subcommands first.
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "auth":
			if err := runBlackbaudAuth(); err != nil {
				fmt.Fprintln(os.Stderr, formatError(err))
				os.Exit(1)
			}
			return
		case "init":
			if err := runInit(); err != nil {
				fmt.Fprintln(os.Stderr, formatError(err))
				os.Exit(1)
			}
			return
		}
	}

	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, `giftbridge - Sync donations from FundraiseUp to Blackbaud Raiser's Edge NXT

Usage:
  giftbridge [command]
  giftbridge [flags]

Commands:
  init        Create a local configuration file
  auth        Authorize with Blackbaud (OAuth flow)

Flags:
`)
		flag.PrintDefaults()
		fmt.Fprintf(os.Stderr, `
When run without flags or commands, starts as an AWS Lambda handler.

Examples:
  # Set up local configuration
  giftbridge init

  # Authorize with Blackbaud
  giftbridge auth

  # Preview what would be synced (no AWS required)
  giftbridge --dry-run --since=2024-01-01T00:00:00Z

  # Run a real sync (requires AWS infrastructure)
  giftbridge --since=2024-01-01T00:00:00Z

  # Run as Lambda handler (default, no flags)
  giftbridge
`)
	}

	dryRun := flag.Bool("dry-run", false, "preview what would happen without making changes")
	since := flag.String("since", "", "override last sync time (RFC3339 format)")
	flag.Parse()

	// If running locally (flags provided), run directly with human-readable logs.
	// Otherwise, start Lambda handler with JSON logs.
	if *dryRun || *since != "" {
		logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
			Level: slog.LevelInfo,
		}))
		slog.SetDefault(logger)

		if err := runLocal(*dryRun, *since); err != nil {
			fmt.Fprintln(os.Stderr, formatError(err))
			os.Exit(1)
		}
		return
	}

	// Lambda mode: use JSON logs for CloudWatch.
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
	slog.SetDefault(logger)

	lambda.Start(handler)
}

// handler is the AWS Lambda entry point that runs a sync cycle.
func handler(ctx context.Context) error {
	slog.InfoContext(ctx, "starting sync")

	// Load configuration from environment variables.
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
	secretsClient := secretsmanager.NewFromConfig(awsCfg)
	ssmClient := ssm.NewFromConfig(awsCfg)

	// Create storage implementations.
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
	syncService, err := sync.New(sync.Config{
		Blackbaud:    blackbaudClient,
		FundraiseUp:  fundraiseupClient,
		GiftDefaults: cfg.GiftDefaults,
		Logger:       slog.Default(),
		StateStore:   stateStore,
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

// runLocal executes a sync using local configuration and file-based token storage.
// This mode is used for dry-run testing without AWS infrastructure.
func runLocal(dryRun bool, sinceStr string) error {
	ctx := context.Background()

	if dryRun {
		fmt.Println("=== DRY-RUN MODE ===")
		fmt.Println("No changes will be made to Blackbaud Raiser's Edge NXT")
		fmt.Println()
	}

	// Parse since time (required for dry-run without AWS).
	var sinceTime time.Time
	if sinceStr != "" {
		t, err := time.Parse(time.RFC3339, sinceStr)
		if err != nil {
			return fmt.Errorf("parsing since time: %w", err)
		}
		sinceTime = t
		fmt.Printf("Using since: %s\n\n", t.Format(time.RFC3339))
	} else if dryRun {
		// Default to 30 days ago for dry-run.
		sinceTime = time.Now().AddDate(0, 0, -30)
		fmt.Printf("Using default since (30 days ago): %s\n\n", sinceTime.Format(time.RFC3339))
	}

	// Load local configuration.
	cfg, err := config.LoadLocal()
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	// Get token path.
	tokenPath, err := config.TokenFilePath()
	if err != nil {
		return fmt.Errorf("getting token path: %w", err)
	}

	// Create local storage implementations (no AWS needed for dry-run).
	tokenStore, err := storage.NewFileTokenStore(tokenPath)
	if err != nil {
		return fmt.Errorf("creating token store: %w", err)
	}

	// Use noop state store for local runs.
	stateStore := storage.NewNoopStateStore(sinceTime)

	// Create API clients.
	fundraiseupClient, err := fundraiseup.NewClient(cfg.FundraiseUp.APIKey)
	if err != nil {
		return fmt.Errorf("creating FundraiseUp client: %w", err)
	}

	blackbaudClient, err := blackbaud.NewClient(blackbaud.Config{
		ClientID:        cfg.Blackbaud.ClientID,
		ClientSecret:    cfg.Blackbaud.ClientSecret,
		SubscriptionKey: cfg.Blackbaud.SubscriptionKey,
		TokenStore:      tokenStore,
	})
	if err != nil {
		return fmt.Errorf("creating Blackbaud client: %w", err)
	}

	// Create and run sync service.
	syncService, err := sync.New(sync.Config{
		Blackbaud:    blackbaudClient,
		DryRun:       dryRun,
		FundraiseUp:  fundraiseupClient,
		GiftDefaults: cfg.GiftDefaults,
		Logger:       slog.Default(),
		StateStore:   stateStore,
	})
	if err != nil {
		return fmt.Errorf("creating sync service: %w", err)
	}

	result, err := syncService.Run(ctx)
	if err != nil {
		return fmt.Errorf("running sync: %w", err)
	}

	// Print summary.
	printSummary(result, sinceTime)

	// Return error if any donations failed.
	if len(result.Errors) > 0 {
		return fmt.Errorf("sync completed with %d errors", len(result.Errors))
	}

	return nil
}

// printSummary outputs a human-readable summary of the sync results to stdout.
func printSummary(result *sync.Result, since time.Time) {
	fmt.Println()
	if result.DryRun {
		fmt.Println("=== Dry-Run Summary ===")
	} else {
		fmt.Println("=== Sync Summary ===")
	}

	fmt.Printf("Donations processed: %d\n", result.DonationsProcessed)
	fmt.Printf("Constituents: %d would be created, %d exist\n",
		result.ConstituentsCreated, result.ConstituentsExisting)

	giftsSummary := fmt.Sprintf("Gifts: %d would be created", result.GiftsCreated)
	if result.GiftsUpdated > 0 {
		giftsSummary += fmt.Sprintf(", %d would be updated", result.GiftsUpdated)
	}
	if result.GiftsSkippedExisting > 0 {
		giftsSummary += fmt.Sprintf(", %d skipped (exists)", result.GiftsSkippedExisting)
	}
	fmt.Println(giftsSummary)

	if len(result.Errors) > 0 {
		fmt.Printf("Errors: %d\n", len(result.Errors))
	}

	if result.DryRun {
		fmt.Println()
		fmt.Println("To run for real, deploy to AWS and run without --dry-run flag.")
		if !since.IsZero() {
			fmt.Printf("Use --since=%s to re-process from the same time.\n", since.Format(time.RFC3339))
		}
	}
}

// formatError formats an error for terminal display, indenting multi-line errors.
func formatError(err error) string {
	msg := err.Error()

	// Find first newline to check if this is a multi-line error.
	newlineIdx := strings.Index(msg, "\n")
	if newlineIdx == -1 {
		return fmt.Sprintf("Error: %s", msg)
	}

	firstLine := msg[:newlineIdx]
	restLines := msg[newlineIdx+1:]

	// Find the last ": " in the first line to separate prefixes from first item.
	// This handles nested error wrapping like "loading config: invalid config: actual error".
	lastColonIdx := strings.LastIndex(firstLine, ": ")
	if lastColonIdx == -1 {
		// No colon found, format each line as a bullet.
		var bullets []string
		for _, line := range strings.Split(msg, "\n") {
			if line = strings.TrimSpace(line); line != "" {
				bullets = append(bullets, "  - "+line)
			}
		}
		return "Error:\n" + strings.Join(bullets, "\n")
	}

	// Use only the first prefix for the header (e.g., "loading config").
	firstColonIdx := strings.Index(firstLine, ": ")
	prefix := firstLine[:firstColonIdx]
	firstItem := firstLine[lastColonIdx+2:]

	// Build bullet list.
	var bullets []string
	bullets = append(bullets, "  - "+firstItem)
	for _, line := range strings.Split(restLines, "\n") {
		if line = strings.TrimSpace(line); line != "" {
			bullets = append(bullets, "  - "+line)
		}
	}

	return fmt.Sprintf("Error: %s:\n%s", prefix, strings.Join(bullets, "\n"))
}
