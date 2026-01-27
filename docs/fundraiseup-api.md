# FundraiseUp API

GiftBridge uses the FundraiseUp API to fetch donation data. This document covers API setup and key information.

**Reference:** [FundraiseUp API Documentation](https://api.fundraiseup.com/v1/docs/)

## Overview

The FundraiseUp API is a REST API that provides access to:
- Donations
- Recurring plans
- Supporters
- Events

GiftBridge uses the donations endpoint to fetch new donations for syncing to Raiser's Edge.

### Key Features

| Feature | Description |
|---------|-------------|
| Architecture | REST principles |
| Request format | JSON-encoded bodies with `Content-Type: application/json` |
| Response format | JSON-encoded |
| Authentication | API key in header |

## Getting an API Key

Only users with the **Organization Administrator** role can create API keys.

### Steps

1. Go to **Dashboard** → **Settings** → **API keys**
2. Click **Create API key**
3. Enter a descriptive name (e.g., "GiftBridge Integration")
4. Choose the operating mode:
   - **Test data** - For testing (keys prefixed with `test_`)
   - **Live data** - For production (no prefix)
5. Set permissions - GiftBridge needs:
   - **Retrieve donation data** ✓
6. Click **Create API key**
7. **Copy and securely store the key** - you won't see it again

The key is activated immediately and does not expire.

## API Key Modes

| Mode | Key prefix | Use case |
|------|-----------|----------|
| Test | `test_` | Testing integrations without affecting live data |
| Live | (none) | Production use with real donations |

You can identify which mode a key uses by checking for the `test_` prefix.

## Rate Limits

FundraiseUp uses concurrency-based rate limiting.

### Concurrency Limit

- Maximum **3 parallel requests** per account
- Applies across all API keys (test and live) for the same account
- Exceeding the limit returns `429 Too Many Requests` with code `concurrent_requests_limit_exceeded`

### Best Practices

GiftBridge processes requests sequentially to avoid hitting limits:
- Donations are fetched in pages
- Each page request completes before the next starts
- No parallel API calls

If you're building additional integrations, keep this shared limit in mind.

## Pagination

The API uses cursor-based pagination for list endpoints.

```
GET /donations?starting_after=don_xxxxx&limit=100
```

| Parameter | Description |
|-----------|-------------|
| `limit` | Number of results per page (max 100) |
| `starting_after` | Cursor for next page (donation ID) |
| `ending_before` | Cursor for previous page |

GiftBridge handles pagination automatically when fetching donations.

## Error Handling

### HTTP Status Codes

| Code | Meaning |
|------|---------|
| 200 | Success |
| 400 | Bad request (invalid parameters) |
| 401 | Unauthorized (invalid API key) |
| 403 | Forbidden (insufficient permissions) |
| 404 | Not found |
| 429 | Too many requests (rate limited) |
| 500 | Server error |

### Error Response Format

```json
{
  "error": {
    "code": "error_code",
    "message": "Human readable message"
  }
}
```

## Data Retrieved by GiftBridge

GiftBridge fetches donations with these key fields:

| Field | Description |
|-------|-------------|
| `id` | Unique donation identifier |
| `amount` | Donation amount |
| `currency` | Currency code |
| `created_at` | Timestamp |
| `supporter` | Donor information (name, email, address) |
| `recurring_id` | Link to recurring plan (if applicable) |
| `designation` | Fund/campaign designation |

For detailed field mapping to Raiser's Edge, see [Field Mapping](field-mapping.md).

## Testing

Use a test mode API key (`test_` prefix) to:
- Verify the integration works
- Test with sample data
- Avoid affecting live donations

Test mode uses separate data from live mode - no real donations or banking activity.

## References

- [FundraiseUp API Documentation](https://api.fundraiseup.com/v1/docs/)
- [FundraiseUp Dashboard](https://dashboard.fundraiseup.com/)
