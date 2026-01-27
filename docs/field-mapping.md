# FundraiseUp → Blackbaud Field Mapping

This document describes how data is mapped from FundraiseUp to Blackbaud Raiser's Edge NXT when syncing donations.

## Donor Information

| FundraiseUp | Blackbaud     | Notes                                   |
|-------------|---------------|-----------------------------------------|
| First Name  | First Name    |                                         |
| Last Name   | Last Name     |                                         |
| Email       | Email Address | Marked as primary                       |
| Phone       | Phone Number  | Marked as primary, type set to "Mobile" |

## Address

| FundraiseUp    | Blackbaud     | Notes                                |
|----------------|---------------|--------------------------------------|
| Address Line 1 | Address Lines | Combined with Line 2 if present      |
| Address Line 2 |               | Appended to Line 1 with a line break |
| City           | City          |                                      |
| Region         | State         |                                      |
| Postal Code    | Post Code     |                                      |
| Country        | Country       |                                      |

Address is marked as primary with type "Home".

## Donation (One-off)

| FundraiseUp    | Blackbaud      | Notes                                          |
|----------------|----------------|------------------------------------------------|
| Amount         | Gift Amount    |                                                |
| Date Created   | Gift Date      |                                                |
| Donation ID    | Lookup ID      | User-defined identifier for deduplication     |
| Comment        | Reference      | Donor's comment on the donation                |
| Payment Method | Payment Method | See payment method mapping below               |
| —              | Batch Prefix   | Always "FundraiseUp"                           |
| —              | Is Manual      | Always true                                    |
| —              | Type           | "Donation" (from configuration)               |

## Recurring Donations

Recurring donations use different Blackbaud gift types to properly track the series.

| FundraiseUp        | Blackbaud    | Notes                                                   |
|--------------------|--------------|---------------------------------------------------------|
| Amount             | Gift Amount  |                                                         |
| Date Created       | Gift Date    |                                                         |
| Recurring Plan ID  | Lookup ID    | Groups all payments in the same series                  |
| Donation ID        | Origin       | JSON: `{"donation_id":"...","name":"FundraiseUp"}`      |
| Comment            | Reference    | Donor's comment                                         |
| Payment Method     | Payment Method | See payment method mapping below                      |
| —                  | Batch Prefix | Always "FundraiseUp"                                    |
| —                  | Is Manual    | Always true                                             |
| —                  | Type         | "RecurringGift" (first) or "RecurringGiftPayment" (subsequent) |
| —                  | Subtype      | Always "Recurring"                                      |
| —                  | Linked Gifts | Points to first gift (subsequent payments only)         |

### Gift Type Logic

| Donation Type       | Blackbaud Type         | Linked Gifts        |
|---------------------|------------------------|---------------------|
| One-off             | Donation               | —                   |
| First recurring     | RecurringGift          | —                   |
| Subsequent recurring | RecurringGiftPayment  | First gift ID       |

### How Recurring Linking Works

1. When the **first payment** in a recurring series is synced, a new gift is created with:
   - Type: `RecurringGift`
   - Lookup ID: The Recurring Plan ID
   - Origin: JSON containing the donation ID

2. When **subsequent payments** are synced:
   - Type: `RecurringGiftPayment`
   - Linked to the first gift using Blackbaud's Linked Gifts feature
   - Same Lookup ID as the first payment (Recurring Plan ID)

This allows you to see the complete donation history for a recurring donor.

## Payment Methods

| FundraiseUp          | Blackbaud    |
|----------------------|--------------|
| Credit Card          | Credit card  |
| Apple Pay            | Credit card  |
| Google Pay           | Credit card  |
| Bank Transfer (BACS) | Direct debit |
| ACH                  | Direct debit |
| SEPA Direct Debit    | Direct debit |
| PayPal               | PayPal       |
| Other / Unknown      | Other        |

## What's Not Mapped

The following FundraiseUp fields are not currently mapped to Blackbaud:

- Campaign
- Designation
- Currency (assumes single currency)
- Donation Status (only successful donations are synced)
