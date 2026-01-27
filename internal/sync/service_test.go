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
	"github.com/peteski22/giftbridge/internal/storage"
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

// mockDonationTracker implements DonationTracker for testing.
type mockDonationTracker struct {
	giftIDs          map[string]string
	recurringRecords map[string][]storage.RecurringInfo
	trackErr         error
	trackRecurErr    error
	queryRecurErr    error
}

// DonationsByRecurringID returns donations for a recurring series.
func (m *mockDonationTracker) DonationsByRecurringID(
	_ context.Context,
	recurringID string,
) ([]storage.RecurringInfo, error) {
	if m.queryRecurErr != nil {
		return nil, m.queryRecurErr
	}
	if m.recurringRecords == nil {
		return nil, nil
	}
	return m.recurringRecords[recurringID], nil
}

// GiftID returns the gift ID for a donation.
func (m *mockDonationTracker) GiftID(_ context.Context, donationID string) (string, error) {
	return m.giftIDs[donationID], nil
}

// Track stores the donation to gift mapping.
func (m *mockDonationTracker) Track(_ context.Context, donationID string, giftID string) error {
	if m.trackErr != nil {
		return m.trackErr
	}
	if m.giftIDs == nil {
		m.giftIDs = make(map[string]string)
	}
	m.giftIDs[donationID] = giftID
	return nil
}

// TrackRecurring stores recurring donation metadata.
func (m *mockDonationTracker) TrackRecurring(_ context.Context, info storage.RecurringInfo) error {
	if m.trackRecurErr != nil {
		return m.trackRecurErr
	}
	if m.recurringRecords == nil {
		m.recurringRecords = make(map[string][]storage.RecurringInfo)
	}
	m.recurringRecords[info.RecurringID] = append(m.recurringRecords[info.RecurringID], info)
	return nil
}

func TestNewService(t *testing.T) {
	t.Parallel()

	validGiftDefaults := config.GiftDefaults{FundID: "fund-123", Type: "Donation"}

	validConfig := Config{
		Blackbaud:       &blackbaud.Client{},
		DonationTracker: &mockDonationTracker{},
		FundraiseUp:     &fundraiseup.Client{},
		GiftDefaults:    validGiftDefaults,
		Logger:          slog.Default(),
		StateStore:      &mockStateStore{},
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
				DonationTracker: &mockDonationTracker{},
				FundraiseUp:     &fundraiseup.Client{},
				GiftDefaults:    validGiftDefaults,
				StateStore:      &mockStateStore{},
			},
			wantErr: true,
			errMsg:  "blackbaud client is required",
		},
		"missing donation tracker": {
			config: Config{
				Blackbaud:    &blackbaud.Client{},
				FundraiseUp:  &fundraiseup.Client{},
				GiftDefaults: validGiftDefaults,
				StateStore:   &mockStateStore{},
			},
			wantErr: true,
			errMsg:  "donation tracker is required",
		},
		"missing fundraiseup client": {
			config: Config{
				Blackbaud:       &blackbaud.Client{},
				DonationTracker: &mockDonationTracker{},
				GiftDefaults:    validGiftDefaults,
				StateStore:      &mockStateStore{},
			},
			wantErr: true,
			errMsg:  "fundraiseup client is required",
		},
		"missing gift defaults fund id": {
			config: Config{
				Blackbaud:       &blackbaud.Client{},
				DonationTracker: &mockDonationTracker{},
				FundraiseUp:     &fundraiseup.Client{},
				StateStore:      &mockStateStore{},
			},
			wantErr: true,
			errMsg:  "gift defaults fund ID is required",
		},
		"missing state store": {
			config: Config{
				Blackbaud:       &blackbaud.Client{},
				DonationTracker: &mockDonationTracker{},
				FundraiseUp:     &fundraiseup.Client{},
				GiftDefaults:    validGiftDefaults,
			},
			wantErr: true,
			errMsg:  "state store is required",
		},
		"nil logger uses default": {
			config: Config{
				Blackbaud:       &blackbaud.Client{},
				DonationTracker: &mockDonationTracker{},
				FundraiseUp:     &fundraiseup.Client{},
				GiftDefaults:    validGiftDefaults,
				StateStore:      &mockStateStore{},
			},
			wantErr: false,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			svc, err := NewService(tc.config)

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
				Blackbaud:       &blackbaud.Client{},
				DonationTracker: &mockDonationTracker{},
				FundraiseUp:     &fundraiseup.Client{},
				GiftDefaults:    config.GiftDefaults{FundID: "fund-123"},
				StateStore:      &mockStateStore{},
			},
			wantErr: false,
		},
		"all fields missing": {
			config:  Config{},
			wantErr: true,
			errFragments: []string{
				"blackbaud client is required",
				"donation tracker is required",
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

	testTime := time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC)

	tests := map[string]struct {
		donation fundraiseup.Donation
		tracker  *mockDonationTracker
		want     recurringContext
		wantErr  bool
	}{
		"non-recurring donation returns empty context": {
			donation: fundraiseup.Donation{
				ID:            "don_123",
				RecurringPlan: nil,
			},
			tracker: &mockDonationTracker{},
			want:    recurringContext{},
			wantErr: false,
		},
		"first recurring donation returns isFirstInSeries true": {
			donation: fundraiseup.Donation{
				ID:            "don_123",
				Installment:   "1",
				RecurringPlan: &fundraiseup.RecurringPlan{ID: "rec_456"},
			},
			tracker: &mockDonationTracker{},
			want: recurringContext{
				isFirstInSeries: true,
				sequenceNumber:  1,
			},
			wantErr: false,
		},
		"subsequent recurring donation returns first gift ID": {
			donation: fundraiseup.Donation{
				ID:            "don_002",
				Installment:   "2",
				RecurringPlan: &fundraiseup.RecurringPlan{ID: "rec_456"},
			},
			tracker: &mockDonationTracker{
				recurringRecords: map[string][]storage.RecurringInfo{
					"rec_456": {
						{
							DonationID:     "don_001",
							GiftID:         "gift_001",
							RecurringID:    "rec_456",
							FirstGiftID:    "gift_001",
							SequenceNumber: 1,
							CreatedAt:      testTime,
						},
					},
				},
			},
			want: recurringContext{
				firstGiftID:     "gift_001",
				isFirstInSeries: false,
				sequenceNumber:  2,
			},
			wantErr: false,
		},
		"uses installment number for sequence even without prior records": {
			donation: fundraiseup.Donation{
				ID:            "don_003",
				Installment:   "3",
				RecurringPlan: &fundraiseup.RecurringPlan{ID: "rec_456"},
			},
			tracker: &mockDonationTracker{},
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
				donationTracker: tc.tracker,
			}

			got, err := svc.getRecurringContext(context.Background(), tc.donation)

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
		wantLinkedGifts []string
		wantLookupID    string
		wantSubtype     string
	}{
		"one-off donation has no recurring attributes": {
			donation: fundraiseup.Donation{
				ID:            "don_123",
				Amount:        "50.00",
				CreatedAt:     testTime,
				RecurringPlan: nil,
			},
			recCtx:          recurringContext{},
			wantLinkedGifts: nil,
			wantLookupID:    "",
			wantSubtype:     "",
		},
		"first recurring donation has LookupID and Subtype but no LinkedGifts": {
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
			wantLinkedGifts: nil,
			wantLookupID:    "rec_456",
			wantSubtype:     "Recurring",
		},
		"subsequent recurring donation has LinkedGifts": {
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
			wantLinkedGifts: []string{"gift_001"},
			wantLookupID:    "rec_456",
			wantSubtype:     "Recurring",
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
			require.Equal(t, tc.wantLinkedGifts, got.LinkedGifts)
			require.Equal(t, tc.wantLookupID, got.LookupID)
			require.Equal(t, tc.wantSubtype, got.Subtype)
			require.Equal(t, "Donation", got.Type)
			require.Len(t, got.GiftSplits, 1)
			require.Equal(t, "fund-123", got.GiftSplits[0].FundID)
		})
	}
}

func TestTrackDonation(t *testing.T) {
	t.Parallel()

	testTime := time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC)

	tests := map[string]struct {
		donation            fundraiseup.Donation
		giftID              string
		recCtx              recurringContext
		wantTrackCalled     bool
		wantTrackRecurCalls int
	}{
		"one-off donation uses Track": {
			donation: fundraiseup.Donation{
				ID:            "don_123",
				RecurringPlan: nil,
				CreatedAt:     testTime,
			},
			giftID:              "gift_456",
			recCtx:              recurringContext{},
			wantTrackCalled:     true,
			wantTrackRecurCalls: 0,
		},
		"first recurring donation uses TrackRecurring with self as first gift": {
			donation: fundraiseup.Donation{
				ID:            "don_123",
				RecurringPlan: &fundraiseup.RecurringPlan{ID: "rec_789"},
				CreatedAt:     testTime,
			},
			giftID: "gift_456",
			recCtx: recurringContext{
				isFirstInSeries: true,
				sequenceNumber:  1,
			},
			wantTrackCalled:     false,
			wantTrackRecurCalls: 1,
		},
		"subsequent recurring donation uses TrackRecurring with existing first gift": {
			donation: fundraiseup.Donation{
				ID:            "don_124",
				RecurringPlan: &fundraiseup.RecurringPlan{ID: "rec_789"},
				CreatedAt:     testTime,
			},
			giftID: "gift_457",
			recCtx: recurringContext{
				firstGiftID:     "gift_001",
				isFirstInSeries: false,
				sequenceNumber:  2,
			},
			wantTrackCalled:     false,
			wantTrackRecurCalls: 1,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			tracker := &mockDonationTracker{}
			svc := &Service{
				donationTracker: tracker,
			}

			err := svc.trackDonation(context.Background(), tc.donation, tc.giftID, tc.recCtx)

			require.NoError(t, err)

			if tc.wantTrackCalled {
				require.Equal(t, tc.giftID, tracker.giftIDs[tc.donation.ID])
			}

			if tc.wantTrackRecurCalls > 0 {
				require.Len(t, tracker.recurringRecords[tc.donation.RecurringID()], tc.wantTrackRecurCalls)
			}
		})
	}
}
