// Package storage provides persistence implementations for the sync service.
package storage

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

// RecurringInfo contains metadata for tracking recurring gift series.
type RecurringInfo struct {
	// CreatedAt is when the donation was created.
	CreatedAt time.Time

	// DonationID is the FundraiseUp donation identifier.
	DonationID string

	// FirstGiftID is the Blackbaud gift ID of the first payment in this series.
	FirstGiftID string

	// GiftID is the Blackbaud gift identifier for this donation.
	GiftID string

	// RecurringID is the FundraiseUp recurring series identifier.
	RecurringID string

	// SequenceNumber is the position of this payment in the series (1-indexed).
	SequenceNumber int
}

// DonationTracker tracks donation to gift mappings in DynamoDB.
type DonationTracker struct {
	// client is the DynamoDB API client.
	client DynamoDBAPI

	// indexName is the name of the recurring ID GSI.
	indexName string

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

// TrackRecurring stores recurring donation metadata including series information.
func (t *DonationTracker) TrackRecurring(ctx context.Context, info RecurringInfo) error {
	if info.DonationID == "" {
		return errors.New("donation ID is required")
	}
	if info.GiftID == "" {
		return errors.New("gift ID is required")
	}
	if info.RecurringID == "" {
		return errors.New("recurring ID is required")
	}
	if info.FirstGiftID == "" {
		return errors.New("first gift ID is required")
	}
	if info.SequenceNumber < 1 {
		return errors.New("sequence number must be positive")
	}

	_, err := t.client.PutItem(ctx, &dynamodb.PutItemInput{
		TableName: aws.String(t.tableName),
		Item: map[string]types.AttributeValue{
			"donation_id":     &types.AttributeValueMemberS{Value: info.DonationID},
			"gift_id":         &types.AttributeValueMemberS{Value: info.GiftID},
			"recurring_id":    &types.AttributeValueMemberS{Value: info.RecurringID},
			"first_gift_id":   &types.AttributeValueMemberS{Value: info.FirstGiftID},
			"sequence_number": &types.AttributeValueMemberN{Value: strconv.Itoa(info.SequenceNumber)},
			"created_at":      &types.AttributeValueMemberS{Value: info.CreatedAt.Format(time.RFC3339)},
		},
	})
	if err != nil {
		return fmt.Errorf("putting recurring item to DynamoDB: %w", err)
	}

	return nil
}

// DonationsByRecurringID retrieves all donations in a recurring series, ordered by creation time.
func (t *DonationTracker) DonationsByRecurringID(ctx context.Context, recurringID string) ([]RecurringInfo, error) {
	if recurringID == "" {
		return nil, errors.New("recurring ID is required")
	}

	output, err := t.client.Query(ctx, &dynamodb.QueryInput{
		TableName:              aws.String(t.tableName),
		IndexName:              aws.String(t.indexName),
		KeyConditionExpression: aws.String("recurring_id = :rid"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":rid": &types.AttributeValueMemberS{Value: recurringID},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("querying DynamoDB: %w", err)
	}

	results := make([]RecurringInfo, 0, len(output.Items))
	for _, item := range output.Items {
		info, err := parseRecurringInfo(item)
		if err != nil {
			return nil, fmt.Errorf("parsing item: %w", err)
		}
		results = append(results, info)
	}

	return results, nil
}

func parseRecurringInfo(item map[string]types.AttributeValue) (RecurringInfo, error) {
	info := RecurringInfo{}

	if v, ok := item["donation_id"].(*types.AttributeValueMemberS); ok {
		info.DonationID = v.Value
	}
	if v, ok := item["gift_id"].(*types.AttributeValueMemberS); ok {
		info.GiftID = v.Value
	}
	if v, ok := item["recurring_id"].(*types.AttributeValueMemberS); ok {
		info.RecurringID = v.Value
	}
	if v, ok := item["first_gift_id"].(*types.AttributeValueMemberS); ok {
		info.FirstGiftID = v.Value
	}
	if v, ok := item["sequence_number"].(*types.AttributeValueMemberN); ok {
		seq, err := strconv.Atoi(v.Value)
		if err != nil {
			return info, fmt.Errorf("parsing sequence number: %w", err)
		}
		info.SequenceNumber = seq
	}
	if v, ok := item["created_at"].(*types.AttributeValueMemberS); ok {
		t, err := time.Parse(time.RFC3339, v.Value)
		if err != nil {
			return info, fmt.Errorf("parsing created_at: %w", err)
		}
		info.CreatedAt = t
	}

	return info, nil
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

	// Query retrieves items matching a key condition from DynamoDB.
	Query(
		ctx context.Context,
		params *dynamodb.QueryInput,
		optFns ...func(*dynamodb.Options),
	) (*dynamodb.QueryOutput, error)
}

// NewDonationTracker creates a new DynamoDB-backed donation tracker.
func NewDonationTracker(client DynamoDBAPI, tableName string, indexName string) (*DonationTracker, error) {
	if client == nil {
		return nil, errors.New("dynamodb client is required")
	}
	if tableName == "" {
		return nil, errors.New("table name is required")
	}
	if indexName == "" {
		return nil, errors.New("index name is required")
	}

	return &DonationTracker{
		client:    client,
		indexName: indexName,
		tableName: tableName,
	}, nil
}
