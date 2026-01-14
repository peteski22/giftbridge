// Package sync provides orchestration for syncing donations from FundraiseUp to Blackbaud.
package sync

import (
	"context"
	"time"
)

// DonationResult contains the outcome of processing a single donation.
type DonationResult struct {
	// ConstituentCreated indicates if a new constituent was created.
	ConstituentCreated bool

	// DonationID is the FundraiseUp donation identifier.
	DonationID string

	// Error contains any error that occurred during processing.
	Error error

	// GiftCreated indicates if a new gift was created.
	GiftCreated bool

	// GiftID is the Blackbaud gift identifier.
	GiftID string

	// GiftUpdated indicates if an existing gift was updated.
	GiftUpdated bool
}

// DonationTracker tracks the mapping between FundraiseUp donation IDs and Blackbaud gift IDs.
type DonationTracker interface {
	// GiftID returns the Blackbaud gift ID for a FundraiseUp donation, or empty if not tracked.
	GiftID(ctx context.Context, donationID string) (string, error)

	// Track stores the mapping between a donation ID and gift ID.
	Track(ctx context.Context, donationID string, giftID string) error
}

// Result contains the outcome of a sync operation.
type Result struct {
	// ConstituentsCreated is the number of new constituents created.
	ConstituentsCreated int

	// DonationsProcessed is the total number of donations processed.
	DonationsProcessed int

	// Errors contains any errors that occurred during the sync.
	Errors []error

	// GiftsCreated is the number of new gifts created.
	GiftsCreated int

	// GiftsUpdated is the number of existing gifts updated.
	GiftsUpdated int
}

// StateStore manages persistent state for the sync process.
type StateStore interface {
	// LastSyncTime returns the timestamp of the last successful sync.
	LastSyncTime(ctx context.Context) (time.Time, error)

	// SetLastSyncTime updates the last sync timestamp.
	SetLastSyncTime(ctx context.Context, t time.Time) error
}
