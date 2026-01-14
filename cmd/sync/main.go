// Package main provides the Lambda handler entry point for giftbridge.
package main

import (
	"context"
	"log/slog"
	"os"

	"github.com/aws/aws-lambda-go/lambda"
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

	// TODO: Load config.
	// TODO: Initialize clients.
	// TODO: Run sync.

	slog.InfoContext(ctx, "sync complete")
	return nil
}
