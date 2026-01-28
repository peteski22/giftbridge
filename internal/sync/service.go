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
)

const (
	defaultSyncDays = -30
	originName      = "FundraiseUp"

	// defaultMaxDonationsPerRun limits donations processed per Lambda invocation.
	// This limit exists because pending donation IDs are stored in SSM Parameter Store
	// which has a 4KB size limit. With 8-character donation IDs stored as comma-separated
	// values, we can safely store ~400 IDs. Setting to 300 provides headroom.
	// If you have sustained volumes exceeding 300 donations per sync interval,
	// consider increasing the sync frequency (e.g., every 15 minutes instead of hourly).
	defaultMaxDonationsPerRun = 300
)

// Config holds the required configuration for creating a Service.
type Config struct {
	// Blackbaud is the Blackbaud API client.
	Blackbaud BlackbaudClient

	// DryRun indicates whether to skip writes to Blackbaud.
	DryRun bool

	// FundraiseUp is the FundraiseUp API client.
	FundraiseUp *fundraiseup.Client

	// GiftDefaults contains default values for gifts in Raiser's Edge.
	GiftDefaults config.GiftDefaults

	// Logger is the structured logger for the service.
	Logger *slog.Logger

	// MaxDonationsPerRun limits donations processed per Lambda invocation.
	// Default is 300. This limit exists because pending donation IDs are stored
	// in SSM Parameter Store (4KB limit). Do not exceed 400.
	MaxDonationsPerRun int

	// SinceOverride optionally overrides the last sync time.
	SinceOverride *time.Time

	// StateStore manages sync state persistence.
	StateStore StateStore
}

// validate checks that all required Config fields are set.
func (c *Config) validate() error {
	var errs []error
	if c.Blackbaud == nil {
		errs = append(errs, errors.New("blackbaud client is required"))
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
	blackbaud          BlackbaudClient
	dryRun             bool
	fundraiseup        *fundraiseup.Client
	giftCache          map[string][]blackbaud.Gift
	giftDefaults       config.GiftDefaults
	logger             *slog.Logger
	maxDonationsPerRun int
	sinceOverride      *time.Time
	stateStore         StateStore
}

// recurringContext contains context for processing a recurring donation.
type recurringContext struct {
	firstGiftID     string
	isFirstInSeries bool
	sequenceNumber  int
}

// New creates a new sync orchestration service.
func New(cfg Config) (*Service, error) {
	if err := cfg.validate(); err != nil {
		return nil, fmt.Errorf("invalid config: %w", err)
	}

	logger := cfg.Logger
	if logger == nil {
		logger = slog.Default()
	}

	bbClient := cfg.Blackbaud
	if cfg.DryRun {
		bbClient = newDryRunClient(cfg.Blackbaud, logger)
	}

	maxDonations := cfg.MaxDonationsPerRun
	if maxDonations <= 0 {
		maxDonations = defaultMaxDonationsPerRun
	}

	return &Service{
		blackbaud:          bbClient,
		dryRun:             cfg.DryRun,
		fundraiseup:        cfg.FundraiseUp,
		giftDefaults:       cfg.GiftDefaults,
		logger:             logger,
		maxDonationsPerRun: maxDonations,
		sinceOverride:      cfg.SinceOverride,
		stateStore:         cfg.StateStore,
	}, nil
}

// Run executes a full sync cycle.
func (s *Service) Run(ctx context.Context) (*Result, error) {
	result := &Result{DryRun: s.dryRun}

	// Initialize gift cache for Blackbaud lookups (sized for worst case: one constituent per donation).
	s.giftCache = make(map[string][]blackbaud.Gift, s.maxDonationsPerRun)

	// Check for pending donations from a previous interrupted run.
	pendingIDs, err := s.stateStore.PendingDonationIDs(ctx)
	if err != nil {
		return nil, fmt.Errorf("getting pending donation IDs: %w", err)
	}

	if len(pendingIDs) > 0 {
		// Resume processing pending donations.
		return s.runResume(ctx, result, pendingIDs)
	}

	// Fresh sync - fetch donations and process.
	return s.runFresh(ctx, result)
}

// runFresh executes a fresh sync cycle, fetching all donations since last sync.
func (s *Service) runFresh(ctx context.Context, result *Result) (*Result, error) {
	since, err := s.stateStore.LastSyncTime(ctx)
	if err != nil {
		return nil, fmt.Errorf("getting last sync time: %w", err)
	}

	// Allow override for testing.
	if s.sinceOverride != nil {
		since = *s.sinceOverride
		s.logger.Info("using override sync time", "since", since)
	}

	if since.IsZero() {
		since = defaultSyncStart()
		s.logger.Info("initial sync detected", "since", since)
	}

	s.logger.Info("starting fresh sync",
		"since", since,
		"dry_run", s.dryRun,
		"max_donations", s.maxDonationsPerRun)

	donations, err := s.fundraiseup.Donations(ctx, since)
	if err != nil {
		return nil, fmt.Errorf("fetching donations: %w", err)
	}

	s.logger.Info("fetched donations", "count", len(donations))

	if len(donations) == 0 {
		s.logger.Info("no donations to process")
		return result, nil
	}

	// Apply max donations limit.
	if s.maxDonationsPerRun > 0 && len(donations) > s.maxDonationsPerRun {
		s.logger.Info("limiting donations to max per run",
			"total", len(donations),
			"limit", s.maxDonationsPerRun)
		donations = donations[:s.maxDonationsPerRun]
	}

	// Extract IDs for pending list.
	pendingIDs := make([]string, len(donations))
	for i, d := range donations {
		pendingIDs[i] = d.ID
	}

	// Store pending list before processing (skip in dry-run).
	if !s.dryRun {
		if err := s.stateStore.SetPendingDonationIDs(ctx, pendingIDs); err != nil {
			return nil, fmt.Errorf("storing pending donation IDs: %w", err)
		}
	}

	// Process each donation.
	for _, donation := range donations {
		s.processAndRecord(ctx, result, donation)

		// Remove from pending after processing (success or failure).
		if !s.dryRun {
			if err := s.stateStore.RemovePendingDonationID(ctx, donation.ID); err != nil {
				s.logger.Error("failed to remove from pending", "donation_id", donation.ID, "error", err)
			}
		}
	}

	// All done - update sync time.
	if !s.dryRun {
		if err := s.stateStore.SetLastSyncTime(ctx, time.Now()); err != nil {
			return result, fmt.Errorf("updating last sync time: %w", err)
		}
	}

	s.logSyncComplete(result)
	return result, nil
}

// runResume resumes processing from a previous interrupted run.
func (s *Service) runResume(ctx context.Context, result *Result, pendingIDs []string) (*Result, error) {
	s.logger.Info("resuming interrupted sync",
		"pending_count", len(pendingIDs),
		"dry_run", s.dryRun)

	for _, donationID := range pendingIDs {
		// Fetch fresh donation data by ID.
		donation, err := s.fundraiseup.Donation(ctx, donationID)
		if err != nil {
			s.logger.Error("failed to fetch donation for resume",
				"donation_id", donationID,
				"error", err)
			result.Errors = append(result.Errors, fmt.Errorf("fetching donation %s: %w", donationID, err))

			// Remove from pending to avoid infinite retry loop.
			if !s.dryRun {
				if rmErr := s.stateStore.RemovePendingDonationID(ctx, donationID); rmErr != nil {
					s.logger.Error("failed to remove from pending", "donation_id", donationID, "error", rmErr)
				}
			}
			continue
		}

		s.processAndRecord(ctx, result, *donation)

		// Remove from pending after processing.
		if !s.dryRun {
			if err := s.stateStore.RemovePendingDonationID(ctx, donationID); err != nil {
				s.logger.Error("failed to remove from pending", "donation_id", donationID, "error", err)
			}
		}
	}

	// All pending processed - update sync time.
	if !s.dryRun {
		if err := s.stateStore.SetLastSyncTime(ctx, time.Now()); err != nil {
			return result, fmt.Errorf("updating last sync time: %w", err)
		}
	}

	s.logSyncComplete(result)
	return result, nil
}

// processAndRecord processes a single donation and records the result.
func (s *Service) processAndRecord(ctx context.Context, result *Result, donation fundraiseup.Donation) {
	donationResult := s.processDonation(ctx, donation)
	result.DonationsProcessed++

	if donationResult.Error != nil {
		result.Errors = append(result.Errors, donationResult.Error)
		s.logger.Error("failed to process donation",
			"donation_id", donation.ID,
			"error", donationResult.Error)
		return
	}

	if donationResult.ConstituentCreated {
		result.ConstituentsCreated++
	} else {
		result.ConstituentsExisting++
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

// logSyncComplete logs the final sync summary.
func (s *Service) logSyncComplete(result *Result) {
	s.logger.Info("sync completed",
		"donations_processed", result.DonationsProcessed,
		"gifts_created", result.GiftsCreated,
		"gifts_updated", result.GiftsUpdated,
		"gifts_skipped_existing", result.GiftsSkippedExisting,
		"constituents_created", result.ConstituentsCreated,
		"errors", len(result.Errors),
		"dry_run", s.dryRun)
}

// findExistingGift searches Blackbaud for a gift that was already created for this donation.
// For one-time donations, it matches by lookup_id = donation_id.
// For recurring donations, it matches by lookup_id = recurring_id AND origin.donation_id.
// Returns nil if no matching gift exists.
func (s *Service) findExistingGift(
	ctx context.Context,
	constituentID string,
	donation fundraiseup.Donation,
) (*blackbaud.Gift, error) {
	gifts, err := s.getConstituentGifts(ctx, constituentID)
	if err != nil {
		return nil, err
	}

	if donation.IsRecurring() && donation.RecurringID() != "" {
		// For recurring: lookup_id = recurring_id, match by Origin.DonationID.
		lookupID := donation.RecurringID()
		for i := range gifts {
			if gifts[i].LookupID != lookupID {
				continue
			}
			origin, _ := blackbaud.ParseGiftOrigin(gifts[i].Origin)
			if origin.DonationID == donation.ID {
				return &gifts[i], nil
			}
		}
	} else {
		// For one-time: lookup_id = donation_id.
		for i := range gifts {
			if gifts[i].LookupID == donation.ID {
				return &gifts[i], nil
			}
		}
	}

	return nil, nil
}

// findFirstRecurringGift locates the initial RecurringGift in a donation series.
// This is needed to link subsequent RecurringGiftPayment records back to the parent gift.
// Returns nil if no RecurringGift exists for the given recurring ID.
func (s *Service) findFirstRecurringGift(
	ctx context.Context,
	constituentID string,
	recurringID string,
) (*blackbaud.Gift, error) {
	gifts, err := s.getConstituentGifts(ctx, constituentID)
	if err != nil {
		return nil, err
	}

	for i := range gifts {
		if gifts[i].LookupID == recurringID &&
			gifts[i].Type == blackbaud.GiftTypeRecurringGift {
			return &gifts[i], nil
		}
	}

	return nil, nil
}

// findOrCreateConstituent searches for an existing constituent by email, creating one if not found.
// Returns the constituent ID, whether a new constituent was created, and any error.
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

// getConstituentGifts retrieves all gifts for a constituent from Blackbaud.
// Results are cached per-constituent for the duration of the sync run to minimise API calls.
func (s *Service) getConstituentGifts(ctx context.Context, constituentID string) ([]blackbaud.Gift, error) {
	if cached, ok := s.giftCache[constituentID]; ok {
		return cached, nil
	}

	// Fetch all gift types for recurring support.
	gifts, err := s.blackbaud.ListGiftsByConstituent(ctx, constituentID, nil)
	if err != nil {
		return nil, fmt.Errorf("listing constituent gifts: %w", err)
	}

	s.giftCache[constituentID] = gifts
	return gifts, nil
}

// getRecurringContext determines the recurring donation context for gift creation.
// For the first payment in a series, it returns isFirstInSeries=true.
// For subsequent payments, it locates the first gift to enable linking.
// If the first gift cannot be found, it treats this payment as the first in series.
func (s *Service) getRecurringContext(
	ctx context.Context,
	constituentID string,
	donation fundraiseup.Donation,
) (recurringContext, error) {
	if !donation.IsRecurring() || donation.RecurringID() == "" {
		return recurringContext{}, nil
	}

	seqNum := max(donation.InstallmentNumber(), 1)

	isFirst := seqNum == 1

	if !isFirst {
		// Look for the first gift in Blackbaud.
		firstGift, err := s.findFirstRecurringGift(ctx, constituentID, donation.RecurringID())
		if err != nil {
			return recurringContext{}, fmt.Errorf("finding first recurring gift: %w", err)
		}

		if firstGift != nil {
			return recurringContext{
				firstGiftID:     firstGift.ID,
				isFirstInSeries: false,
				sequenceNumber:  seqNum,
			}, nil
		}
		// First gift not found in Blackbaud - treat as first.
		isFirst = true
	}

	return recurringContext{
		isFirstInSeries: isFirst,
		sequenceNumber:  seqNum,
	}, nil
}

// mapDonationToGift converts a FundraiseUp donation to a Blackbaud gift.
// It applies gift defaults (fund, campaign, appeal) and handles recurring gift linking.
// For recurring donations, it sets the appropriate gift type and links to the first gift.
func (s *Service) mapDonationToGift(
	donation fundraiseup.Donation,
	recCtx recurringContext,
) (*blackbaud.Gift, error) {
	gift, err := donation.ToDomainType()
	if err != nil {
		return nil, fmt.Errorf("converting donation to gift: %w", err)
	}

	gift.BatchPrefix = originName
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
			Name:       originName,
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

// processDonation handles the complete sync workflow for a single donation.
// It finds or creates the constituent, checks for existing gifts, and creates the gift if needed.
// Returns a DonationResult containing the outcome and any error encountered.
func (s *Service) processDonation(
	ctx context.Context,
	donation fundraiseup.Donation,
) DonationResult {
	result := DonationResult{DonationID: donation.ID}

	// Find or create constituent first - we need the ID for Blackbaud queries.
	constituentID, created, err := s.findOrCreateConstituent(ctx, donation)
	if err != nil {
		result.Error = fmt.Errorf("finding/creating constituent: %w", err)
		return result
	}
	result.ConstituentCreated = created

	// Check if gift already exists in Blackbaud.
	existingGift, err := s.findExistingGift(ctx, constituentID, donation)
	if err != nil {
		result.Error = fmt.Errorf("checking for existing gift: %w", err)
		return result
	}

	if existingGift != nil {
		// Gift already exists - skip.
		s.logger.Warn("gift already exists in Blackbaud, skipping",
			"donation_id", donation.ID,
			"existing_gift_id", existingGift.ID)
		result.GiftID = existingGift.ID
		result.GiftSkippedExisting = true
		return result
	}

	// Get recurring context for gift mapping.
	recCtx, err := s.getRecurringContext(ctx, constituentID, donation)
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

	giftID, err := s.blackbaud.CreateGift(ctx, gift)
	if err != nil {
		result.Error = fmt.Errorf("creating gift: %w", err)
		return result
	}
	result.GiftID = giftID
	result.GiftCreated = true

	return result
}

// defaultSyncStart returns the default start time for initial syncs.
func defaultSyncStart() time.Time {
	return time.Now().AddDate(0, 0, defaultSyncDays)
}
