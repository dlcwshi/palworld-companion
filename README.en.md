# Palworld Companion

[简体中文](README.md) | **English**

[![License: MIT](https://img.shields.io/badge/License-MIT-green.svg)](LICENSE)
![Go](https://img.shields.io/badge/Go-1.24%2B-00ADD8)
![Vue](https://img.shields.io/badge/Vue-3-42b883)

Palworld Companion is a self-hosted, mobile-first PWA for Palworld players. Its Go backend accesses a strict read-only Palworld REST API allowlist, stores Companion-owned accounts and tasks in SQLite, and embeds the Vue frontend in one executable.

**Current repository version: 0.4.4-dev.** This is not a v0.4.4 tag or formal release.

## Current capabilities

- Server dashboard, metrics, and a privacy-filtered persistent player roster.
- Mandatory first-run creation of a local administrator.
- Administrators log in with a local username/password; players can log in with a character name or SteamID64 and local password.
- Player applications require the character to be online. The backend freshly finds one exact case-sensitive character name and parses its strict `userId=steam_<SteamID64>` identity before administrator approval.
- Approval, rejection, disable, soft delete, restore, role management, session revocation, and password reset.
- Personal and shared tasks with SQL- and service-level authorization and mutually exclusive, deduplicated home groups.
- Auto-updating mobile-first PWA, SQLite WAL, pure-Go Linux AMD64 binary, and systemd deployment.

The server derives SteamID64 from the online player identity and keeps it as a compatible login identifier. Steam OpenID is disabled. Companion does not contact `steamcommunity.com`, the Steam Web API, or an external authentication broker.

## Authentication flow

### First administrator

For an empty database, or a schema 3 database with no administrator, `GET /api/v1/setup/status` returns `setupRequired=true`. The frontend routes every application page to `/setup`. Creation of the administrator, `setup_completed=true`, and its session happen in one transaction, so only one concurrent request can succeed.

Setup never reopens automatically, even if administrators later become unusable. Recovery is available through the CLI.

### Player application

1. Join this Palworld server and remain online.
2. Submit the in-game character name and a local password at `/register`.
3. The backend calls a fresh `/players` result and requires one exact, case-sensitive online character-name match without cache or stale fallback.
4. The backend strictly parses `userId=steam_<SteamID64>` from that player and creates a `role=player,status=pending` application.
5. After administrator approval, the player can log in with the character name or SteamID64. Existing active users can log in while offline or while the Palworld API is unavailable.

Pending, disabled, rejected, and deleted users cannot log in. Unique SteamID64, Palworld userId, and stable playerId constraints prevent repeated applications from bypassing state.

A successful login returns to `/` by default. Only an explicitly requested `/tasks` or administrator-users page retains an internal return target. Account, authentication, setup, unknown, and external targets all resolve to home, so a PWA restored on an expired `/account` session does not reopen the password page after login.

## Passwords and sessions

- Passwords use Argon2id with an independent random salt and upgradeable PHC parameter encoding; accepted length is 8–128 bytes.
- Plaintext passwords are never stored, and plain SHA-256 is not used as a password hash.
- Raw session tokens exist only in Secure, HttpOnly, SameSite=Lax, Path=/ browser cookies. SQLite stores only their SHA-256 hashes.
- A user password change retains the current session and revokes the others. Administrator reset revokes every session of the target user.
- Login, registration, setup, password-change, and password-reset endpoints have bounded in-process rate limiting.

See [docs/security.md](docs/security.md) and [docs/architecture.md](docs/architecture.md).

## Task authorization

- A player cannot list or access another player's personal tasks, including by ID.
- Shared tasks are visible to every authenticated user and editable only by their creator or an administrator.
- Administrators can manage all tasks. Unauthorized object access returns 404.
- Schema 5 preserves existing `owner_id`, `created_by`, and `visibility` data.
- The home page derives both groups from one `scope=visible,status=pending` result: it deduplicates by stable task ID, then assigns the current user's personal tasks and all shared tasks to mutually exclusive groups.

## Quick start

Go 1.24+, Node.js, and npm are required for development. Docker, CGO, an external database, and Steam services are not required.

```powershell
cd frontend
npm.cmd ci
npm.cmd run build
cd ..
go test ./...
go run ./cmd/companion --config deploy/config.example.yaml
```

Open <http://127.0.0.1:8091>. The example uses mock mode and `./data/companion.db`.

Legacy `auth.enabled`, `public_base_url`, and `admin_steam_ids` keys remain accepted but are unused. `auth.session_ttl` still controls session lifetime.

## Persistent player roster

The current version stores every fully validated fresh /players snapshot in the schema 5 SQLite player_roster. The stable identity key is internal palworld_user_id; character names are display and local-login lookup values only. Public responses never include SteamID64, Palworld user/player IDs, account names, IP addresses, or database IDs.

Only a fresh, complete, valid snapshot may change presence and advance player_roster_last_success_at. Upstream failures, malformed payloads, transaction failures, TTL hits, and SQLite fallback never mark everyone offline or extend last-online timestamps. During a failure, the persisted roster remains visible while every current status is unknown; it survives Companion restarts. Version 0.4.3-dev starts failure cooldown when the upstream attempt completes, preventing queued requests from immediately repeating a slow /players timeout.

Last online means the last successful snapshot in which Companion observed the identity online. This release has no online-duration statistics, history charts, or always-on background poller. Existing home, summary and player requests trigger refreshes, while character registration still forces a real-time /players request and never binds from normal TTL cache, failure backoff, or the persisted roster.

The home page shows the complete persistent roster by default and filters the existing response locally by All, Online, or Offline. Unknown presence returns to All and keeps every historical player visible. Ordinary TTL hits update normally without exposing cache implementation labels.

## PWA updates

The production build empties and regenerates `web/dist`, then verifies the home bundle, hashed assets, and service-worker precache before Go embeds that same directory. The PWA uses `autoUpdate`, `skipWaiting`, and `clientsClaim`; it checks on launch, foreground return, network recovery, and hourly, and the standard registration reloads once when the updated worker activates. Secure/HttpOnly login cookies are not cleared.

The manifest fixes both `start_url` and `scope` at `/`. The mobile home keeps a compact single-line brand, compact server status, 2×2 metrics, consistent inline-SVG navigation, safe-area spacing, and the existing task/roster behavior. The account page shows only the account overview and settings actions by default; its password form expands on demand and clears sensitive input when cancelled or left. Password and cookie implementation details remain in the security documentation.

`index.html`, the manifest, and `sw.js` use `no-cache` revalidation, while content-hashed `/assets/` are long-lived and immutable. The service worker neither precaches `/api/` nor adds an API runtime cache.

## API

Public and setup:

- `GET /api/v1/health`
- `GET /api/v1/system/version`
- `GET /api/v1/system/capabilities`
- `GET /api/v1/server/summary`
- `GET /api/v1/server/players`
- `GET /api/v1/setup/status`
- `POST /api/v1/setup/admin`

Authentication:

- `POST /api/v1/auth/register`
- `POST /api/v1/auth/login`
- `POST /api/v1/auth/change-password`
- `GET /api/v1/auth/me`
- `POST /api/v1/auth/logout`
- Legacy Steam and callback routes return `410 steam_auth_disabled`

Administration:

- `GET /api/v1/admin/users?status=pending|active|disabled|rejected|deleted`
- `POST /api/v1/admin/users/{id}/approve|reject|reset-password|role`
- `POST /api/v1/admin/users/{id}/disable|enable|restore|revoke-sessions`
- `DELETE /api/v1/admin/users/{id}` (soft delete)

Tasks use `GET|POST /api/v1/tasks` and `GET|PATCH|DELETE /api/v1/tasks/{id}`.

## Recovery CLI

Passwords are read without echo from an interactive TTY and are never accepted as command-line arguments:

```bash
palworld-companion setup status --config /etc/palworld-companion/config.yaml
palworld-companion users create-admin --config /etc/palworld-companion/config.yaml --username <username>
palworld-companion users approve --config /etc/palworld-companion/config.yaml --steam-id <SteamID64>
palworld-companion users reject --config /etc/palworld-companion/config.yaml --steam-id <SteamID64>
palworld-companion users reset-password --config /etc/palworld-companion/config.yaml --steam-id <SteamID64>
palworld-companion users reset-password --config /etc/palworld-companion/config.yaml --username <username>
```

Password-requiring commands fail safely without a TTY.

## Database upgrade and rollback

Schema 5 adds the persistent player roster while schema 4 adds local usernames, Argon2id password hashes, approval/rejection audit fields, and persistent `system_settings.setup_completed`. Schema 3 user IDs, sessions, users, and tasks are preserved. Legacy Steam users without a password cannot log in until an administrator resets their password. Newer-than-supported schemas are rejected.

Before upgrading, stop only Companion and back up the executable, configuration, database, WAL, and SHM. Migration failure rolls back and prevents startup. To roll back the binary, restore the pre-upgrade database files as well; an old executable cannot open schema 5.

See [docs/deployment.md](docs/deployment.md) for the complete procedure.

## Security boundary

The backend only calls Palworld `/info`, `/metrics`, and `/players`. REST credentials, player IPs, and raw upstream payloads do not reach the frontend. Companion does not read or write saves, depend on the PST database, or modify Palworld configuration. Never commit real configuration, passwords, databases, cookies, tokens, or private player identifiers.

## License

Original source is licensed under the [MIT License](LICENSE). Third-party data and assets retain their own licenses; see [NOTICE](NOTICE). This project is not affiliated with or endorsed by Pocketpair, Inc.
