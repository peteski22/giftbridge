package storage

import (
	"context"
	"errors"
	"testing"

	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/stretchr/testify/require"
)

type mockDynamoDBClient struct {
	getItemFunc func(ctx context.Context, params *dynamodb.GetItemInput, optFns ...func(*dynamodb.Options)) (*dynamodb.GetItemOutput, error)
	putItemFunc func(ctx context.Context, params *dynamodb.PutItemInput, optFns ...func(*dynamodb.Options)) (*dynamodb.PutItemOutput, error)
}

func (m *mockDynamoDBClient) GetItem(
	ctx context.Context,
	params *dynamodb.GetItemInput,
	optFns ...func(*dynamodb.Options),
) (*dynamodb.GetItemOutput, error) {
	if m.getItemFunc != nil {
		return m.getItemFunc(ctx, params, optFns...)
	}
	return &dynamodb.GetItemOutput{}, nil
}

func (m *mockDynamoDBClient) PutItem(
	ctx context.Context,
	params *dynamodb.PutItemInput,
	optFns ...func(*dynamodb.Options),
) (*dynamodb.PutItemOutput, error) {
	if m.putItemFunc != nil {
		return m.putItemFunc(ctx, params, optFns...)
	}
	return &dynamodb.PutItemOutput{}, nil
}

func TestNewDonationTracker(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		client    DynamoDBAPI
		errMsg    string
		tableName string
		wantErr   bool
	}{
		"valid inputs": {
			client:    &mockDynamoDBClient{},
			tableName: "donations",
			wantErr:   false,
		},
		"nil client": {
			client:    nil,
			tableName: "donations",
			wantErr:   true,
			errMsg:    "dynamodb client is required",
		},
		"empty table name": {
			client:    &mockDynamoDBClient{},
			tableName: "",
			wantErr:   true,
			errMsg:    "table name is required",
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			tracker, err := NewDonationTracker(tc.client, tc.tableName)

			if tc.wantErr {
				require.Error(t, err)
				require.Contains(t, err.Error(), tc.errMsg)
				require.Nil(t, tracker)
			} else {
				require.NoError(t, err)
				require.NotNil(t, tracker)
			}
		})
	}
}

func TestDonationTracker_GiftID(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		client     *mockDynamoDBClient
		donationID string
		errMsg     string
		want       string
		wantErr    bool
	}{
		"returns gift ID when found": {
			client: &mockDynamoDBClient{
				getItemFunc: func(_ context.Context, _ *dynamodb.GetItemInput, _ ...func(*dynamodb.Options)) (*dynamodb.GetItemOutput, error) {
					return &dynamodb.GetItemOutput{
						Item: map[string]types.AttributeValue{
							"donation_id": &types.AttributeValueMemberS{Value: "don_123"},
							"gift_id":     &types.AttributeValueMemberS{Value: "gift_456"},
						},
					}, nil
				},
			},
			donationID: "don_123",
			want:       "gift_456",
			wantErr:    false,
		},
		"returns empty when not found": {
			client: &mockDynamoDBClient{
				getItemFunc: func(_ context.Context, _ *dynamodb.GetItemInput, _ ...func(*dynamodb.Options)) (*dynamodb.GetItemOutput, error) {
					return &dynamodb.GetItemOutput{Item: nil}, nil
				},
			},
			donationID: "don_unknown",
			want:       "",
			wantErr:    false,
		},
		"returns empty when gift_id missing": {
			client: &mockDynamoDBClient{
				getItemFunc: func(_ context.Context, _ *dynamodb.GetItemInput, _ ...func(*dynamodb.Options)) (*dynamodb.GetItemOutput, error) {
					return &dynamodb.GetItemOutput{
						Item: map[string]types.AttributeValue{
							"donation_id": &types.AttributeValueMemberS{Value: "don_123"},
						},
					}, nil
				},
			},
			donationID: "don_123",
			want:       "",
			wantErr:    false,
		},
		"empty donation ID": {
			client:     &mockDynamoDBClient{},
			donationID: "",
			wantErr:    true,
			errMsg:     "donation ID is required",
		},
		"dynamodb error": {
			client: &mockDynamoDBClient{
				getItemFunc: func(_ context.Context, _ *dynamodb.GetItemInput, _ ...func(*dynamodb.Options)) (*dynamodb.GetItemOutput, error) {
					return nil, errors.New("dynamodb error")
				},
			},
			donationID: "don_123",
			wantErr:    true,
			errMsg:     "getting item from DynamoDB",
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			tracker, err := NewDonationTracker(tc.client, "donations")
			require.NoError(t, err)

			got, err := tracker.GiftID(context.Background(), tc.donationID)

			if tc.wantErr {
				require.Error(t, err)
				require.Contains(t, err.Error(), tc.errMsg)
			} else {
				require.NoError(t, err)
				require.Equal(t, tc.want, got)
			}
		})
	}
}

func TestDonationTracker_Track(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		client     *mockDynamoDBClient
		donationID string
		errMsg     string
		giftID     string
		wantErr    bool
	}{
		"successful track": {
			client: &mockDynamoDBClient{
				putItemFunc: func(_ context.Context, _ *dynamodb.PutItemInput, _ ...func(*dynamodb.Options)) (*dynamodb.PutItemOutput, error) {
					return &dynamodb.PutItemOutput{}, nil
				},
			},
			donationID: "don_123",
			giftID:     "gift_456",
			wantErr:    false,
		},
		"empty donation ID": {
			client:     &mockDynamoDBClient{},
			donationID: "",
			giftID:     "gift_456",
			wantErr:    true,
			errMsg:     "donation ID is required",
		},
		"empty gift ID": {
			client:     &mockDynamoDBClient{},
			donationID: "don_123",
			giftID:     "",
			wantErr:    true,
			errMsg:     "gift ID is required",
		},
		"dynamodb error": {
			client: &mockDynamoDBClient{
				putItemFunc: func(_ context.Context, _ *dynamodb.PutItemInput, _ ...func(*dynamodb.Options)) (*dynamodb.PutItemOutput, error) {
					return nil, errors.New("dynamodb error")
				},
			},
			donationID: "don_123",
			giftID:     "gift_456",
			wantErr:    true,
			errMsg:     "putting item to DynamoDB",
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			tracker, err := NewDonationTracker(tc.client, "donations")
			require.NoError(t, err)

			err = tracker.Track(context.Background(), tc.donationID, tc.giftID)

			if tc.wantErr {
				require.Error(t, err)
				require.Contains(t, err.Error(), tc.errMsg)
			} else {
				require.NoError(t, err)
			}
		})
	}
}
