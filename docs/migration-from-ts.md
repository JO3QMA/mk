# Misskey-TS to Misskey-Go Migration Guide

This guide explains how to set up Misskey-Go as a replacement backend for an existing Misskey-TS instance, sharing the same database, Redis, and frontend assets.

## Prerequisites

- Go 1.24+
- PostgreSQL 15+ (existing Misskey-TS database)
- Redis 7+
- Misskey-TS source code (for frontend assets)
- git

## 1. Clone and Build

```bash
git clone https://github.com/shiroha-a/mk.git misskey-go
cd misskey-go
go build -o built/misskey ./cmd/misskey
```

## 2. Build Frontend Assets

Misskey-Go uses the same frontend as Misskey-TS. You need the built frontend assets.

```bash
# Clone Misskey-TS (if not already present)
git clone https://github.com/misskey-dev/misskey.git ../misskey
cd ../misskey

# Install dependencies and build
pnpm install
pnpm build

# The build outputs are in:
#   built/_frontend_vite_/  (JS/CSS bundles)
#   built/_frontend_dist_/  (locales, fonts)
#   packages/frontend/assets/  (game images, icons)
```

## 3. Configuration

### 3.1 Application Config

Create `.config/default.yml`:

```yaml
url: https://your-instance.example.com
port: 3000

db:
  host: localhost
  port: 5432
  db: misskey        # Your existing Misskey-TS database
  user: misskey
  pass: your_password

redis:
  host: localhost
  port: 6379

id: aidx             # Must match your Misskey-TS id generation method
```

> **Important:** The `id` field must match the ID generation method used by your Misskey-TS instance. Check your Misskey-TS `.config/default.yml` for the correct value. Common values: `aidx`, `aid`, `meid`, `ulid`, `objectid`.

### 3.2 Environment Variables

Create `.env`:

```bash
# Path to built frontend assets (JS/CSS bundles)
MISSKEY_FRONTEND_DIR=/path/to/misskey/built/_frontend_vite_

# Path to frontend dist assets (locales, fonts)
MISSKEY_FRONTEND_DIST_DIR=/path/to/misskey/built/_frontend_dist_

# Path to twemoji SVG files
MISSKEY_TWEMOJI_DIR=/path/to/misskey/node_modules/@discordapp/twemoji/dist/svg

# Path to client assets (game images, etc.)
MISSKEY_CLIENT_ASSETS_DIR=/path/to/misskey/packages/frontend/assets

# Path to static assets (favicon, splash, icons)
# MISSKEY_STATIC_DIR=assets
```

## 4. Database Migration

Misskey-Go adds its own tables alongside the existing Misskey-TS tables. Existing data is preserved.

```bash
# Apply Misskey-Go migrations
go run ./cmd/migrate -direction up
```

This creates additional tables required by Go-specific features (e.g., `app`, `auth_session`, `webhook`, `sw_subscription`, `chat_room`, `chat_message`, `bubble_game_record`, etc.).

> **Note:** Misskey-Go migrations are additive and do not modify existing Misskey-TS tables. Both backends can coexist on the same database.

## 5. Stop Misskey-TS

```bash
# Stop the existing Misskey-TS server
# (method depends on your deployment: systemd, pm2, docker, etc.)
systemctl stop misskey
# or
pm2 stop misskey
# or
docker compose stop web
```

## 6. Start Misskey-Go

```bash
# Load environment variables
source .env

# Start the server
./built/misskey -config .config/default.yml
```

Or with explicit environment variables:

```bash
MISSKEY_FRONTEND_DIR=/path/to/misskey/built/_frontend_vite_ \
MISSKEY_FRONTEND_DIST_DIR=/path/to/misskey/built/_frontend_dist_ \
MISSKEY_TWEMOJI_DIR=/path/to/misskey/node_modules/@discordapp/twemoji/dist/svg \
MISSKEY_CLIENT_ASSETS_DIR=/path/to/misskey/packages/frontend/assets \
./built/misskey -config .config/default.yml
```

## 7. Verify

1. Open `http://localhost:3000` in your browser
2. Check that the entrance page loads with proper styling
3. Log in with your existing account
4. Verify:
   - Timeline displays your notes
   - Profile page shows correctly
   - Notifications work
   - File uploads work
   - Reactions work

## Docker Setup

Alternatively, use Docker Compose for a fresh instance:

```bash
docker compose up -d
```

This starts Misskey-Go with PostgreSQL and Redis. See `docker-compose.yml` for details.

## Rollback to Misskey-TS

To switch back to Misskey-TS:

1. Stop Misskey-Go
2. Start Misskey-TS as before

The database is fully compatible in both directions. Misskey-Go's additional tables are ignored by Misskey-TS.

## Known Limitations

### Stub Implementations
The following features return valid responses but do not perform full processing:

- **2FA/WebAuthn** - Returns 204 (not yet implemented)
- **Export/Import** - Jobs are queued but workers are not yet implemented
- **Reversi** - Game listing works but real-time play is not yet implemented
- **Federation (remote)** - Local ActivityPub works; remote object fetching is limited

### Differences from Misskey-TS

- **Timeline** - Falls back to DB query when Redis cache is empty (e.g., after server restart)
- **Identicon** - Generated avatars have a slightly different visual style
- **Notifications** - Not pushed via WebSocket in real-time; visible on page reload

## Troubleshooting

### Page shows "Loading..." forever
- Check that `MISSKEY_FRONTEND_DIR` points to a valid built frontend directory
- Verify the frontend was built successfully (`ls $MISSKEY_FRONTEND_DIR/manifest.json`)

### Emojis not displaying
- Check that `MISSKEY_TWEMOJI_DIR` points to the correct twemoji SVG directory
- Verify: `ls $MISSKEY_TWEMOJI_DIR/1f44d.svg`

### Game images not showing
- Check that `MISSKEY_CLIENT_ASSETS_DIR` points to `packages/frontend/assets/`
- Verify: `ls $MISSKEY_CLIENT_ASSETS_DIR/drop-and-fusion/`

### CSS/styling broken
- Ensure you're using the production build (not Vite dev mode)
- Check that `MISSKEY_FRONTEND_DIST_DIR` is set (for locales/fonts)

### Timeline empty after restart
- This is expected. New notes will appear in the timeline immediately.
- Existing notes will show after the first DB fallback query.

### File upload fails with CREDENTIAL_REQUIRED
- Ensure the auth middleware is processing multipart/form-data requests correctly
- Check server logs for authentication errors

## See also

- [docs/e2e.md](./e2e.md) — Running Misskey upstream's Cypress e2e suite against mk-go via the `third_party/misskey` submodule.
