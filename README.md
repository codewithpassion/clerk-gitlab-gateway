# Clerk-to-GitLab OAuth Gateway

A lightweight Go service that translates Clerk's OIDC into GitLab-compatible OAuth endpoints, enabling Mattermost Team Edition to authenticate users via Clerk.

## Why This Exists

Mattermost Team Edition only supports GitLab as an OAuth provider -- it doesn't support generic OIDC. To authenticate users via Clerk, we need a gateway that exposes three GitLab-compatible endpoints and proxies them to Clerk's OIDC endpoints.

## Architecture

```
Browser → Mattermost → Gateway → Clerk
                ↕                    ↕
         (GitLab OAuth)       (OIDC provider)
```

### Auth Flow

1. User clicks "Login with Claude Community Australia" on Mattermost
2. Mattermost redirects to gateway `GET /oauth/authorize`
3. Gateway redirects to Clerk `GET /oauth/authorize` (adding `scope=openid profile email`)
4. User authenticates on Clerk
5. Clerk redirects back to Mattermost with an authorization `code`
6. Mattermost calls gateway `POST /oauth/token` with the code
7. Gateway proxies the token exchange to Clerk, returns access token
8. Mattermost calls gateway `GET /api/v4/user` with the access token
9. Gateway calls Clerk's `/oauth/userinfo`, transforms the response into GitLab's user JSON format
10. Mattermost creates/logs in the user

### Endpoints

| Gateway Endpoint     | Purpose                              | Upstream (Clerk)         |
|----------------------|--------------------------------------|--------------------------|
| `GET /oauth/authorize` | Redirect user to Clerk login        | `{clerk}/oauth/authorize` |
| `POST /oauth/token`   | Exchange auth code for access token  | `{clerk}/oauth/token`     |
| `GET /api/v4/user`    | Return user in GitLab JSON format    | `{clerk}/oauth/userinfo`  |
| `GET /health`         | Health check                         | —                         |

### User ID Mapping

Mattermost requires a numeric `id` field (GitLab format). Clerk uses string IDs (e.g. `user_2abc123`). The gateway converts Clerk IDs to deterministic numeric IDs using FNV-1a hash, masked to 53 bits for JSON safety. No database is needed -- the same Clerk ID always produces the same numeric ID.

## Configuration

The gateway uses a single environment variable:

| Variable          | Required | Description                                      |
|-------------------|----------|--------------------------------------------------|
| `CLERK_ISSUER_URL` | Yes      | Clerk's OIDC issuer URL (e.g. `https://clerk.example.com`) |
| `PORT`            | No       | HTTP listen port (default: `8080`)               |

The Client ID and Secret are **not** configured on the gateway -- they are configured on the Mattermost side. The gateway simply proxies requests.

## Deployment

### Clerk OAuth Application Setup

1. In Clerk Dashboard → **OAuth Applications** → Create new
2. Enable scopes: **`openid`**, **`profile`**, **`email`**
3. Set redirect URIs:
   - `https://<mattermost-domain>/signup/gitlab/complete`
   - `https://<mattermost-domain>/login/gitlab/complete`
4. Copy the **Client ID** and **Client Secret**

### Deploy the Gateway

Build and run with Docker:

```bash
docker build -t clerk-gitlab-gateway .
docker run -p 8080:8080 -e CLERK_ISSUER_URL=https://clerk.example.com clerk-gitlab-gateway
```

The image is ~15MB (Alpine-based, statically compiled Go binary).

### Mattermost Configuration

Set these environment variables on Mattermost:

```
MM_GITLABSETTINGS_ENABLE=true
MM_GITLABSETTINGS_ID=<clerk-client-id>
MM_GITLABSETTINGS_SECRET=<clerk-client-secret>
MM_GITLABSETTINGS_AUTHENDPOINT=https://<gateway-domain>/oauth/authorize
MM_GITLABSETTINGS_TOKENENDPOINT=https://<gateway-domain>/oauth/token
MM_GITLABSETTINGS_USERAPIENDPOINT=https://<gateway-domain>/api/v4/user
MM_GITLABSETTINGS_BUTTONTEXT=Login with Claude Community Australia
MM_GITLABSETTINGS_BUTTONCOLOR=#6C47FF
```

Then restart Mattermost.

### Custom Login Button Icon (Optional)

Mattermost hardcodes the GitLab icon on the login button. To replace it with a custom icon, a small Mattermost webapp plugin is used.

The plugin source is in `../mm-custom-login-plugin/`. It injects CSS that hides the GitLab SVG and replaces it with a "CC" (Claude Community) icon.

To install:

1. Enable plugins on Mattermost:
   ```
   MM_PLUGINSETTINGS_ENABLE=true
   MM_PLUGINSETTINGS_ENABLEUPLOADS=true
   ```
2. Build the plugin:
   ```bash
   cd mm-custom-login-plugin/webapp && npm install && npm run build && cd ..
   tar czf mm-custom-login-plugin.tar.gz plugin.json webapp/dist/main.js
   ```
3. Upload via **System Console** → **Plugin Management** → Upload Plugin
4. Enable the plugin

## Current Deployment

- **Gateway**: `https://auth-gateway.rockyshoreslabs.io` (Coolify app `igoosowoc8s40wkwk8gs48so`)
- **Mattermost**: `https://chat.claudecommunity.com.au` (Coolify app `i4k40kwkgw480wc0koo488wg`)
- **Clerk issuer**: `https://clerk.claudecommunity.com.au`
- **Server**: ccommunity (`n44808040g0so4s4088og8g0`)

## Development

```bash
# Run tests (requires Go 1.23+ or Docker)
go test -v ./...

# Or via Docker
docker run --rm -v $(pwd):/app -w /app golang:1.23-alpine go test -v ./...
```

Zero external dependencies -- stdlib only.

## File Structure

```
main.go           # Config loading, HTTP server, route registration
gateway.go        # 3 endpoint handlers, logging middleware, types
idmap.go          # ClerkIDToNumeric() - FNV-1a hash, 53-bit mask
idmap_test.go     # Determinism, uniqueness, range, non-zero tests
gateway_test.go   # Handler tests with mock Clerk responses
Dockerfile        # Multi-stage: golang:1.23-alpine → alpine:3.21
go.mod            # Zero external dependencies (stdlib only)
```

## Troubleshooting

| Symptom | Cause | Fix |
|---------|-------|-----|
| `oauth_missing_code` on Mattermost | Clerk rejecting the authorize request | Check Clerk OAuth App scopes (`openid`, `profile`, `email`) and redirect URIs |
| `invalid_scope` error from Clerk | OAuth App doesn't have required scopes | Enable `openid`, `profile`, `email` in Clerk Dashboard |
| No requests in gateway logs | Domain not configured on gateway | Set FQDN in Coolify UI |
| Token exchange fails | Client ID/Secret mismatch | Verify `MM_GITLABSETTINGS_ID` and `SECRET` match Clerk OAuth App |
