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

// mockDonationTracker implements DonationTracker for testing.
type mockDonationTracker struct {
	giftIDs  map[string]string
	trackErr error
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
