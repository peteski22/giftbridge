package sync

import (
	"context"
	"fmt"
	"log/slog"
	"sync/atomic"

	"github.com/peteski22/giftbridge/internal/blackbaud"
)

// dryRunClient wraps a BlackbaudClient and logs write operations instead of executing them.
type dryRunClient struct {
	client  BlackbaudClient
	logger  *slog.Logger
	counter uint64
}

// newDryRunClient creates a new dryRunClient that wraps the given BlackbaudClient.
func newDryRunClient(client BlackbaudClient, logger *slog.Logger) *dryRunClient {
	return &dryRunClient{
		client: client,
		logger: logger,
	}
}

// CreateConstituent logs what would be created and returns a fake ID.
func (d *dryRunClient) CreateConstituent(ctx context.Context, constituent *blackbaud.Constituent) (string, error) {
	fakeID := d.nextFakeID("constituent")

	email := ""
	if constituent.Email != nil {
		email = constituent.Email.Address
	}

	d.logger.Info("[DRY-RUN] would create constituent",
		"fake_id", fakeID,
		"first_name", constituent.FirstName,
		"last_name", constituent.LastName,
		"email", email,
		"type", constituent.Type)

	return fakeID, nil
}

// CreateGift logs what would be created and returns a fake ID.
func (d *dryRunClient) CreateGift(ctx context.Context, gift *blackbaud.Gift) (string, error) {
	fakeID := d.nextFakeID("gift")

	amount := 0.0
	if gift.Amount != nil {
		amount = gift.Amount.Value
	}

	d.logger.Info("[DRY-RUN] would create gift",
		"fake_id", fakeID,
		"amount", amount,
		"type", gift.Type,
		"lookup_id", gift.LookupID,
		"constituent_id", gift.ConstituentID,
		"date", gift.Date)

	return fakeID, nil
}

// ListGiftsByConstituent delegates to the real client.
func (d *dryRunClient) ListGiftsByConstituent(
	ctx context.Context,
	constituentID string,
	giftTypes []blackbaud.GiftType,
) ([]blackbaud.Gift, error) {
	return d.client.ListGiftsByConstituent(ctx, constituentID, giftTypes)
}

// SearchConstituents delegates to the real client.
func (d *dryRunClient) SearchConstituents(ctx context.Context, email string) ([]blackbaud.Constituent, error) {
	return d.client.SearchConstituents(ctx, email)
}

// UpdateGift logs what would be updated and returns nil.
func (d *dryRunClient) UpdateGift(ctx context.Context, giftID string, gift *blackbaud.Gift) error {
	amount := 0.0
	if gift.Amount != nil {
		amount = gift.Amount.Value
	}

	d.logger.Info("[DRY-RUN] would update gift",
		"gift_id", giftID,
		"amount", amount,
		"type", gift.Type,
		"lookup_id", gift.LookupID)

	return nil
}

// nextFakeID generates a unique fake ID for dry-run operations.
func (d *dryRunClient) nextFakeID(prefix string) string {
	n := atomic.AddUint64(&d.counter, 1)
	return fmt.Sprintf("dry-run-%s-%d", prefix, n)
}
