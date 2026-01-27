package storage

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/stretchr/testify/require"
)

type mockDynamoDBClient struct {
	getItemFunc func(ctx context.Context, params *dynamodb.GetItemInput, optFns ...func(*dynamodb.Options)) (*dynamodb.GetItemOutput, error)
	putItemFunc func(ctx context.Context, params *dynamodb.PutItemInput, optFns ...func(*dynamodb.Options)) (*dynamodb.PutItemOutput, error)
	queryFunc   func(ctx context.Context, params *dynamodb.QueryInput, optFns ...func(*dynamodb.Options)) (*dynamodb.QueryOutput, error)
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

func (m *mockDynamoDBClient) Query(
	ctx context.Context,
	params *dynamodb.QueryInput,
	optFns ...func(*dynamodb.Options),
) (*dynamodb.QueryOutput, error) {
	if m.queryFunc != nil {
		return m.queryFunc(ctx, params, optFns...)
	}
	return &dynamodb.QueryOutput{}, nil
}

func TestNewDonationTracker(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		client    DynamoDBAPI
		errMsg    string
		indexName string
		tableName string
		wantErr   bool
	}{
		"valid inputs": {
			client:    &mockDynamoDBClient{},
			indexName: "RecurringIdIndex",
			tableName: "donations",
			wantErr:   false,
		},
		"nil client": {
			client:    nil,
			indexName: "RecurringIdIndex",
			tableName: "donations",
			wantErr:   true,
			errMsg:    "dynamodb client is required",
		},
		"empty table name": {
			client:    &mockDynamoDBClient{},
			indexName: "RecurringIdIndex",
			tableName: "",
			wantErr:   true,
			errMsg:    "table name is required",
		},
		"empty index name": {
			client:    &mockDynamoDBClient{},
			indexName: "",
			tableName: "donations",
			wantErr:   true,
			errMsg:    "index name is required",
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			tracker, err := NewDonationTracker(tc.client, tc.tableName, tc.indexName)

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

			tracker, err := NewDonationTracker(tc.client, "donations", "RecurringIdIndex")
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

			tracker, err := NewDonationTracker(tc.client, "donations", "RecurringIdIndex")
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

func TestDonationTracker_TrackRecurring(t *testing.T) {
	t.Parallel()

	testTime := time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC)

	tests := map[string]struct {
		client  *mockDynamoDBClient
		errMsg  string
		info    RecurringInfo
		wantErr bool
	}{
		"successful track recurring": {
			client: &mockDynamoDBClient{
				putItemFunc: func(_ context.Context, params *dynamodb.PutItemInput, _ ...func(*dynamodb.Options)) (*dynamodb.PutItemOutput, error) {
					require.NotNil(t, params.Item["recurring_id"])
					require.NotNil(t, params.Item["first_gift_id"])
					require.NotNil(t, params.Item["sequence_number"])
					require.NotNil(t, params.Item["created_at"])
					return &dynamodb.PutItemOutput{}, nil
				},
			},
			info: RecurringInfo{
				CreatedAt:      testTime,
				DonationID:     "don_123",
				FirstGiftID:    "gift_001",
				GiftID:         "gift_456",
				RecurringID:    "rec_789",
				SequenceNumber: 2,
			},
			wantErr: false,
		},
		"empty donation ID": {
			client: &mockDynamoDBClient{},
			info: RecurringInfo{
				GiftID:         "gift_456",
				RecurringID:    "rec_789",
				FirstGiftID:    "gift_001",
				SequenceNumber: 1,
			},
			wantErr: true,
			errMsg:  "donation ID is required",
		},
		"empty gift ID": {
			client: &mockDynamoDBClient{},
			info: RecurringInfo{
				DonationID:     "don_123",
				RecurringID:    "rec_789",
				FirstGiftID:    "gift_001",
				SequenceNumber: 1,
			},
			wantErr: true,
			errMsg:  "gift ID is required",
		},
		"empty recurring ID": {
			client: &mockDynamoDBClient{},
			info: RecurringInfo{
				DonationID:     "don_123",
				GiftID:         "gift_456",
				FirstGiftID:    "gift_001",
				SequenceNumber: 1,
			},
			wantErr: true,
			errMsg:  "recurring ID is required",
		},
		"empty first gift ID": {
			client: &mockDynamoDBClient{},
			info: RecurringInfo{
				DonationID:     "don_123",
				GiftID:         "gift_456",
				RecurringID:    "rec_789",
				SequenceNumber: 1,
			},
			wantErr: true,
			errMsg:  "first gift ID is required",
		},
		"zero sequence number": {
			client: &mockDynamoDBClient{},
			info: RecurringInfo{
				DonationID:     "don_123",
				GiftID:         "gift_456",
				RecurringID:    "rec_789",
				FirstGiftID:    "gift_001",
				SequenceNumber: 0,
			},
			wantErr: true,
			errMsg:  "sequence number must be positive",
		},
		"dynamodb error": {
			client: &mockDynamoDBClient{
				putItemFunc: func(_ context.Context, _ *dynamodb.PutItemInput, _ ...func(*dynamodb.Options)) (*dynamodb.PutItemOutput, error) {
					return nil, errors.New("dynamodb error")
				},
			},
			info: RecurringInfo{
				CreatedAt:      testTime,
				DonationID:     "don_123",
				FirstGiftID:    "gift_001",
				GiftID:         "gift_456",
				RecurringID:    "rec_789",
				SequenceNumber: 1,
			},
			wantErr: true,
			errMsg:  "putting recurring item to DynamoDB",
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			tracker, err := NewDonationTracker(tc.client, "donations", "RecurringIdIndex")
			require.NoError(t, err)

			err = tracker.TrackRecurring(context.Background(), tc.info)

			if tc.wantErr {
				require.Error(t, err)
				require.Contains(t, err.Error(), tc.errMsg)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestDonationTracker_DonationsByRecurringID(t *testing.T) {
	t.Parallel()

	testTime := time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC)

	tests := map[string]struct {
		client      *mockDynamoDBClient
		errMsg      string
		recurringID string
		want        []RecurringInfo
		wantErr     bool
	}{
		"returns donations when found": {
			client: &mockDynamoDBClient{
				queryFunc: func(_ context.Context, params *dynamodb.QueryInput, _ ...func(*dynamodb.Options)) (*dynamodb.QueryOutput, error) {
					require.Equal(t, "RecurringIdIndex", *params.IndexName)
					return &dynamodb.QueryOutput{
						Items: []map[string]types.AttributeValue{
							{
								"donation_id":     &types.AttributeValueMemberS{Value: "don_001"},
								"gift_id":         &types.AttributeValueMemberS{Value: "gift_001"},
								"recurring_id":    &types.AttributeValueMemberS{Value: "rec_123"},
								"first_gift_id":   &types.AttributeValueMemberS{Value: "gift_001"},
								"sequence_number": &types.AttributeValueMemberN{Value: "1"},
								"created_at":      &types.AttributeValueMemberS{Value: testTime.Format(time.RFC3339)},
							},
							{
								"donation_id":     &types.AttributeValueMemberS{Value: "don_002"},
								"gift_id":         &types.AttributeValueMemberS{Value: "gift_002"},
								"recurring_id":    &types.AttributeValueMemberS{Value: "rec_123"},
								"first_gift_id":   &types.AttributeValueMemberS{Value: "gift_001"},
								"sequence_number": &types.AttributeValueMemberN{Value: "2"},
								"created_at": &types.AttributeValueMemberS{
									Value: testTime.Add(24 * time.Hour).Format(time.RFC3339),
								},
							},
						},
					}, nil
				},
			},
			recurringID: "rec_123",
			want: []RecurringInfo{
				{
					CreatedAt:      testTime,
					DonationID:     "don_001",
					FirstGiftID:    "gift_001",
					GiftID:         "gift_001",
					RecurringID:    "rec_123",
					SequenceNumber: 1,
				},
				{
					CreatedAt:      testTime.Add(24 * time.Hour),
					DonationID:     "don_002",
					FirstGiftID:    "gift_001",
					GiftID:         "gift_002",
					RecurringID:    "rec_123",
					SequenceNumber: 2,
				},
			},
			wantErr: false,
		},
		"returns empty when not found": {
			client: &mockDynamoDBClient{
				queryFunc: func(_ context.Context, _ *dynamodb.QueryInput, _ ...func(*dynamodb.Options)) (*dynamodb.QueryOutput, error) {
					return &dynamodb.QueryOutput{Items: nil}, nil
				},
			},
			recurringID: "rec_unknown",
			want:        []RecurringInfo{},
			wantErr:     false,
		},
		"empty recurring ID": {
			client:      &mockDynamoDBClient{},
			recurringID: "",
			wantErr:     true,
			errMsg:      "recurring ID is required",
		},
		"dynamodb error": {
			client: &mockDynamoDBClient{
				queryFunc: func(_ context.Context, _ *dynamodb.QueryInput, _ ...func(*dynamodb.Options)) (*dynamodb.QueryOutput, error) {
					return nil, errors.New("dynamodb error")
				},
			},
			recurringID: "rec_123",
			wantErr:     true,
			errMsg:      "querying DynamoDB",
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			tracker, err := NewDonationTracker(tc.client, "donations", "RecurringIdIndex")
			require.NoError(t, err)

			got, err := tracker.DonationsByRecurringID(context.Background(), tc.recurringID)

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
