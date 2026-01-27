package fundraiseup

import (
	"fmt"
	"strconv"

	"github.com/peteski22/giftbridge/internal/blackbaud"
)

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
		State:        a.Region,
		Type:         "Home",
	}
}

// ToDomainType converts a Donation to its Blackbaud domain representation.
// Returns a gift with donation-specific fields only. The caller is responsible
// for setting ConstituentID, Type, and GiftSplits based on configuration.
func (d *Donation) ToDomainType() (*blackbaud.Gift, error) {
	if d == nil {
		return nil, nil
	}

	// FundraiseUp amount is a decimal string, Blackbaud expects float.
	amount, err := strconv.ParseFloat(d.Amount, 64)
	if err != nil {
		return nil, fmt.Errorf("parsing donation amount %s: %w", d.Amount, err)
	}

	gift := &blackbaud.Gift{
		Amount: &blackbaud.GiftAmount{Value: amount},
		Date:   d.CreatedAt.Format("2006-01-02"),
	}

	if d.Payment != nil && d.Payment.Method != "" {
		gift.PaymentMethod = d.Payment.Method.ToDomainType()
	}

	if d.Comment != "" {
		gift.Reference = d.Comment
	}

	return gift, nil
}

// ToDomainType converts a PaymentMethod to its Blackbaud payment method string.
func (pm PaymentMethod) ToDomainType() string {
	switch pm {
	case PaymentMethodCard, PaymentMethodApplePay, PaymentMethodGooglePay:
		return "Credit card"
	case PaymentMethodBankTransfer, PaymentMethodACH, PaymentMethodSEPA:
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

// InstallmentNumber returns the installment number for recurring donations.
// Returns 0 if not set or not parseable.
func (d *Donation) InstallmentNumber() int {
	if d == nil || d.Installment == "" {
		return 0
	}
	n, _ := strconv.Atoi(d.Installment)
	return n
}

// IsRecurring returns true if the donation is part of a recurring plan.
func (d *Donation) IsRecurring() bool {
	return d != nil && d.RecurringPlan != nil
}

// RecurringID returns the recurring plan ID, or empty string if not recurring.
func (d *Donation) RecurringID() string {
	if d == nil || d.RecurringPlan == nil {
		return ""
	}
	return d.RecurringPlan.ID
}
