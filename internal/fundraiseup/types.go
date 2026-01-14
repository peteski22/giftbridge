// Package fundraiseup provides a client for the FundraiseUp API.
package fundraiseup

import "time"

const (
	// PaymentMethodACH represents an ACH bank transfer payment.
	PaymentMethodACH PaymentMethod = "ach"

	// PaymentMethodApplePay represents an Apple Pay payment.
	PaymentMethodApplePay PaymentMethod = "apple_pay"

	// PaymentMethodBankTransfer represents a bank transfer payment.
	PaymentMethodBankTransfer PaymentMethod = "bank_transfer"

	// PaymentMethodCard represents a credit or debit card payment.
	PaymentMethodCard PaymentMethod = "card"

	// PaymentMethodGooglePay represents a Google Pay payment.
	PaymentMethodGooglePay PaymentMethod = "google_pay"

	// PaymentMethodPayPal represents a PayPal payment.
	PaymentMethodPayPal PaymentMethod = "paypal"
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
	PostalCode string `json:"postalCode"`

	// State is the state or province.
	State string `json:"state"`
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
	// Amount is the donation amount in cents.
	Amount int `json:"amount"`

	// Campaign is the associated campaign.
	Campaign *Campaign `json:"campaign"`

	// Comment is the donor's comment.
	Comment string `json:"comment"`

	// CreatedAt is the donation creation timestamp.
	CreatedAt time.Time `json:"createdAt"`

	// Currency is the three-letter currency code.
	Currency string `json:"currency"`

	// Designation is the fund designation.
	Designation string `json:"designation"`

	// ID is the unique donation identifier.
	ID string `json:"id"`

	// IsRecurring indicates if this is a recurring donation.
	IsRecurring bool `json:"isRecurring"`

	// PaymentMethod is the method used for payment.
	PaymentMethod PaymentMethod `json:"paymentMethod"`

	// RecurringID links recurring donations together.
	RecurringID string `json:"recurringId"`

	// Status is the donation status.
	Status string `json:"status"`

	// Supporter is the person who made the donation.
	Supporter *Supporter `json:"supporter"`

	// UpdatedAt is the last update timestamp.
	UpdatedAt time.Time `json:"updatedAt"`
}

// PaymentMethod represents a FundraiseUp payment method.
type PaymentMethod string

// Supporter represents a person who has donated via FundraiseUp.
type Supporter struct {
	// Address is the supporter's address.
	Address *Address `json:"address"`

	// Email is the supporter's email address.
	Email string `json:"email"`

	// FirstName is the supporter's first name.
	FirstName string `json:"firstName"`

	// ID is the unique supporter identifier.
	ID string `json:"id"`

	// LastName is the supporter's last name.
	LastName string `json:"lastName"`

	// Phone is the supporter's phone number.
	Phone string `json:"phone"`
}

// donationsResponse represents the API response for listing donations.
type donationsResponse struct {
	// Data contains the list of donations.
	Data []Donation `json:"data"`

	// HasMore indicates if there are more results.
	HasMore bool `json:"hasMore"`

	// NextCursor is the pagination cursor for the next page.
	NextCursor string `json:"nextCursor"`
}
