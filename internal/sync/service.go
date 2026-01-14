package sync

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/peteski22/giftbridge/internal/blackbaud"
	"github.com/peteski22/giftbridge/internal/fundraiseup"
)

// Config holds the required configuration for creating a Service.
type Config struct {
	// Blackbaud is the Blackbaud API client.
	Blackbaud *blackbaud.Client

	// DonationTracker tracks donation to gift mappings.
	DonationTracker DonationTracker

	// FundraiseUp is the FundraiseUp API client.
	FundraiseUp *fundraiseup.Client

	// Logger is the structured logger for the service.
	Logger *slog.Logger

	// StateStore manages sync state persistence.
	StateStore StateStore
}

func (c *Config) validate() error {
	var errs []error
	if c.Blackbaud == nil {
		errs = append(errs, errors.New("blackbaud client is required"))
	}
	if c.DonationTracker == nil {
		errs = append(errs, errors.New("donation tracker is required"))
	}
	if c.FundraiseUp == nil {
		errs = append(errs, errors.New("fundraiseup client is required"))
	}
	if c.StateStore == nil {
		errs = append(errs, errors.New("state store is required"))
	}
	return errors.Join(errs...)
}

// Service orchestrates the sync between FundraiseUp and Blackbaud.
type Service struct {
	// blackbaud is the Blackbaud API client.
	blackbaud *blackbaud.Client

	// donationTracker tracks donation to gift mappings.
	donationTracker DonationTracker

	// fundraiseup is the FundraiseUp API client.
	fundraiseup *fundraiseup.Client

	// logger is the structured logger.
	logger *slog.Logger

	// stateStore manages sync state persistence.
	stateStore StateStore
}

// Run executes a full sync cycle.
func (s *Service) Run(ctx context.Context) (*Result, error) {
	result := &Result{}

	// Get last sync time.
	since, err := s.stateStore.LastSyncTime(ctx)
	if err != nil {
		return nil, fmt.Errorf("getting last sync time: %w", err)
	}

	// Use default if no previous sync.
	if since.IsZero() {
		since = defaultSyncStart()
		s.logger.Info("no previous sync found, using default start time", "since", since)
	}

	s.logger.Info("starting sync", "since", since)

	// Fetch donations from FundraiseUp.
	donations, err := s.fundraiseup.Donations(ctx, since)
	if err != nil {
		return nil, fmt.Errorf("fetching donations: %w", err)
	}

	s.logger.Info("fetched donations", "count", len(donations))

	// Process each donation.
	for _, donation := range donations {
		donationResult := s.processDonation(ctx, donation)
		result.DonationsProcessed++

		if donationResult.Error != nil {
			result.Errors = append(result.Errors, donationResult.Error)
			s.logger.Error("failed to process donation",
				"donation_id", donation.ID,
				"error", donationResult.Error)
			continue
		}

		if donationResult.ConstituentCreated {
			result.ConstituentsCreated++
		}
		if donationResult.GiftCreated {
			result.GiftsCreated++
		}
		if donationResult.GiftUpdated {
			result.GiftsUpdated++
		}

		s.logger.Info("processed donation",
			"donation_id", donation.ID,
			"gift_id", donationResult.GiftID,
			"created", donationResult.GiftCreated,
			"updated", donationResult.GiftUpdated)
	}

	// Update last sync time.
	if err := s.stateStore.SetLastSyncTime(ctx, time.Now()); err != nil {
		return result, fmt.Errorf("updating last sync time: %w", err)
	}

	s.logger.Info("sync completed",
		"donations_processed", result.DonationsProcessed,
		"gifts_created", result.GiftsCreated,
		"gifts_updated", result.GiftsUpdated,
		"constituents_created", result.ConstituentsCreated,
		"errors", len(result.Errors))

	return result, nil
}

func (s *Service) findOrCreateConstituent(ctx context.Context, donation fundraiseup.Donation) (string, bool, error) {
	if donation.Supporter == nil {
		return "", false, errors.New("donation has no supporter")
	}

	supporter := donation.Supporter

	// Search by email.
	if supporter.Email != "" {
		constituents, err := s.blackbaud.SearchConstituents(ctx, supporter.Email)
		if err != nil {
			return "", false, fmt.Errorf("searching constituents: %w", err)
		}

		if len(constituents) > 0 {
			return constituents[0].ID, false, nil
		}
	}

	// Create new constituent.
	constituent := supporter.ToDomainType()
	constituentID, err := s.blackbaud.CreateConstituent(ctx, constituent)
	if err != nil {
		return "", false, fmt.Errorf("creating constituent: %w", err)
	}

	return constituentID, true, nil
}

func (s *Service) processDonation(ctx context.Context, donation fundraiseup.Donation) DonationResult {
	result := DonationResult{DonationID: donation.ID}

	// Check if already synced.
	existingGiftID, err := s.donationTracker.GiftID(ctx, donation.ID)
	if err != nil {
		result.Error = fmt.Errorf("checking donation tracker: %w", err)
		return result
	}

	// Find or create constituent.
	constituentID, created, err := s.findOrCreateConstituent(ctx, donation)
	if err != nil {
		result.Error = fmt.Errorf("finding/creating constituent: %w", err)
		return result
	}
	result.ConstituentCreated = created

	// Map donation to gift.
	gift := donation.ToDomainType(constituentID)

	if existingGiftID != "" {
		// Update existing gift.
		if err := s.blackbaud.UpdateGift(ctx, existingGiftID, gift); err != nil {
			result.Error = fmt.Errorf("updating gift: %w", err)
			return result
		}
		result.GiftID = existingGiftID
		result.GiftUpdated = true
	} else {
		// Create new gift.
		giftID, err := s.blackbaud.CreateGift(ctx, gift)
		if err != nil {
			result.Error = fmt.Errorf("creating gift: %w", err)
			return result
		}
		result.GiftID = giftID
		result.GiftCreated = true

		// Track the mapping.
		if err := s.donationTracker.Track(ctx, donation.ID, giftID); err != nil {
			result.Error = fmt.Errorf("tracking donation: %w", err)
			return result
		}
	}

	return result
}

// NewService creates a new sync orchestration service.
func NewService(cfg Config) (*Service, error) {
	if err := cfg.validate(); err != nil {
		return nil, fmt.Errorf("invalid config: %w", err)
	}

	logger := cfg.Logger
	if logger == nil {
		logger = slog.Default()
	}

	return &Service{
		blackbaud:       cfg.Blackbaud,
		donationTracker: cfg.DonationTracker,
		fundraiseup:     cfg.FundraiseUp,
		logger:          logger,
		stateStore:      cfg.StateStore,
	}, nil
}

func defaultSyncStart() time.Time {
	return time.Now().AddDate(0, 0, -30)
}
