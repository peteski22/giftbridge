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

	// GiftDefaults contains default values for gifts in Raiser's Edge.
	GiftDefaults config.GiftDefaults

	// Logger is the structured logger for the service.
	Logger *slog.Logger

	// StateStore manages sync state persistence.
	StateStore StateStore
}

// validate checks that all required Config fields are set.
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
	blackbaud       *blackbaud.Client
	donationTracker DonationTracker
	fundraiseup     *fundraiseup.Client
	giftCache       map[string][]blackbaud.Gift
	giftDefaults    config.GiftDefaults
	isInitialSync   bool
	logger          *slog.Logger
	stateStore      StateStore
}

// recurringContext contains context for processing a recurring donation.
type recurringContext struct {
	firstGiftID     string
	isFirstInSeries bool
	sequenceNumber  int
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

// Run executes a full sync cycle.
func (s *Service) Run(ctx context.Context) (*Result, error) {
	result := &Result{}

	since, err := s.stateStore.LastSyncTime(ctx)
	if err != nil {
		return nil, fmt.Errorf("getting last sync time: %w", err)
	}

	s.isInitialSync = since.IsZero()
	if s.isInitialSync {
		since = defaultSyncStart()
		s.giftCache = make(map[string][]blackbaud.Gift)
		s.logger.Info("initial sync detected, checking Blackbaud for existing gifts", "since", since)
	}

	s.logger.Info("starting sync", "since", since, "initial_sync", s.isInitialSync)

	donations, err := s.fundraiseup.Donations(ctx, since)
	if err != nil {
		return nil, fmt.Errorf("fetching donations: %w", err)
	}

	s.logger.Info("fetched donations", "count", len(donations))

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

func (s *Service) findExistingGiftByLookupID(
	ctx context.Context,
	constituentID string,
	lookupID string,
	giftType blackbaud.GiftType,
) (*blackbaud.Gift, error) {
	if lookupID == "" {
		return nil, nil
	}

	gifts, cached := s.giftCache[constituentID]
	if !cached {
		var err error
		gifts, err = s.blackbaud.ListGiftsByConstituent(ctx, constituentID, []blackbaud.GiftType{giftType})
		if err != nil {
			return nil, fmt.Errorf("listing constituent gifts: %w", err)
		}
		s.giftCache[constituentID] = gifts
	}

	for i := range gifts {
		if gifts[i].LookupID == lookupID {
			return &gifts[i], nil
		}
	}

	return nil, nil
}

func (s *Service) findOrCreateConstituent(
	ctx context.Context,
	donation fundraiseup.Donation,
) (string, bool, error) {
	if donation.Supporter == nil {
		return "", false, errors.New("donation has no supporter")
	}

	supporter := donation.Supporter

	if supporter.Email != "" {
		constituents, err := s.blackbaud.SearchConstituents(ctx, supporter.Email)
		if err != nil {
			return "", false, fmt.Errorf("searching constituents: %w", err)
		}

		if len(constituents) > 0 {
			return constituents[0].ID, false, nil
		}
	}

	constituent := supporter.ToDomainType()
	constituentID, err := s.blackbaud.CreateConstituent(ctx, constituent)
	if err != nil {
		return "", false, fmt.Errorf("creating constituent: %w", err)
	}

	return constituentID, true, nil
}

func (s *Service) getRecurringContext(
	ctx context.Context,
	donation fundraiseup.Donation,
) (recurringContext, error) {
	if !donation.IsRecurring() || donation.RecurringID() == "" {
		return recurringContext{}, nil
	}

	seqNum := donation.InstallmentNumber()
	if seqNum < 1 {
		seqNum = 1
	}

	isFirst := seqNum == 1

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
		isFirst = true
	}

	return recurringContext{
		isFirstInSeries: isFirst,
		sequenceNumber:  seqNum,
	}, nil
}

func (s *Service) mapDonationToGift(
	donation fundraiseup.Donation,
	recCtx recurringContext,
) (*blackbaud.Gift, error) {
	gift, err := donation.ToDomainType()
	if err != nil {
		return nil, fmt.Errorf("converting donation to gift: %w", err)
	}

	gift.BatchPrefix = "FundraiseUp"
	gift.IsManual = true
	gift.GiftSplits = []blackbaud.GiftSplit{{
		Amount:     gift.Amount,
		FundID:     s.giftDefaults.FundID,
		CampaignID: s.giftDefaults.CampaignID,
		AppealID:   s.giftDefaults.AppealID,
	}}

	if donation.IsRecurring() && donation.RecurringID() != "" {
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
		gift.Type = blackbaud.GiftType(s.giftDefaults.Type)
		gift.LookupID = donation.ID
	}

	return gift, nil
}

func (s *Service) processDonation(
	ctx context.Context,
	donation fundraiseup.Donation,
) DonationResult {
	result := DonationResult{DonationID: donation.ID}

	existingGiftID, err := s.donationTracker.GiftID(ctx, donation.ID)
	if err != nil {
		result.Error = fmt.Errorf("checking donation tracker: %w", err)
		return result
	}

	constituentID, created, err := s.findOrCreateConstituent(ctx, donation)
	if err != nil {
		result.Error = fmt.Errorf("finding/creating constituent: %w", err)
		return result
	}
	result.ConstituentCreated = created

	recCtx, err := s.getRecurringContext(ctx, donation)
	if err != nil {
		result.Error = fmt.Errorf("getting recurring context: %w", err)
		return result
	}

	gift, err := s.mapDonationToGift(donation, recCtx)
	if err != nil {
		result.Error = fmt.Errorf("mapping donation to gift: %w", err)
		return result
	}
	gift.ConstituentID = constituentID

	if existingGiftID != "" {
		if err := s.blackbaud.UpdateGift(ctx, existingGiftID, gift); err != nil {
			result.Error = fmt.Errorf("updating gift: %w", err)
			return result
		}
		result.GiftID = existingGiftID
		result.GiftUpdated = true
	} else {
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

				if err := s.trackDonation(ctx, donation, existingGift.ID, recCtx); err != nil {
					result.Error = fmt.Errorf("tracking existing gift: %w", err)
					return result
				}
				return result
			}
		}

		giftID, err := s.blackbaud.CreateGift(ctx, gift)
		if err != nil {
			result.Error = fmt.Errorf("creating gift: %w", err)
			return result
		}
		result.GiftID = giftID
		result.GiftCreated = true
	}

	if err := s.trackDonation(ctx, donation, result.GiftID, recCtx); err != nil {
		result.Error = fmt.Errorf("tracking donation: %w", err)
		return result
	}

	return result
}

func (s *Service) trackDonation(
	ctx context.Context,
	donation fundraiseup.Donation,
	giftID string,
	recCtx recurringContext,
) error {
	if !donation.IsRecurring() || donation.RecurringID() == "" {
		return s.donationTracker.Track(ctx, donation.ID, giftID)
	}

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

func defaultSyncStart() time.Time {
	return time.Now().AddDate(0, 0, -30)
}
