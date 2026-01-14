package fundraiseup

import (
	"fmt"

	"github.com/peteski22/giftbridge/internal/blackbaud"
)

// Compile-time interface satisfaction checks.
// Note: Donation.ToDomainType takes a constituentID parameter, so it doesn't satisfy Convertible.
var (
	// Verify Address implements Convertible.
	_ Convertible[*blackbaud.Address] = (*Address)(nil)

	// Verify PaymentMethod implements Convertible.
	_ Convertible[string] = PaymentMethod("")

	// Verify Supporter implements Convertible.
	_ Convertible[*blackbaud.Constituent] = (*Supporter)(nil)
)

// Convertible defines types that can convert to a domain representation.
type Convertible[T any] interface {
	ToDomainType() T
}

// ToDomainType converts an Address to its Blackbaud domain representation.
func (a *Address) ToDomainType() *blackbaud.Address {
	if a == nil {
		return nil
	}

	lines := a.Line1
	if a.Line2 != "" {
		lines = fmt.Sprintf("%s\n%s", a.Line1, a.Line2)
	}

	return &blackbaud.Address{
		AddressLines: lines,
		City:         a.City,
		Country:      a.Country,
		PostCode:     a.PostalCode,
		Primary:      true,
		State:        a.State,
		Type:         "Home",
	}
}

// ToDomainType converts a Donation to its Blackbaud domain representation.
func (d Donation) ToDomainType(constituentID string) *blackbaud.Gift {
	// FundraiseUp amount is in cents, Blackbaud expects decimal.
	amount := float64(d.Amount) / 100

	gift := &blackbaud.Gift{
		Amount:        &blackbaud.GiftAmount{Value: amount},
		ConstituentID: constituentID,
		Date:          d.CreatedAt.Format("2006-01-02"),
		ExternalID:    d.ID,
		Type:          "Donation",
	}

	if d.PaymentMethod != "" {
		gift.PaymentMethod = d.PaymentMethod.ToDomainType()
	}

	if d.Comment != "" {
		gift.Reference = d.Comment
	}

	return gift
}

// ToDomainType converts a PaymentMethod to its Blackbaud payment method string.
func (pm PaymentMethod) ToDomainType() string {
	switch pm {
	case PaymentMethodCard, PaymentMethodApplePay, PaymentMethodGooglePay:
		return "Credit card"
	case PaymentMethodBankTransfer, PaymentMethodACH:
		return "Direct debit"
	case PaymentMethodPayPal:
		return "PayPal"
	default:
		return "Other"
	}
}

// ToDomainType converts a Supporter to its Blackbaud domain representation.
func (s *Supporter) ToDomainType() *blackbaud.Constituent {
	if s == nil {
		return nil
	}

	constituent := &blackbaud.Constituent{
		FirstName: s.FirstName,
		LastName:  s.LastName,
		Type:      "Individual",
	}

	if s.Email != "" {
		constituent.Email = &blackbaud.Email{
			Address: s.Email,
			Primary: true,
			Type:    "Email",
		}
	}

	if s.Phone != "" {
		constituent.Phone = &blackbaud.Phone{
			Number:  s.Phone,
			Primary: true,
			Type:    "Mobile",
		}
	}

	constituent.Address = s.Address.ToDomainType()

	return constituent
}
