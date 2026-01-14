package fundraiseup

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/peteski22/giftbridge/internal/blackbaud"
)

func TestAddress_ToDomainType(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		address *Address
		want    *blackbaud.Address
	}{
		"nil address": {
			address: nil,
			want:    nil,
		},
		"address with single line": {
			address: &Address{
				City:       "London",
				Country:    "UK",
				Line1:      "123 Main Street",
				PostalCode: "SW1A 1AA",
				State:      "England",
			},
			want: &blackbaud.Address{
				AddressLines: "123 Main Street",
				City:         "London",
				Country:      "UK",
				PostCode:     "SW1A 1AA",
				Primary:      true,
				State:        "England",
				Type:         "Home",
			},
		},
		"address with two lines": {
			address: &Address{
				City:       "New York",
				Country:    "USA",
				Line1:      "456 Park Ave",
				Line2:      "Suite 100",
				PostalCode: "10022",
				State:      "NY",
			},
			want: &blackbaud.Address{
				AddressLines: "456 Park Ave\nSuite 100",
				City:         "New York",
				Country:      "USA",
				PostCode:     "10022",
				Primary:      true,
				State:        "NY",
				Type:         "Home",
			},
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			got := tc.address.ToDomainType()

			require.Equal(t, tc.want, got)
		})
	}
}

func TestDonation_ToDomainType(t *testing.T) {
	t.Parallel()

	createdAt := time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC)

	tests := map[string]struct {
		constituentID string
		donation      Donation
		want          *blackbaud.Gift
	}{
		"basic donation": {
			donation: Donation{
				Amount:        5000,
				CreatedAt:     createdAt,
				ID:            "don_123",
				PaymentMethod: PaymentMethodCard,
			},
			constituentID: "const_456",
			want: &blackbaud.Gift{
				Amount:        &blackbaud.GiftAmount{Value: 50.00},
				ConstituentID: "const_456",
				Date:          "2024-01-15",
				ExternalID:    "don_123",
				PaymentMethod: "Credit card",
				Type:          "Donation",
			},
		},
		"donation with comment": {
			donation: Donation{
				Amount:        10000,
				Comment:       "In memory of John",
				CreatedAt:     createdAt,
				ID:            "don_789",
				PaymentMethod: PaymentMethodPayPal,
			},
			constituentID: "const_abc",
			want: &blackbaud.Gift{
				Amount:        &blackbaud.GiftAmount{Value: 100.00},
				ConstituentID: "const_abc",
				Date:          "2024-01-15",
				ExternalID:    "don_789",
				PaymentMethod: "PayPal",
				Reference:     "In memory of John",
				Type:          "Donation",
			},
		},
		"donation without payment method": {
			donation: Donation{
				Amount:    2500,
				CreatedAt: createdAt,
				ID:        "don_empty",
			},
			constituentID: "const_xyz",
			want: &blackbaud.Gift{
				Amount:        &blackbaud.GiftAmount{Value: 25.00},
				ConstituentID: "const_xyz",
				Date:          "2024-01-15",
				ExternalID:    "don_empty",
				Type:          "Donation",
			},
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			got := tc.donation.ToDomainType(tc.constituentID)

			require.Equal(t, tc.want, got)
		})
	}
}

func TestPaymentMethod_ToDomainType(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		pm   PaymentMethod
		want string
	}{
		"card": {
			pm:   PaymentMethodCard,
			want: "Credit card",
		},
		"apple pay": {
			pm:   PaymentMethodApplePay,
			want: "Credit card",
		},
		"google pay": {
			pm:   PaymentMethodGooglePay,
			want: "Credit card",
		},
		"bank transfer": {
			pm:   PaymentMethodBankTransfer,
			want: "Direct debit",
		},
		"ach": {
			pm:   PaymentMethodACH,
			want: "Direct debit",
		},
		"paypal": {
			pm:   PaymentMethodPayPal,
			want: "PayPal",
		},
		"unknown": {
			pm:   PaymentMethod("unknown"),
			want: "Other",
		},
		"empty": {
			pm:   PaymentMethod(""),
			want: "Other",
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			got := tc.pm.ToDomainType()

			require.Equal(t, tc.want, got)
		})
	}
}

func TestSupporter_ToDomainType(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		supporter *Supporter
		want      *blackbaud.Constituent
	}{
		"nil supporter": {
			supporter: nil,
			want:      nil,
		},
		"minimal supporter": {
			supporter: &Supporter{
				FirstName: "Jane",
				LastName:  "Doe",
			},
			want: &blackbaud.Constituent{
				FirstName: "Jane",
				LastName:  "Doe",
				Type:      "Individual",
			},
		},
		"supporter with email": {
			supporter: &Supporter{
				Email:     "jane@example.com",
				FirstName: "Jane",
				LastName:  "Doe",
			},
			want: &blackbaud.Constituent{
				Email: &blackbaud.Email{
					Address: "jane@example.com",
					Primary: true,
					Type:    "Email",
				},
				FirstName: "Jane",
				LastName:  "Doe",
				Type:      "Individual",
			},
		},
		"supporter with phone": {
			supporter: &Supporter{
				FirstName: "Jane",
				LastName:  "Doe",
				Phone:     "+1234567890",
			},
			want: &blackbaud.Constituent{
				FirstName: "Jane",
				LastName:  "Doe",
				Phone: &blackbaud.Phone{
					Number:  "+1234567890",
					Primary: true,
					Type:    "Mobile",
				},
				Type: "Individual",
			},
		},
		"supporter with address": {
			supporter: &Supporter{
				Address: &Address{
					City:       "London",
					Country:    "UK",
					Line1:      "123 Main St",
					PostalCode: "SW1A 1AA",
				},
				FirstName: "Jane",
				LastName:  "Doe",
			},
			want: &blackbaud.Constituent{
				Address: &blackbaud.Address{
					AddressLines: "123 Main St",
					City:         "London",
					Country:      "UK",
					PostCode:     "SW1A 1AA",
					Primary:      true,
					Type:         "Home",
				},
				FirstName: "Jane",
				LastName:  "Doe",
				Type:      "Individual",
			},
		},
		"full supporter": {
			supporter: &Supporter{
				Address: &Address{
					City:       "New York",
					Country:    "USA",
					Line1:      "456 Park Ave",
					Line2:      "Apt 5",
					PostalCode: "10022",
					State:      "NY",
				},
				Email:     "john@example.com",
				FirstName: "John",
				ID:        "sup_123",
				LastName:  "Smith",
				Phone:     "+1987654321",
			},
			want: &blackbaud.Constituent{
				Address: &blackbaud.Address{
					AddressLines: "456 Park Ave\nApt 5",
					City:         "New York",
					Country:      "USA",
					PostCode:     "10022",
					Primary:      true,
					State:        "NY",
					Type:         "Home",
				},
				Email: &blackbaud.Email{
					Address: "john@example.com",
					Primary: true,
					Type:    "Email",
				},
				FirstName: "John",
				LastName:  "Smith",
				Phone: &blackbaud.Phone{
					Number:  "+1987654321",
					Primary: true,
					Type:    "Mobile",
				},
				Type: "Individual",
			},
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			got := tc.supporter.ToDomainType()

			require.Equal(t, tc.want, got)
		})
	}
}
