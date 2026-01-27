package sync

import (
	"context"
	"log/slog"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/peteski22/giftbridge/internal/blackbaud"
	"github.com/peteski22/giftbridge/internal/config"
	"github.com/peteski22/giftbridge/internal/fundraiseup"
)

// mockStateStore implements StateStore for testing.
type mockStateStore struct {
	lastSync time.Time
	setErr   error
}

// LastSyncTime returns the last sync time.
func (m *mockStateStore) LastSyncTime(_ context.Context) (time.Time, error) {
	return m.lastSync, nil
}

// SetLastSyncTime sets the last sync time.
func (m *mockStateStore) SetLastSyncTime(_ context.Context, t time.Time) error {
	if m.setErr != nil {
		return m.setErr
	}
	m.lastSync = t
	return nil
}

// mockBlackbaudClient implements BlackbaudClient for testing.
type mockBlackbaudClient struct {
	gifts        map[string][]blackbaud.Gift
	constituents []blackbaud.Constituent
}

// CreateConstituent creates a new constituent.
func (m *mockBlackbaudClient) CreateConstituent(_ context.Context, _ *blackbaud.Constituent) (string, error) {
	return "constituent-123", nil
}

// CreateGift creates a new gift.
func (m *mockBlackbaudClient) CreateGift(_ context.Context, _ *blackbaud.Gift) (string, error) {
	return "gift-123", nil
}

// ListGiftsByConstituent lists gifts for a constituent.
func (m *mockBlackbaudClient) ListGiftsByConstituent(
	_ context.Context,
	constituentID string,
	_ []blackbaud.GiftType,
) ([]blackbaud.Gift, error) {
	if m.gifts == nil {
		return nil, nil
	}
	return m.gifts[constituentID], nil
}

// SearchConstituents searches for constituents.
func (m *mockBlackbaudClient) SearchConstituents(_ context.Context, _ string) ([]blackbaud.Constituent, error) {
	return m.constituents, nil
}

// UpdateGift updates a gift.
func (m *mockBlackbaudClient) UpdateGift(_ context.Context, _ string, _ *blackbaud.Gift) error {
	return nil
}

func TestNew(t *testing.T) {
	t.Parallel()

	validGiftDefaults := config.GiftDefaults{FundID: "fund-123", Type: "Donation"}

	validConfig := Config{
		Blackbaud:    &blackbaud.Client{},
		FundraiseUp:  &fundraiseup.Client{},
		GiftDefaults: validGiftDefaults,
		Logger:       slog.Default(),
		StateStore:   &mockStateStore{},
	}

	tests := map[string]struct {
		config  Config
		errMsg  string
		wantErr bool
	}{
		"valid config": {
			config:  validConfig,
			wantErr: false,
		},
		"missing blackbaud client": {
			config: Config{
				FundraiseUp:  &fundraiseup.Client{},
				GiftDefaults: validGiftDefaults,
				StateStore:   &mockStateStore{},
			},
			wantErr: true,
			errMsg:  "blackbaud client is required",
		},
		"missing fundraiseup client": {
			config: Config{
				Blackbaud:    &blackbaud.Client{},
				GiftDefaults: validGiftDefaults,
				StateStore:   &mockStateStore{},
			},
			wantErr: true,
			errMsg:  "fundraiseup client is required",
		},
		"missing gift defaults fund id": {
			config: Config{
				Blackbaud:   &blackbaud.Client{},
				FundraiseUp: &fundraiseup.Client{},
				StateStore:  &mockStateStore{},
			},
			wantErr: true,
			errMsg:  "gift defaults fund ID is required",
		},
		"missing state store": {
			config: Config{
				Blackbaud:    &blackbaud.Client{},
				FundraiseUp:  &fundraiseup.Client{},
				GiftDefaults: validGiftDefaults,
			},
			wantErr: true,
			errMsg:  "state store is required",
		},
		"nil logger uses default": {
			config: Config{
				Blackbaud:    &blackbaud.Client{},
				FundraiseUp:  &fundraiseup.Client{},
				GiftDefaults: validGiftDefaults,
				StateStore:   &mockStateStore{},
			},
			wantErr: false,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			svc, err := New(tc.config)

			if tc.wantErr {
				require.Error(t, err)
				require.Contains(t, err.Error(), tc.errMsg)
				require.Nil(t, svc)
			} else {
				require.NoError(t, err)
				require.NotNil(t, svc)
			}
		})
	}
}

func TestConfigValidate(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		config       Config
		errFragments []string
		wantErr      bool
	}{
		"valid config": {
			config: Config{
				Blackbaud:    &blackbaud.Client{},
				FundraiseUp:  &fundraiseup.Client{},
				GiftDefaults: config.GiftDefaults{FundID: "fund-123"},
				StateStore:   &mockStateStore{},
			},
			wantErr: false,
		},
		"all fields missing": {
			config:  Config{},
			wantErr: true,
			errFragments: []string{
				"blackbaud client is required",
				"fundraiseup client is required",
				"gift defaults fund ID is required",
				"state store is required",
			},
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			err := tc.config.validate()

			if tc.wantErr {
				require.Error(t, err)
				for _, fragment := range tc.errFragments {
					require.Contains(t, err.Error(), fragment)
				}
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestDefaultSyncStart(t *testing.T) {
	t.Parallel()

	start := defaultSyncStart()
	expected := time.Now().AddDate(0, 0, -30)

	// Allow 1 second tolerance.
	require.WithinDuration(t, expected, start, time.Second)
}

func TestGetRecurringContext(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		bbClient *mockBlackbaudClient
		donation fundraiseup.Donation
		want     recurringContext
		wantErr  bool
	}{
		"non-recurring donation returns empty context": {
			bbClient: &mockBlackbaudClient{},
			donation: fundraiseup.Donation{
				ID:            "don_123",
				RecurringPlan: nil,
			},
			want:    recurringContext{},
			wantErr: false,
		},
		"first recurring donation returns isFirstInSeries true": {
			bbClient: &mockBlackbaudClient{},
			donation: fundraiseup.Donation{
				ID:            "don_123",
				Installment:   "1",
				RecurringPlan: &fundraiseup.RecurringPlan{ID: "rec_456"},
			},
			want: recurringContext{
				isFirstInSeries: true,
				sequenceNumber:  1,
			},
			wantErr: false,
		},
		"subsequent recurring donation with existing first gift": {
			bbClient: &mockBlackbaudClient{
				gifts: map[string][]blackbaud.Gift{
					"constituent-123": {
						{
							ID:       "gift_001",
							LookupID: "rec_456",
							Type:     blackbaud.GiftTypeRecurringGift,
						},
					},
				},
			},
			donation: fundraiseup.Donation{
				ID:            "don_002",
				Installment:   "2",
				RecurringPlan: &fundraiseup.RecurringPlan{ID: "rec_456"},
			},
			want: recurringContext{
				firstGiftID:     "gift_001",
				isFirstInSeries: false,
				sequenceNumber:  2,
			},
			wantErr: false,
		},
		"subsequent recurring donation without prior gifts treated as first": {
			bbClient: &mockBlackbaudClient{},
			donation: fundraiseup.Donation{
				ID:            "don_003",
				Installment:   "3",
				RecurringPlan: &fundraiseup.RecurringPlan{ID: "rec_456"},
			},
			want: recurringContext{
				isFirstInSeries: true,
				sequenceNumber:  3,
			},
			wantErr: false,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			svc := &Service{
				blackbaud: tc.bbClient,
				giftCache: make(map[string][]blackbaud.Gift),
			}

			got, err := svc.getRecurringContext(context.Background(), "constituent-123", tc.donation)

			if tc.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				require.Equal(t, tc.want, got)
			}
		})
	}
}

func TestMapDonationToGift(t *testing.T) {
	t.Parallel()

	testTime := time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC)

	tests := map[string]struct {
		donation        fundraiseup.Donation
		recCtx          recurringContext
		wantBatchPrefix string
		wantIsManual    bool
		wantLinkedGifts []string
		wantLookupID    string
		wantOrigin      string
		wantSubtype     blackbaud.GiftSubtype
		wantType        blackbaud.GiftType
	}{
		"one-off donation uses Donation type with donation ID as LookupID": {
			donation: fundraiseup.Donation{
				ID:            "don_123",
				Amount:        "50.00",
				CreatedAt:     testTime,
				RecurringPlan: nil,
			},
			recCtx:          recurringContext{},
			wantBatchPrefix: "FundraiseUp",
			wantIsManual:    true,
			wantLinkedGifts: nil,
			wantLookupID:    "don_123",
			wantOrigin:      "",
			wantSubtype:     "",
			wantType:        blackbaud.GiftTypeDonation,
		},
		"first recurring donation uses RecurringGift type": {
			donation: fundraiseup.Donation{
				ID:            "don_123",
				Amount:        "50.00",
				CreatedAt:     testTime,
				RecurringPlan: &fundraiseup.RecurringPlan{ID: "rec_456"},
			},
			recCtx: recurringContext{
				isFirstInSeries: true,
				sequenceNumber:  1,
			},
			wantBatchPrefix: "FundraiseUp",
			wantIsManual:    true,
			wantLinkedGifts: nil,
			wantLookupID:    "rec_456",
			wantOrigin:      `{"donation_id":"don_123","name":"FundraiseUp"}`,
			wantSubtype:     blackbaud.GiftSubtypeRecurring,
			wantType:        blackbaud.GiftTypeRecurringGift,
		},
		"subsequent recurring donation uses RecurringGiftPayment type with LinkedGifts": {
			donation: fundraiseup.Donation{
				ID:            "don_124",
				Amount:        "50.00",
				CreatedAt:     testTime,
				RecurringPlan: &fundraiseup.RecurringPlan{ID: "rec_456"},
			},
			recCtx: recurringContext{
				firstGiftID:     "gift_001",
				isFirstInSeries: false,
				sequenceNumber:  2,
			},
			wantBatchPrefix: "FundraiseUp",
			wantIsManual:    true,
			wantLinkedGifts: []string{"gift_001"},
			wantLookupID:    "rec_456",
			wantOrigin:      `{"donation_id":"don_124","name":"FundraiseUp"}`,
			wantSubtype:     blackbaud.GiftSubtypeRecurring,
			wantType:        blackbaud.GiftTypeRecurringGiftPayment,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			svc := &Service{
				giftDefaults: config.GiftDefaults{
					FundID: "fund-123",
					Type:   "Donation",
				},
			}

			got, err := svc.mapDonationToGift(tc.donation, tc.recCtx)

			require.NoError(t, err)
			require.Equal(t, tc.wantBatchPrefix, got.BatchPrefix)
			require.Equal(t, tc.wantIsManual, got.IsManual)
			require.Equal(t, tc.wantLinkedGifts, got.LinkedGifts)
			require.Equal(t, tc.wantLookupID, got.LookupID)
			require.Equal(t, tc.wantOrigin, got.Origin)
			require.Equal(t, tc.wantSubtype, got.Subtype)
			require.Equal(t, tc.wantType, got.Type)
			require.Len(t, got.GiftSplits, 1)
			require.Equal(t, "fund-123", got.GiftSplits[0].FundID)
		})
	}
}

func TestFindExistingGift(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		bbClient   *mockBlackbaudClient
		donation   fundraiseup.Donation
		wantGiftID string
		wantFound  bool
	}{
		"one-time donation found by lookup_id": {
			bbClient: &mockBlackbaudClient{
				gifts: map[string][]blackbaud.Gift{
					"constituent-123": {
						{ID: "gift_001", LookupID: "don_123", Type: blackbaud.GiftTypeDonation},
					},
				},
			},
			donation: fundraiseup.Donation{
				ID: "don_123",
			},
			wantGiftID: "gift_001",
			wantFound:  true,
		},
		"one-time donation not found": {
			bbClient: &mockBlackbaudClient{},
			donation: fundraiseup.Donation{
				ID: "don_123",
			},
			wantGiftID: "",
			wantFound:  false,
		},
		"recurring donation found by origin donation_id": {
			bbClient: &mockBlackbaudClient{
				gifts: map[string][]blackbaud.Gift{
					"constituent-123": {
						{
							ID:       "gift_001",
							LookupID: "rec_456",
							Origin:   `{"donation_id":"don_123","name":"FundraiseUp"}`,
							Type:     blackbaud.GiftTypeRecurringGift,
						},
					},
				},
			},
			donation: fundraiseup.Donation{
				ID:            "don_123",
				RecurringPlan: &fundraiseup.RecurringPlan{ID: "rec_456"},
			},
			wantGiftID: "gift_001",
			wantFound:  true,
		},
		"recurring donation not found when origin doesn't match": {
			bbClient: &mockBlackbaudClient{
				gifts: map[string][]blackbaud.Gift{
					"constituent-123": {
						{
							ID:       "gift_001",
							LookupID: "rec_456",
							Origin:   `{"donation_id":"don_different","name":"FundraiseUp"}`,
							Type:     blackbaud.GiftTypeRecurringGift,
						},
					},
				},
			},
			donation: fundraiseup.Donation{
				ID:            "don_123",
				RecurringPlan: &fundraiseup.RecurringPlan{ID: "rec_456"},
			},
			wantGiftID: "",
			wantFound:  false,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			svc := &Service{
				blackbaud: tc.bbClient,
				giftCache: make(map[string][]blackbaud.Gift),
			}

			got, err := svc.findExistingGift(context.Background(), "constituent-123", tc.donation)

			require.NoError(t, err)
			if tc.wantFound {
				require.NotNil(t, got)
				require.Equal(t, tc.wantGiftID, got.ID)
			} else {
				require.Nil(t, got)
			}
		})
	}
}

func TestFindFirstRecurringGift(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		bbClient    *mockBlackbaudClient
		recurringID string
		wantGiftID  string
		wantFound   bool
	}{
		"first gift found": {
			bbClient: &mockBlackbaudClient{
				gifts: map[string][]blackbaud.Gift{
					"constituent-123": {
						{ID: "gift_001", LookupID: "rec_456", Type: blackbaud.GiftTypeRecurringGift},
						{ID: "gift_002", LookupID: "rec_456", Type: blackbaud.GiftTypeRecurringGiftPayment},
					},
				},
			},
			recurringID: "rec_456",
			wantGiftID:  "gift_001",
			wantFound:   true,
		},
		"first gift not found when only payments exist": {
			bbClient: &mockBlackbaudClient{
				gifts: map[string][]blackbaud.Gift{
					"constituent-123": {
						{ID: "gift_002", LookupID: "rec_456", Type: blackbaud.GiftTypeRecurringGiftPayment},
					},
				},
			},
			recurringID: "rec_456",
			wantGiftID:  "",
			wantFound:   false,
		},
		"first gift not found with different recurring_id": {
			bbClient: &mockBlackbaudClient{
				gifts: map[string][]blackbaud.Gift{
					"constituent-123": {
						{ID: "gift_001", LookupID: "rec_different", Type: blackbaud.GiftTypeRecurringGift},
					},
				},
			},
			recurringID: "rec_456",
			wantGiftID:  "",
			wantFound:   false,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			svc := &Service{
				blackbaud: tc.bbClient,
				giftCache: make(map[string][]blackbaud.Gift),
			}

			got, err := svc.findFirstRecurringGift(context.Background(), "constituent-123", tc.recurringID)

			require.NoError(t, err)
			if tc.wantFound {
				require.NotNil(t, got)
				require.Equal(t, tc.wantGiftID, got.ID)
			} else {
				require.Nil(t, got)
			}
		})
	}
}

func TestGetConstituentGifts(t *testing.T) {
	t.Parallel()

	t.Run("returns cached gifts on second call", func(t *testing.T) {
		t.Parallel()

		callCount := 0
		client := &countingBlackbaudClient{
			gifts: map[string][]blackbaud.Gift{
				"constituent-123": {
					{ID: "gift_001", LookupID: "don_123"},
				},
			},
			callCount: &callCount,
		}

		svc := &Service{
			blackbaud: client,
			giftCache: make(map[string][]blackbaud.Gift),
		}

		// First call should hit the client.
		gifts1, err := svc.getConstituentGifts(context.Background(), "constituent-123")
		require.NoError(t, err)
		require.Len(t, gifts1, 1)
		require.Equal(t, 1, callCount)

		// Second call should return cached results.
		gifts2, err := svc.getConstituentGifts(context.Background(), "constituent-123")
		require.NoError(t, err)
		require.Len(t, gifts2, 1)
		require.Equal(t, 1, callCount) // Still 1, not 2.
	})

	t.Run("different constituents are cached separately", func(t *testing.T) {
		t.Parallel()

		callCount := 0
		client := &countingBlackbaudClient{
			gifts: map[string][]blackbaud.Gift{
				"constituent-A": {{ID: "gift_A"}},
				"constituent-B": {{ID: "gift_B"}},
			},
			callCount: &callCount,
		}

		svc := &Service{
			blackbaud: client,
			giftCache: make(map[string][]blackbaud.Gift),
		}

		giftsA, err := svc.getConstituentGifts(context.Background(), "constituent-A")
		require.NoError(t, err)
		require.Equal(t, "gift_A", giftsA[0].ID)
		require.Equal(t, 1, callCount)

		giftsB, err := svc.getConstituentGifts(context.Background(), "constituent-B")
		require.NoError(t, err)
		require.Equal(t, "gift_B", giftsB[0].ID)
		require.Equal(t, 2, callCount) // Second call for different constituent.
	})
}

// countingBlackbaudClient tracks how many times ListGiftsByConstituent is called.
type countingBlackbaudClient struct {
	callCount    *int
	constituents []blackbaud.Constituent
	gifts        map[string][]blackbaud.Gift
}

// CreateConstituent creates a new constituent.
func (c *countingBlackbaudClient) CreateConstituent(
	_ context.Context,
	_ *blackbaud.Constituent,
) (string, error) {
	return "constituent-123", nil
}

// CreateGift creates a new gift.
func (c *countingBlackbaudClient) CreateGift(_ context.Context, _ *blackbaud.Gift) (string, error) {
	return "gift-123", nil
}

// ListGiftsByConstituent lists gifts for a constituent and increments the call counter.
func (c *countingBlackbaudClient) ListGiftsByConstituent(
	_ context.Context,
	constituentID string,
	_ []blackbaud.GiftType,
) ([]blackbaud.Gift, error) {
	*c.callCount++
	if c.gifts == nil {
		return nil, nil
	}
	return c.gifts[constituentID], nil
}

// SearchConstituents searches for constituents.
func (c *countingBlackbaudClient) SearchConstituents(
	_ context.Context,
	_ string,
) ([]blackbaud.Constituent, error) {
	return c.constituents, nil
}

// UpdateGift updates a gift.
func (c *countingBlackbaudClient) UpdateGift(_ context.Context, _ string, _ *blackbaud.Gift) error {
	return nil
}

func TestFindOrCreateConstituent(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		bbClient    *mockBlackbaudClient
		donation    fundraiseup.Donation
		wantCreated bool
		wantErr     bool
		wantErrMsg  string
		wantID      string
	}{
		"finds existing constituent by email": {
			bbClient: &mockBlackbaudClient{
				constituents: []blackbaud.Constituent{
					{ID: "existing-123"},
				},
			},
			donation: fundraiseup.Donation{
				ID:        "don_123",
				Supporter: &fundraiseup.Supporter{Email: "test@example.com"},
			},
			wantID:      "existing-123",
			wantCreated: false,
			wantErr:     false,
		},
		"creates new constituent when not found": {
			bbClient: &mockBlackbaudClient{
				constituents: nil, // No existing constituents.
			},
			donation: fundraiseup.Donation{
				ID: "don_123",
				Supporter: &fundraiseup.Supporter{
					Email:     "new@example.com",
					FirstName: "John",
					LastName:  "Doe",
				},
			},
			wantID:      "constituent-123", // From mock.
			wantCreated: true,
			wantErr:     false,
		},
		"returns error when donation has no supporter": {
			bbClient: &mockBlackbaudClient{},
			donation: fundraiseup.Donation{
				ID:        "don_123",
				Supporter: nil,
			},
			wantErr:    true,
			wantErrMsg: "donation has no supporter",
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			svc := &Service{
				blackbaud: tc.bbClient,
			}

			id, created, err := svc.findOrCreateConstituent(context.Background(), tc.donation)

			if tc.wantErr {
				require.Error(t, err)
				require.Contains(t, err.Error(), tc.wantErrMsg)
			} else {
				require.NoError(t, err)
				require.Equal(t, tc.wantID, id)
				require.Equal(t, tc.wantCreated, created)
			}
		})
	}
}

func TestProcessDonation(t *testing.T) {
	t.Parallel()

	t.Run("skips existing gift", func(t *testing.T) {
		t.Parallel()

		svc := &Service{
			blackbaud: &mockBlackbaudClient{
				constituents: []blackbaud.Constituent{{ID: "const-123"}},
				gifts: map[string][]blackbaud.Gift{
					"const-123": {{ID: "existing-gift", LookupID: "don_123"}},
				},
			},
			giftCache:    make(map[string][]blackbaud.Gift),
			giftDefaults: config.GiftDefaults{FundID: "fund-1", Type: "Donation"},
			logger:       slog.Default(),
		}

		donation := fundraiseup.Donation{
			ID:        "don_123",
			Supporter: &fundraiseup.Supporter{Email: "test@example.com"},
		}

		result := svc.processDonation(context.Background(), donation)

		require.NoError(t, result.Error)
		require.True(t, result.GiftSkippedExisting)
		require.False(t, result.GiftCreated)
		require.Equal(t, "existing-gift", result.GiftID)
	})

	t.Run("creates new gift when none exists", func(t *testing.T) {
		t.Parallel()

		svc := &Service{
			blackbaud: &mockBlackbaudClient{
				constituents: []blackbaud.Constituent{{ID: "const-123"}},
				gifts:        nil,
			},
			giftCache:    make(map[string][]blackbaud.Gift),
			giftDefaults: config.GiftDefaults{FundID: "fund-1", Type: "Donation"},
			logger:       slog.Default(),
		}

		donation := fundraiseup.Donation{
			ID:        "don_456",
			Amount:    "50.00",
			Supporter: &fundraiseup.Supporter{Email: "test@example.com"},
		}

		result := svc.processDonation(context.Background(), donation)

		require.NoError(t, result.Error)
		require.False(t, result.GiftSkippedExisting)
		require.True(t, result.GiftCreated)
		require.Equal(t, "gift-123", result.GiftID) // From mock.
	})

	t.Run("returns error when no supporter", func(t *testing.T) {
		t.Parallel()

		svc := &Service{
			blackbaud:    &mockBlackbaudClient{},
			giftCache:    make(map[string][]blackbaud.Gift),
			giftDefaults: config.GiftDefaults{FundID: "fund-1", Type: "Donation"},
			logger:       slog.Default(),
		}

		donation := fundraiseup.Donation{
			ID:        "don_789",
			Supporter: nil,
		}

		result := svc.processDonation(context.Background(), donation)

		require.Error(t, result.Error)
		require.Contains(t, result.Error.Error(), "donation has no supporter")
	})
}
