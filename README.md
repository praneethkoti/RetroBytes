# RetroBytes

RetroBytes is a small e-commerce web application for a retro electronics store, built with Go (Fiber v2) and SQLite. It supports browsing categories and products, checking local stock by ZIP, a cart and checkout flow, a per-user wishlist, order history, and an admin area for inventory, orders, and users. The code is organized in layers: domain models, repositories for data access, services for business logic, and HTTP handlers, with bcrypt password hashing and cookie-based sessions.

## Secure code review remediation

RetroBytes went through a peer secure code review that raised 9 findings. I verified all 9 against the actual source rather than trusting the report text, then:

- **Fixed 4** confirmed findings (session binding, wishlist authorization, sensitive data in logs, plus the wishlist scoping described below).
- **Applied 1 defense-in-depth partial**: a page-size cap on search and listing queries. The live route was already bounded, so this hardens a latent path rather than closing an active bug.
- **Dismissed 4 as false positives** with written justification (an alleged order IDOR that already had a correct owner check, an open redirect that redirects to a fixed path, a stored XSS where template auto-escaping is intact, and error-message info exposure where responses are already generic).

Each fix landed as its own commit with a regression test that fails without the change and passes with it. The full write-up, including the false-positive justifications and the finding-count reconciliation, is in [documents/RetroBytes - Remediation Summary.md](documents/RetroBytes%20-%20Remediation%20Summary.md).

Highlights:

- **Session hardening**: the session id is rotated on login (session fixation defense), sessions expire after a configurable TTL, and the session and CSRF cookie `Secure` flag is driven by configuration for HTTPS deployments.
- **Wishlist authorization**: wishlist read and write now require an authenticated user, and the wishlist is scoped to the user's id rather than the session id. This is a hardening from session-scoped to user-scoped binding. No cross-user exploit existed before the change, because the key was a server-side value with no client-supplied id, but binding to the user id removes any ambiguity.
- **Log scrubbing**: session ids, CSRF tokens, and raw rejected search input are no longer written to logs (only request id and IP are kept for correlation).
- **No shipped default credentials**: demo accounts seed only when explicitly requested, and the admin account is created only from environment variables with no hardcoded password.

## Run it

Requires Go 1.25 or newer (see [go.mod](go.mod), toolchain `go 1.25.3`). The app serves on `http://localhost:8081`.

Configuration is via environment variables:

| Variable | Default | Purpose |
|---|---|---|
| `SEED_DEMO` | unset (off) | Set to `true` to seed four demo user accounts. |
| `ADMIN_EMAIL` | unset | Admin account email. Required (with the password) to seed an admin. |
| `ADMIN_PASSWORD` | unset | Admin account password. There is no hardcoded default. |
| `COOKIE_SECURE` | `false` | Set to `true` behind HTTPS to mark session and CSRF cookies Secure. |
| `SESSION_TTL_HOURS` | `24` | Authenticated session lifetime in hours. |

Example (PowerShell), seeding demo data and an admin for local exploration:

```powershell
$env:SEED_DEMO='true'; $env:ADMIN_EMAIL='admin@retrobytes.test'; $env:ADMIN_PASSWORD='<choose-a-strong-password>'
go run ./cmd/retrobytes
```

With no environment variables set, the app starts with zero user accounts (safe to run publicly). Open `http://localhost:8081/` to browse.

## Tests

```
go test ./...
```

The suite includes security regression tests labeled with `SR-*` ids in the test names and comments, covering authentication and throttling (`SR-AUTH-*`), authorization including session rotation and expiry and wishlist scoping (`SR-AUTHZ-*`), input validation and template escaping (`SR-VAL-*`), logging and the no-secrets-in-logs guarantee (`SR-LOG-*`), rate and body-size limits (`SR-RATE-*`, `SR-SIZE-*`), error handling (`SR-ERR-*`), and opt-in seeding (`SR-CONF-*`).
