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

## Donation

| FundraiseUp    | Blackbaud      | Notes                            |
|----------------|----------------|----------------------------------|
| Amount         | Gift Amount    |                                  |
| Date Created   | Gift Date      |                                  |
| Donation ID    | External ID    | Used to prevent duplicate syncs  |
| Comment        | Reference      | Donor's comment on the donation  |
| Payment Method | Payment Method | See payment method mapping below |

Gift Type and Fund ID are set from your configuration, not mapped from FundraiseUp.

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

## Recurring Donations

Recurring donations are linked together in Blackbaud so you can see the full series of payments.

| FundraiseUp        | Blackbaud    | Notes                                                   |
|--------------------|--------------|---------------------------------------------------------|
| Recurring Plan ID  | Lookup ID    | Identifies all payments in the same series              |
| Installment Number | —            | Used to determine payment sequence                      |
| —                  | Gift Subtype | Automatically set to "Recurring"                        |
| —                  | Linked Gifts | Each payment links back to the first gift in the series |

### How Recurring Linking Works

1. When the **first payment** in a recurring series is synced, a new gift is created with the Recurring Plan ID stored as the Lookup ID.

2. When **subsequent payments** are synced, they are linked to the first gift using Blackbaud's Linked Gifts feature. This allows you to see the complete donation history for a recurring donor.

## What's Not Mapped

The following FundraiseUp fields are not currently mapped to Blackbaud:

- Campaign
- Designation
- Currency (assumes single currency)
- Donation Status (only successful donations are synced)
