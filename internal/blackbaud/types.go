// Package blackbaud provides a client for the Blackbaud SKY API.
package blackbaud

import "encoding/json"

// GiftType represents the type of gift in Raiser's Edge NXT.
type GiftType string

const (
	// GiftTypeDonation is a one-off donation.
	GiftTypeDonation GiftType = "Donation"

	// GiftTypeRecurringGift is the first payment in a recurring series.
	GiftTypeRecurringGift GiftType = "RecurringGift"

	// GiftTypeRecurringGiftPayment is a subsequent payment in a recurring series.
	GiftTypeRecurringGiftPayment GiftType = "RecurringGiftPayment"
)

// GiftSubtype represents the subtype of gift in Raiser's Edge NXT.
type GiftSubtype string

const (
	// GiftSubtypeRecurring indicates a recurring gift.
	GiftSubtypeRecurring GiftSubtype = "Recurring"
)

// GiftOrigin contains source system information for a gift.
type GiftOrigin struct {
	// DonationID is the original donation identifier from the source system.
	DonationID string `json:"donation_id"`

	// Name is the source system name.
	Name string `json:"name"`
}

// String returns the JSON representation of the origin.
func (o GiftOrigin) String() string {
	b, _ := json.Marshal(o)
	return string(b)
}

// Address represents a constituent's address.
type Address struct {
	// AddressLines contains the street address.
	AddressLines string `json:"address_lines"`

	// City is the city name.
	City string `json:"city"`

	// Country is the country name or code.
	Country string `json:"country"`

	// PostCode is the postal or ZIP code.
	PostCode string `json:"post_code"`

	// Primary indicates if this is the primary address.
	Primary bool `json:"primary"`

	// State is the state or province.
	State string `json:"state"`

	// Type is the address type (e.g., Home, Work).
	Type string `json:"type"`
}

// Constituent represents a person or organisation in the Raiser's Edge NXT database (donor, prospect, etc.).
type Constituent struct {
	// Address is the constituent's address.
	Address *Address `json:"address,omitempty"`

	// Email is the constituent's email.
	Email *Email `json:"email,omitempty"`

	// FirstName is the constituent's first name.
	FirstName string `json:"first"`

	// ID is the unique constituent identifier.
	ID string `json:"id,omitempty"`

	// LastName is the constituent's last name.
	LastName string `json:"last"`

	// Phone is the constituent's phone number.
	Phone *Phone `json:"phone,omitempty"`

	// Type is the constituent type (e.g., Individual, Organization).
	Type string `json:"type"`
}

// Email represents a constituent's email.
type Email struct {
	// Address is the email address.
	Address string `json:"address"`

	// Primary indicates if this is the primary email.
	Primary bool `json:"primary"`

	// Type is the email type (e.g., Email, Work).
	Type string `json:"type"`
}

// Gift represents a gift in Raiser's Edge NXT.
type Gift struct {
	// Amount is the gift amount.
	Amount *GiftAmount `json:"amount"`

	// BatchNumber is the batch identifier.
	BatchNumber string `json:"batch_number,omitempty"`

	// BatchPrefix is the batch prefix for grouping gifts.
	BatchPrefix string `json:"batch_prefix,omitempty"`

	// ConstituentID links the gift to a constituent.
	ConstituentID string `json:"constituent_id"`

	// Date is the gift date in YYYY-MM-DD format.
	Date string `json:"date"`

	// GiftAidAmount is the UK Gift Aid amount.
	GiftAidAmount *GiftAmount `json:"gift_aid_amount,omitempty"`

	// GiftAidEligible indicates if eligible for UK Gift Aid.
	GiftAidEligible bool `json:"is_gift_aid_eligible,omitempty"`

	// GiftSplits defines how the gift is split across funds.
	GiftSplits []GiftSplit `json:"gift_splits,omitempty"`

	// GiftStatus is the gift status.
	GiftStatus string `json:"gift_status,omitempty"`

	// ID is the unique gift identifier.
	ID string `json:"id,omitempty"`

	// IsAnonymous indicates if the gift is anonymous.
	IsAnonymous bool `json:"is_anonymous,omitempty"`

	// IsManual indicates if the gift was entered manually.
	IsManual bool `json:"is_manual,omitempty"`

	// LinkedGifts contains IDs of related gifts.
	LinkedGifts []string `json:"linked_gifts,omitempty"`

	// LookupID is the user-defined lookup identifier.
	LookupID string `json:"lookup_id,omitempty"`

	// Origin contains source system information as JSON with name and donation_id fields.
	Origin string `json:"origin,omitempty"`

	// PaymentMethod is the payment method used.
	PaymentMethod string `json:"payment_method,omitempty"`

	// PostDate is the date the gift was posted.
	PostDate string `json:"post_date,omitempty"`

	// PostStatus is the posting status.
	PostStatus string `json:"post_status,omitempty"`

	// Receipts contains receipt information.
	Receipts []Receipt `json:"receipts,omitempty"`

	// Reference is a reference note or comment.
	Reference string `json:"reference,omitempty"`

	// SoftCredits contains soft credit attributions.
	SoftCredits []SoftCredit `json:"soft_credits,omitempty"`

	// Subtype is the gift subtype.
	Subtype GiftSubtype `json:"subtype,omitempty"`

	// Tribute contains tribute or memorial information.
	Tribute *Tribute `json:"tribute,omitempty"`

	// Type is the gift type (e.g., Donation, Pledge).
	Type GiftType `json:"type"`
}

// GiftAmount represents an amount with currency.
type GiftAmount struct {
	// Value is the monetary amount.
	Value float64 `json:"value"`
}

// GiftSplit represents how a gift is split across funds.
type GiftSplit struct {
	// Amount is the split amount.
	Amount *GiftAmount `json:"amount"`

	// AppealID links to a specific appeal.
	AppealID string `json:"appeal_id,omitempty"`

	// CampaignID links to a specific campaign.
	CampaignID string `json:"campaign_id,omitempty"`

	// FundID is the fund receiving this portion.
	FundID string `json:"fund_id"`
}

// Phone represents a constituent's phone number.
type Phone struct {
	// Number is the phone number.
	Number string `json:"number"`

	// Primary indicates if this is the primary phone.
	Primary bool `json:"primary"`

	// Type is the phone type (e.g., Mobile, Home).
	Type string `json:"type"`
}

// Receipt represents a gift receipt.
type Receipt struct {
	// Amount is the receipt amount.
	Amount string `json:"amount,omitempty"`

	// Date is the receipt date.
	Date string `json:"date,omitempty"`

	// Status is the receipt status.
	Status string `json:"status"`
}

// SoftCredit represents a soft credit on a gift.
type SoftCredit struct {
	// Amount is the soft credit amount.
	Amount *GiftAmount `json:"amount"`

	// ConstituentID is the constituent receiving the soft credit.
	ConstituentID string `json:"constituent_id"`
}

// Tribute represents a tribute/memorial for a gift.
type Tribute struct {
	// TributeID is the tribute identifier.
	TributeID string `json:"tribute_id"`
}

// constituentSearchResponse represents the constituent search API response.
type constituentSearchResponse struct {
	// Count is the total number of results.
	Count int `json:"count"`

	// Value contains the matching constituents.
	Value []Constituent `json:"value"`
}

// createResponse represents the response when creating a resource.
type createResponse struct {
	// ID is the identifier of the created resource.
	ID string `json:"id"`
}

// tokenResponse represents the OAuth token response from Blackbaud.
type tokenResponse struct {
	// AccessToken is the OAuth access token.
	AccessToken string `json:"access_token"`

	// ExpiresIn is the token lifetime in seconds.
	ExpiresIn int `json:"expires_in"`

	// RefreshToken is the token used to obtain new access tokens.
	RefreshToken string `json:"refresh_token"`

	// TokenType is the type of token (e.g., Bearer).
	TokenType string `json:"token_type"`
}
