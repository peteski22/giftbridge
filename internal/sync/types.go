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

	// GiftSkippedExisting indicates the gift already existed in Blackbaud.
	GiftSkippedExisting bool

	// GiftUpdated indicates if an existing gift was updated.
	GiftUpdated bool
}

// Result contains the outcome of a sync operation.
type Result struct {
	// ConstituentsCreated is the number of new constituents created.
	ConstituentsCreated int

	// ConstituentsExisting is the number of constituents that already existed.
	ConstituentsExisting int

	// DonationsProcessed is the total number of donations processed.
	DonationsProcessed int

	// DryRun indicates this was a dry-run (no writes to Blackbaud).
	DryRun bool

	// Errors contains any errors that occurred during the sync.
	Errors []error

	// GiftsCreated is the number of new gifts created.
	GiftsCreated int

	// GiftsSkippedExisting is the number of gifts skipped because they already existed.
	GiftsSkippedExisting int

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
