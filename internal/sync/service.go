package sync

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/peteski22/giftbridge/internal/blackbaud"
	"github.com/peteski22/giftbridge/internal/config"
	"github.com/peteski22/giftbridge/internal/fundraiseup"
	"github.com/peteski22/giftbridge/internal/storage"
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

	// GiftDefaults contains default values for gifts in Raiser's Edge.
	GiftDefaults config.GiftDefaults

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
	if c.GiftDefaults.FundID == "" {
		errs = append(errs, errors.New("gift defaults fund ID is required"))
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

	// giftCache caches constituent gifts during initial sync to avoid repeated API calls.
	giftCache map[string][]blackbaud.Gift

	// giftDefaults contains default values for gifts.
	giftDefaults config.GiftDefaults

	// isInitialSync indicates if this is the first sync run.
	isInitialSync bool

	// logger is the structured logger.
	logger *slog.Logger

	// stateStore manages sync state persistence.
	stateStore StateStore
}

// recurringContext contains context for processing a recurring donation.
type recurringContext struct {
	// firstGiftID is the Blackbaud gift ID of the first payment in this series.
	firstGiftID string

	// isFirstInSeries indicates if this is the first payment in the series.
	isFirstInSeries bool

	// sequenceNumber is the position of this payment in the series (1-indexed).
	sequenceNumber int
}

// Run executes a full sync cycle.
func (s *Service) Run(ctx context.Context) (*Result, error) {
	result := &Result{}

	// Get last sync time.
	since, err := s.stateStore.LastSyncTime(ctx)
	if err != nil {
		return nil, fmt.Errorf("getting last sync time: %w", err)
	}

	// Determine if this is an initial sync (no previous sync recorded).
	s.isInitialSync = since.IsZero()
	if s.isInitialSync {
		since = defaultSyncStart()
		s.giftCache = make(map[string][]blackbaud.Gift)
		s.logger.Info("initial sync detected, checking Blackbaud for existing gifts", "since", since)
	}

	s.logger.Info("starting sync", "since", since, "initial_sync", s.isInitialSync)

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
		if donationResult.GiftSkippedExisting {
			result.GiftsSkippedExisting++
		}

		s.logger.Info("processed donation",
			"donation_id", donation.ID,
			"gift_id", donationResult.GiftID,
			"created", donationResult.GiftCreated,
			"updated", donationResult.GiftUpdated,
			"skipped_existing", donationResult.GiftSkippedExisting)
	}

	// Update last sync time.
	if err := s.stateStore.SetLastSyncTime(ctx, time.Now()); err != nil {
		return result, fmt.Errorf("updating last sync time: %w", err)
	}

	s.logger.Info("sync completed",
		"donations_processed", result.DonationsProcessed,
		"gifts_created", result.GiftsCreated,
		"gifts_updated", result.GiftsUpdated,
		"gifts_skipped_existing", result.GiftsSkippedExisting,
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

func (s *Service) getRecurringContext(ctx context.Context, donation fundraiseup.Donation) (recurringContext, error) {
	if !donation.IsRecurring() || donation.RecurringID() == "" {
		return recurringContext{}, nil
	}

	// Use FundraiseUp's installment number for sequence if available.
	// This avoids GSI eventual consistency issues when processing multiple
	// installments from the same series in a single sync run.
	seqNum := donation.InstallmentNumber()
	if seqNum < 1 {
		seqNum = 1
	}

	isFirst := seqNum == 1

	// For subsequent payments, query for the first gift ID to link to.
	if !isFirst {
		existing, err := s.donationTracker.DonationsByRecurringID(ctx, donation.RecurringID())
		if err != nil {
			return recurringContext{}, fmt.Errorf("querying recurring donations: %w", err)
		}

		if len(existing) > 0 {
			return recurringContext{
				firstGiftID:     existing[0].FirstGiftID,
				isFirstInSeries: false,
				sequenceNumber:  seqNum,
			}, nil
		}
		// No existing records found - treat as first even if installment > 1.
		// This handles cases where we missed earlier installments.
		isFirst = true
	}

	return recurringContext{
		isFirstInSeries: isFirst,
		sequenceNumber:  seqNum,
	}, nil
}

func (s *Service) mapDonationToGift(donation fundraiseup.Donation, recCtx recurringContext) (*blackbaud.Gift, error) {
	gift, err := donation.ToDomainType()
	if err != nil {
		return nil, fmt.Errorf("converting donation to gift: %w", err)
	}

	// Common fields matching FundraiseUp integration.
	gift.BatchPrefix = "FundraiseUp"
	gift.IsManual = true
	gift.GiftSplits = []blackbaud.GiftSplit{{
		Amount:     gift.Amount,
		FundID:     s.giftDefaults.FundID,
		CampaignID: s.giftDefaults.CampaignID,
		AppealID:   s.giftDefaults.AppealID,
	}}

	if donation.IsRecurring() && donation.RecurringID() != "" {
		// Recurring donations use RecurringGift or RecurringGiftPayment types.
		gift.LookupID = donation.RecurringID()
		gift.Subtype = blackbaud.GiftSubtypeRecurring
		gift.Origin = blackbaud.GiftOrigin{
			DonationID: donation.ID,
			Name:       "FundraiseUp",
		}.String()

		if recCtx.isFirstInSeries {
			gift.Type = blackbaud.GiftTypeRecurringGift
		} else {
			gift.Type = blackbaud.GiftTypeRecurringGiftPayment
			if recCtx.firstGiftID != "" {
				gift.LinkedGifts = []string{recCtx.firstGiftID}
			}
		}
	} else {
		// One-off donations use the configured gift type.
		gift.Type = blackbaud.GiftType(s.giftDefaults.Type)
		gift.LookupID = donation.ID
	}

	return gift, nil
}

// findExistingGiftByLookupID searches Blackbaud for an existing gift with the given lookup ID.
// Uses s.giftCache to avoid repeated API calls for the same constituent during initial sync.
func (s *Service) findExistingGiftByLookupID(
	ctx context.Context,
	constituentID string,
	lookupID string,
	giftType blackbaud.GiftType,
) (*blackbaud.Gift, error) {
	if lookupID == "" {
		return nil, nil
	}

	// Check cache first.
	gifts, cached := s.giftCache[constituentID]
	if !cached {
		// Query gifts for this constituent, filtered by type for efficiency.
		var err error
		gifts, err = s.blackbaud.ListGiftsByConstituent(ctx, constituentID, []blackbaud.GiftType{giftType})
		if err != nil {
			return nil, fmt.Errorf("listing constituent gifts: %w", err)
		}
		s.giftCache[constituentID] = gifts
	}

	// Find a gift with matching lookup_id.
	for i := range gifts {
		if gifts[i].LookupID == lookupID {
			return &gifts[i], nil
		}
	}

	return nil, nil
}

func (s *Service) trackDonation(
	ctx context.Context,
	donation fundraiseup.Donation,
	giftID string,
	recCtx recurringContext,
) error {
	// Use simple tracking for one-off donations.
	if !donation.IsRecurring() || donation.RecurringID() == "" {
		return s.donationTracker.Track(ctx, donation.ID, giftID)
	}

	// Determine first gift ID for recurring series.
	firstGiftID := recCtx.firstGiftID
	if recCtx.isFirstInSeries {
		firstGiftID = giftID
	}

	return s.donationTracker.TrackRecurring(ctx, storage.RecurringInfo{
		CreatedAt:      donation.CreatedAt,
		DonationID:     donation.ID,
		FirstGiftID:    firstGiftID,
		GiftID:         giftID,
		RecurringID:    donation.RecurringID(),
		SequenceNumber: recCtx.sequenceNumber,
	})
}

func (s *Service) processDonation(
	ctx context.Context,
	donation fundraiseup.Donation,
) DonationResult {
	result := DonationResult{DonationID: donation.ID}

	// Check if already synced in our tracker.
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

	// Get recurring context for recurring donations.
	recCtx, err := s.getRecurringContext(ctx, donation)
	if err != nil {
		result.Error = fmt.Errorf("getting recurring context: %w", err)
		return result
	}

	// Map donation to gift with recurring attributes.
	gift, err := s.mapDonationToGift(donation, recCtx)
	if err != nil {
		result.Error = fmt.Errorf("mapping donation to gift: %w", err)
		return result
	}
	gift.ConstituentID = constituentID

	if existingGiftID != "" {
		// Update existing gift.
		if err := s.blackbaud.UpdateGift(ctx, existingGiftID, gift); err != nil {
			result.Error = fmt.Errorf("updating gift: %w", err)
			return result
		}
		result.GiftID = existingGiftID
		result.GiftUpdated = true
	} else {
		// During initial sync, check Blackbaud for existing gifts to avoid duplicates.
		if s.isInitialSync {
			existingGift, err := s.findExistingGiftByLookupID(ctx, constituentID, gift.LookupID, gift.Type)
			if err != nil {
				result.Error = fmt.Errorf("checking for existing gift: %w", err)
				return result
			}
			if existingGift != nil {
				s.logger.Warn("gift already exists in Blackbaud, skipping creation",
					"donation_id", donation.ID,
					"lookup_id", gift.LookupID,
					"existing_gift_id", existingGift.ID)
				result.GiftID = existingGift.ID
				result.GiftSkippedExisting = true

				// Track the existing gift so we don't check again.
				if err := s.trackDonation(ctx, donation, existingGift.ID, recCtx); err != nil {
					result.Error = fmt.Errorf("tracking existing gift: %w", err)
					return result
				}
				return result
			}
		}

		// Create new gift.
		giftID, err := s.blackbaud.CreateGift(ctx, gift)
		if err != nil {
			result.Error = fmt.Errorf("creating gift: %w", err)
			return result
		}
		result.GiftID = giftID
		result.GiftCreated = true
	}

	// Track the mapping (upserts, so safe for both create and update paths).
	// This ensures recurring metadata is stored even for previously-synced
	// donations that were tracked before recurring support was added.
	if err := s.trackDonation(ctx, donation, result.GiftID, recCtx); err != nil {
		result.Error = fmt.Errorf("tracking donation: %w", err)
		return result
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
		giftDefaults:    cfg.GiftDefaults,
		logger:          logger,
		stateStore:      cfg.StateStore,
	}, nil
}

func defaultSyncStart() time.Time {
	return time.Now().AddDate(0, 0, -30)
}
