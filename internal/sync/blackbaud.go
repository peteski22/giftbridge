package sync

import (
	"context"

	"github.com/peteski22/giftbridge/internal/blackbaud"
)

// BlackbaudClient defines the Blackbaud operations required by the sync service.
type BlackbaudClient interface {
	// CreateConstituent creates a new constituent and returns the new constituent ID.
	CreateConstituent(ctx context.Context, constituent *blackbaud.Constituent) (string, error)

	// CreateGift creates a new gift and returns the new gift ID.
	CreateGift(ctx context.Context, gift *blackbaud.Gift) (string, error)

	// ListGiftsByConstituent returns all gifts for a constituent, optionally filtered by gift type.
	ListGiftsByConstituent(
		ctx context.Context,
		constituentID string,
		giftTypes []blackbaud.GiftType,
	) ([]blackbaud.Gift, error)

	// SearchConstituents searches for constituents matching the given email address.
	SearchConstituents(ctx context.Context, email string) ([]blackbaud.Constituent, error)

	// UpdateGift updates an existing gift by ID.
	UpdateGift(ctx context.Context, giftID string, gift *blackbaud.Gift) error
}
