# Authentication Setup

GiftBridge uses the Blackbaud SKY API, which requires OAuth 2.0 authorization. This document explains the authentication model, setup process, and how credentials are managed.

## Why OAuth? (Not Just API Keys)

Unlike simpler APIs that use static API keys, Blackbaud SKY API requires OAuth 2.0 Authorization Code Flow. This is because:

1. **User-delegated access** - The API accesses data belonging to a specific Blackbaud organization
2. **Consent required** - A real user (admin) must explicitly authorize your application
3. **Scoped permissions** - The app can only access data the authorizing user can access

The refresh token represents an ongoing authorization grant from a user to your application. It's not tied to a specific machine or IP address.

**Reference:** [Blackbaud SKY API Security](https://developer.blackbaud.com/skyapi/docs/security)

## Credentials Overview

You'll need these credentials from Blackbaud:

| Credential       | What it is                                      | Where to get it                               |
|------------------|-------------------------------------------------|-----------------------------------------------|
| Client ID        | Identifies your application (OAuth `client_id`) | Blackbaud Developer Portal → My Applications  |
| Client Secret    | Application password (OAuth `client_secret`)    | Blackbaud Developer Portal → My Applications  |
| Subscription Key | API access key for rate limiting/tracking       | Blackbaud Developer Portal → My Subscriptions |
| Refresh Token    | Authorization grant from a user                 | OAuth flow (one-time per environment)         |

You'll also need:

| Credential          | What it is                          | Where to get it                              |
|---------------------|-------------------------------------|----------------------------------------------|
| FundraiseUp API Key | Access to FundraiseUp donations API | FundraiseUp Dashboard → Settings → API keys  |

See [FundraiseUp API](fundraiseup-api.md) for detailed setup instructions.

**Reference:** [Blackbaud SKY API Applications](https://developer.blackbaud.com/skyapi/docs/applications)

## Initial Setup (One-Time)

### 1. Register a Blackbaud Developer Account

Go to [developer.blackbaud.com](https://developer.blackbaud.com) and create an account.

### 2. Create an Application

1. Go to **My Applications** → **Add**
2. Fill in application details
3. Note your **Application ID** (Client ID)
4. Note your **Application Secret** (Client Secret) - treat this like a password

### 3. Add Redirect URIs

For the OAuth flow, Blackbaud needs to know where to redirect after authorization.

1. In your application settings, find **Redirect URIs**
2. Add `http://localhost:8080/callback` (for local auth flow)
3. Add any production callback URLs if needed

### 4. Get a Subscription Key

1. Go to **My Subscriptions**
2. Subscribe to the APIs you need (Constituent, Gift)
3. Note your **Primary Access Key** (Subscription Key)

### 5. Connect to Your Blackbaud Environment

An administrator in your Raiser's Edge NXT organization must activate your application:

1. In Raiser's Edge NXT, go to **Control Panel** → **Applications**
2. Search for your application by name or Application ID
3. Click **Connect** to authorize it

This grants your application permission to access that organization's data.

**Reference:** [Blackbaud SKY API Authorization](https://developer.blackbaud.com/skyapi/docs/authorization)

### 6. Complete the OAuth Flow

Use the `giftbridge auth` command to complete the OAuth flow:

```bash
# First, create and edit your local config
giftbridge init
# Edit ~/.giftbridge/config.yaml with your credentials

# Then run the auth command
giftbridge auth
```

This will:
1. Start a local web server on `http://localhost:8080`
2. Open your browser to the Blackbaud authorization page
3. After you authorize, capture the callback
4. Exchange the code for tokens
5. Save the refresh token to `~/.giftbridge/token`

#### Manual OAuth Flow (Alternative)

If you prefer to do it manually:

1. Direct a user to the authorization URL:
   ```
   https://app.blackbaud.com/oauth/authorize?
     client_id=YOUR_CLIENT_ID&
     response_type=code&
     redirect_uri=http://localhost:8080/callback
   ```

2. User logs in and clicks "Authorize"

3. Blackbaud redirects to your callback URL with an authorization code:
   ```
   http://localhost:8080/callback?code=AUTHORIZATION_CODE
   ```

4. Exchange the code for tokens:
   ```bash
   curl -X POST https://oauth2.sky.blackbaud.com/token \
     -d "grant_type=authorization_code" \
     -d "code=AUTHORIZATION_CODE" \
     -d "client_id=YOUR_CLIENT_ID" \
     -d "client_secret=YOUR_CLIENT_SECRET" \
     -d "redirect_uri=http://localhost:8080/callback"
   ```

5. Save the `refresh_token` from the response to `~/.giftbridge/token`

## Token Lifecycle

Once you have a refresh token:

1. GiftBridge uses it to get short-lived access tokens (valid ~60 minutes)
2. Each token refresh may return a new refresh token
3. GiftBridge automatically saves the new refresh token
4. The cycle continues indefinitely

You should never need to repeat the OAuth flow unless:
- The refresh token is revoked (user disconnects your app)
- The refresh token expires due to inactivity (rare)
- You're setting up a new environment

## Running Modes

### Production (AWS Lambda)

Credentials are stored in AWS:
- Refresh token → AWS Secrets Manager (auto-rotates)
- Client ID/Secret → Environment variables (from CloudFormation parameters)
- Subscription Key → Environment variable
- Sync state → SSM Parameter Store
- Donation deduplication → Blackbaud lookup ID (no external state needed)

### Local Development

For local testing with `--dry-run`, credentials are stored locally:
```
~/.giftbridge/
  config.yaml      # API keys, client ID/secret, subscription key
  token            # Refresh token (auto-updated)
```

Setup:
```bash
giftbridge init    # Create config file
# Edit ~/.giftbridge/config.yaml
giftbridge auth    # Complete OAuth flow
giftbridge --dry-run --since=2024-01-01T00:00:00Z
```

No AWS infrastructure required for dry-run mode.

## Credential Rotation

Blackbaud recommends rotating credentials every 90 days:

- **Application Secret**: Regenerate in Developer Portal, update in your config
- **Subscription Key**: Regenerate in Developer Portal, update in your config

Blackbaud provides primary and secondary secrets/keys to allow rotation without downtime.

**Reference:** [Blackbaud SKY API Security - Credential Rotation](https://developer.blackbaud.com/skyapi/docs/security#credential-rotation)

## Troubleshooting

### "Application not connected"

An admin needs to activate your application in Raiser's Edge NXT.

### "Invalid refresh token"

The refresh token may have been revoked or expired. Complete the OAuth flow again.

### "Insufficient permissions"

The user who authorized the app doesn't have permission to access the requested data. Have an admin with full access complete the OAuth flow.

## References

- [Blackbaud SKY API Security](https://developer.blackbaud.com/skyapi/docs/security)
- [Blackbaud SKY API Applications](https://developer.blackbaud.com/skyapi/docs/applications)
- [Blackbaud SKY API Authorization](https://developer.blackbaud.com/skyapi/docs/authorization)
- [OAuth 2.0 Authorization Code Flow](https://oauth.net/2/grant-types/authorization-code/)
