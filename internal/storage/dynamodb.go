// Package storage provides persistence implementations for the sync service.
package storage

import (
	"context"
	"errors"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

// DonationTracker tracks donation to gift mappings in DynamoDB.
type DonationTracker struct {
	// client is the DynamoDB API client.
	client DynamoDBAPI

	// tableName is the name of the DynamoDB table.
	tableName string
}

// GiftID returns the Blackbaud gift ID for a FundraiseUp donation, or empty if not tracked.
func (t *DonationTracker) GiftID(ctx context.Context, donationID string) (string, error) {
	if donationID == "" {
		return "", errors.New("donation ID is required")
	}

	output, err := t.client.GetItem(ctx, &dynamodb.GetItemInput{
		TableName: aws.String(t.tableName),
		Key: map[string]types.AttributeValue{
			"donation_id": &types.AttributeValueMemberS{Value: donationID},
		},
	})
	if err != nil {
		return "", fmt.Errorf("getting item from DynamoDB: %w", err)
	}

	if output.Item == nil {
		return "", nil
	}

	giftIDAttr, ok := output.Item["gift_id"]
	if !ok {
		return "", nil
	}

	giftIDStr, ok := giftIDAttr.(*types.AttributeValueMemberS)
	if !ok {
		return "", nil
	}

	return giftIDStr.Value, nil
}

// Track stores the mapping between a donation ID and gift ID.
func (t *DonationTracker) Track(ctx context.Context, donationID string, giftID string) error {
	if donationID == "" {
		return errors.New("donation ID is required")
	}
	if giftID == "" {
		return errors.New("gift ID is required")
	}

	_, err := t.client.PutItem(ctx, &dynamodb.PutItemInput{
		TableName: aws.String(t.tableName),
		Item: map[string]types.AttributeValue{
			"donation_id": &types.AttributeValueMemberS{Value: donationID},
			"gift_id":     &types.AttributeValueMemberS{Value: giftID},
		},
	})
	if err != nil {
		return fmt.Errorf("putting item to DynamoDB: %w", err)
	}

	return nil
}

// DynamoDBAPI defines the DynamoDB operations used by the tracker.
type DynamoDBAPI interface {
	// GetItem retrieves an item from DynamoDB.
	GetItem(
		ctx context.Context,
		params *dynamodb.GetItemInput,
		optFns ...func(*dynamodb.Options),
	) (*dynamodb.GetItemOutput, error)

	// PutItem stores an item in DynamoDB.
	PutItem(
		ctx context.Context,
		params *dynamodb.PutItemInput,
		optFns ...func(*dynamodb.Options),
	) (*dynamodb.PutItemOutput, error)
}

// NewDonationTracker creates a new DynamoDB-backed donation tracker.
func NewDonationTracker(client DynamoDBAPI, tableName string) (*DonationTracker, error) {
	if client == nil {
		return nil, errors.New("dynamodb client is required")
	}
	if tableName == "" {
		return nil, errors.New("table name is required")
	}

	return &DonationTracker{
		client:    client,
		tableName: tableName,
	}, nil
}
