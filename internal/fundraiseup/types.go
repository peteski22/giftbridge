// Package fundraiseup provides a client for the FundraiseUp API.
package fundraiseup

import "time"

const (
	// PaymentMethodACH represents an ACH bank transfer payment.
	PaymentMethodACH PaymentMethod = "ach"

	// PaymentMethodApplePay represents an Apple Pay payment.
	PaymentMethodApplePay PaymentMethod = "apple_pay"

	// PaymentMethodBankTransfer represents a bank transfer payment.
	PaymentMethodBankTransfer PaymentMethod = "bacs_direct_debit"

	// PaymentMethodCard represents a credit or debit card payment.
	PaymentMethodCard PaymentMethod = "credit_card"

	// PaymentMethodGooglePay represents a Google Pay payment.
	PaymentMethodGooglePay PaymentMethod = "google_pay"

	// PaymentMethodPayPal represents a PayPal payment.
	PaymentMethodPayPal PaymentMethod = "paypal"

	// PaymentMethodSEPA represents a SEPA direct debit payment.
	PaymentMethodSEPA PaymentMethod = "sepa_direct_debit"
)

// Address represents a supporter's address.
type Address struct {
	// City is the city name.
	City string `json:"city"`

	// Country is the country name or code.
	Country string `json:"country"`

	// Line1 is the first line of the street address.
	Line1 string `json:"line1"`

	// Line2 is the second line of the street address.
	Line2 string `json:"line2"`

	// PostalCode is the postal or ZIP code.
	PostalCode string `json:"postal_code"`

	// Region is the state or province.
	Region string `json:"region"`
}

// Campaign represents a FundraiseUp campaign.
type Campaign struct {
	// ID is the unique campaign identifier.
	ID string `json:"id"`

	// Name is the campaign name.
	Name string `json:"name"`
}

// Donation represents a donation from FundraiseUp.
type Donation struct {
	// Amount is the donation amount as a decimal string.
	Amount string `json:"amount"`

	// Campaign is the associated campaign.
	Campaign *Campaign `json:"campaign"`

	// Comment is the donor's comment.
	Comment string `json:"comment"`

	// CreatedAt is the donation creation timestamp.
	CreatedAt time.Time `json:"created_at"`

	// Currency is the three-letter currency code.
	Currency string `json:"currency"`

	// Designation is the fund designation.
	Designation *Designation `json:"designation"`

	// ID is the unique donation identifier.
	ID string `json:"id"`

	// Installment is the installment number for recurring donations (e.g., "1", "2").
	Installment string `json:"installment"`

	// Payment contains payment details.
	Payment *Payment `json:"payment"`

	// RecurringPlan contains recurring plan details, nil for one-off donations.
	RecurringPlan *RecurringPlan `json:"recurring_plan"`

	// Status is the donation status.
	Status string `json:"status"`

	// Supporter is the person who made the donation.
	Supporter *Supporter `json:"supporter"`
}

// Designation represents a fund designation.
type Designation struct {
	// ID is the unique designation identifier.
	ID string `json:"id"`

	// Name is the designation name.
	Name string `json:"name"`
}

// Payment contains payment details for a donation.
type Payment struct {
	// Method is the payment method used.
	Method PaymentMethod `json:"method"`
}

// PaymentMethod represents a FundraiseUp payment method.
type PaymentMethod string

// RecurringPlan represents a recurring donation plan.
type RecurringPlan struct {
	// CreatedAt is when the recurring plan was created.
	CreatedAt time.Time `json:"created_at"`

	// EndedAt is when the recurring plan ended, nil if active.
	EndedAt *time.Time `json:"ended_at"`

	// Frequency is the donation frequency (e.g., "monthly", "annual").
	Frequency string `json:"frequency"`

	// ID is the unique recurring plan identifier.
	ID string `json:"id"`

	// NextInstallmentAt is the next scheduled donation date.
	NextInstallmentAt *time.Time `json:"next_installment_at"`

	// Status is the recurring plan status (e.g., "active", "canceled").
	Status string `json:"status"`
}

// Supporter represents a person who has donated via FundraiseUp.
type Supporter struct {
	// Address is the supporter's address.
	Address *Address `json:"address"`

	// Email is the supporter's email address.
	Email string `json:"email"`

	// FirstName is the supporter's first name.
	FirstName string `json:"first_name"`

	// ID is the unique supporter identifier.
	ID string `json:"id"`

	// LastName is the supporter's last name.
	LastName string `json:"last_name"`

	// Phone is the supporter's phone number.
	Phone string `json:"phone"`
}

// donationsResponse represents the API response for listing donations.
type donationsResponse struct {
	// Data contains the list of donations.
	Data []Donation `json:"data"`

	// HasMore indicates if there are more results.
	HasMore bool `json:"has_more"`
}
