# RetroBytes Secure Code Review Remediation Summary

**Application:** RetroBytes (Go Fiber v2 e-commerce, SQLite, bcrypt auth, session cookies)
**Developer:** Sai Praneeth Koti (ksp@umd.edu)
**Peer review:** "RetroBytes Secure Code Review Report" by an assigned course peer reviewer
**Date of remediation:** 2026-07-23

This document records how each finding from the peer secure code review was handled. Every finding was verified against the actual source before acting. Confirmed and partially valid findings were fixed with a focused commit and a regression test that fails without the fix and passes with it. False positives were reviewed and dismissed with justification, and were not "fixed."

## Finding count reconciliation

The review's summary tallies Critical 2 / High 3 / Medium 3 / Low 2 (10 total), but its detailed body lists only 9 findings (Medium is 2, not 3). There is no hidden tenth finding; the summary's Medium count is a tally error. The CWE label for CSRF also drifts between sections (summary CWE-352, body CWE-351). The 9 detailed findings were treated as authoritative.

## Outcome at a glance

| # | Finding (CWE) | Reported severity | Verdict | Outcome |
|---|---|---|---|---|
| 1 | IDOR in order viewing (CWE-639) | Critical | False positive | Dismissed, no change |
| 2 | Missing authorization on wishlist (CWE-285/862) | Critical | Confirmed | Fixed |
| 3 | Missing CSRF match validation (CWE-352/351) | High | False positive (core) | Dismissed core claim; Secure-flag hardening applied |
| 4 | Weak session binding / no expiration (CWE-522/384) | High | Confirmed | Fixed |
| 5 | Open redirect on login (CWE-601) | High | False positive | Dismissed, no change |
| 6 | Stored XSS via template (CWE-116) | Medium | False positive | Dismissed, no change |
| 7 | DoS via missing pagination (CWE-770) | Medium | Partially valid | Fixed (defense in depth) |
| 8 | Sensitive data in logs (CWE-532) | Low | Confirmed | Fixed |
| 9 | Info exposure via error messages (CWE-200) | Low | False positive | Dismissed, no change |

## What was fixed

### Finding 4 (Confirmed, High): Weak session binding, no expiration, session fixation
Commit `fix(auth): harden sessions against fixation, add expiry, config-driven Secure cookie`.

The session model had three real defects: sessions never expired, the session id was not regenerated on login (so a session id planted before login stayed valid afterward, a classic session fixation), and the session cookie hardcoded `Secure: false`.

Changes:
- **Rotation on login:** `AuthService.Login` now mints a fresh session id and calls `UserRepo.RotateSession`, which re-keys the caller's cart and wishlist to the new id and deletes the old anonymous session row in one transaction. A pre-login session id no longer resolves a user.
- **Expiration:** the `sessions` table gained an `expires_at` column (idempotent migration for existing databases). `BindSession` sets `now + TTL`; `SessionUser` rejects sessions past their expiry, so a stolen or idle session stops working. TTL is configurable via `SESSION_TTL_HOURS` (default 24h).
- **Secure cookie flag:** the session and CSRF cookie `Secure` flag is now driven by config (`COOKIE_SECURE`, default false for local http). The four duplicated `ensureSID` copies were consolidated into one shared helper so cookie flags live in one place.

Regression tests: `TestSessionRotatedOnLogin` (SR-AUTHZ-05), `TestExpiredSessionRejected` (SR-AUTHZ-06).

### Finding 2 (Confirmed): Missing authorization on wishlist
Commits `fix(wishlist): require authentication for wishlist read and write routes` and `fix(wishlist): scope wishlist to authenticated user id`.

The wishlist save, unsave, and list routes were reachable anonymously and scoped only by the anonymous `sid` cookie, so a wishlist was never bound to an authenticated identity. This is not the horizontal IDOR the report suggested (the wishlist id is derived server-side, never from client input, so one user cannot target another user's wishlist by request tampering), but the routes genuinely required no login and the wishlist was tied to a session rather than a user.

Changes, in two steps:
1. The three wishlist routes are gated by `RequireUser`, so wishlist operations require a logged-in user. Anonymous callers are redirected to `/login`.
2. The wishlist is now scoped to the authenticated user's id rather than the session id. `WishlistHandler.List`, `Save`, and `Unsave` read the authenticated user from `c.Locals("user")` (set by `RequireUser`) and pass `user.ID` to the wishlist service (`internal/http/handlers/wishlist_handler.go`). The wishlist row is therefore keyed by the user id, so a user can only ever read or modify their own wishlist, independent of session. `UserRepo.DeleteUserCascade` also deletes the user-keyed wishlist so account deletion stays clean.

Regression tests: `TestWishlistRequiresAuth` (SR-AUTHZ-07) for the auth gate, and `TestWishlistScopedToUser` plus `TestWishlistRepoKeyedByOwner` (SR-AUTHZ-08) for the user-id scoping. The scoping test asserts the wishlist row is keyed by the user id (`u-alice`) and not by the session id, and was verified to fail when the handler keys by session id instead.

### Finding 7 (Partially valid, Medium): DoS via missing pagination limits
Commit `fix(catalog): cap search and listing page size to bound query work`.

The `/search` route was already bounded in practice (it passes a fixed page size of 20 and the SQL uses `LIMIT`), but `CatalogService.Search` and `ListProductsByCategory` enforced only a floor on page size, not a ceiling. A future caller passing a large page size could request unbounded rows.

Change: a `maxPageSize` cap (50) is applied via a shared `clampPageSize` helper on both methods, so no caller can exceed the cap. This is defense in depth; current behavior is unchanged.

Regression test: `TestSearchPageSizeIsCapped` (SR-RATE-02).

### Finding 8 (Confirmed, Low): Sensitive data in logs
Commit `fix(logging): stop logging session ids, CSRF tokens, and raw search input`.

The session id (the authentication credential) was written to logs in the admin access-denied path, the order-placement failure path, and logout; the submitted CSRF token and the raw rejected search input were also logged verbatim.

Changes:
- `access.denied.admin`: dropped the raw `sid`.
- `order.place.fail`: dropped the raw `sid`; the error is now recorded in the dedicated `err` field.
- `csrf.fail`: dropped the submitted token value.
- search `validation.fail`: logs the input length instead of the raw value (avoids log injection and storing attacker-controlled content).
- logout stopped logging `sid` (in the session-hardening change).

Request id and IP (recorded automatically) remain for correlation. Email in auth logs is intentionally retained: it is needed for authentication auditing and is not a credential.

Regression test: `TestLogsDoNotLeakSensitiveData` (SR-LOG-05).

### Additional hardening: demo credential removal (portfolio quality)
Commit `fix(seed): make demo and admin account seeding opt-in via environment`.

Not a review finding, but required to make the app safe to show publicly. Previously five accounts (four demo users and one admin), all with the hardcoded password `Passw0rd!`, were seeded on every startup.

Changes:
- Demo user accounts seed only when `SEED_DEMO=true`.
- The admin account seeds only when `ADMIN_EMAIL` and `ADMIN_PASSWORD` are both set. There is no hardcoded admin password, so no fixed admin credential ships.
- With nothing opted in, the database starts with zero accounts.

Regression test: `TestSeedingIsOptIn` (SR-CONF-01).

## What was dismissed (reviewed, no change)

### Finding 1 (Critical): IDOR in order viewing (CWE-639)
**False positive.** The order View handler enforces a real owner check: access is denied (404) unless the caller's session id matches the order's session id, or the caller's authenticated user id matches the order's user id; admins are allowed. An authenticated user A viewing user B's order is denied. The claimed enumeration/IDOR does not exist. Evidence: `internal/http/handlers/order_handler.go` View handler ownership comparison.

### Finding 3 (High): Missing CSRF match validation (CWE-352/351)
**Core claim is a false positive.** The report hedged that CSRF "risk increases" "if Fiber middleware rejects only absent tokens but not mismatches." Fiber's CSRF middleware (configured globally with default storage) performs double-submit match validation, not presence-only. A forged token is rejected with 403 (empirically confirmed by the CSRF-failure path exercised in `TestLogsDoNotLeakSensitiveData`). The genuine residual weakness was the `Secure: false` cookie flag, which is now config-driven (fixed alongside Finding 4). The cookie-to-field token sync (JS plus a server-side fallback) is inherent to Fiber's double-submit pattern and is required for legitimate form submissions; removing it would break CSRF-protected forms, so it was retained. Evidence: `cmd/retrobytes/main.go` CSRF config.

### Finding 5 (High): Open redirect on login (CWE-601)
**False positive.** The report itself flagged this as conditional ("not shown here but typical"). The login handler redirects to the hardcoded string `/`. It reads no `next`, `redirect`, `return`, or similar parameter from the query or form. There is no user-controlled redirect target. Evidence: `internal/http/handlers/auth_handler.go` Login redirect.

### Finding 6 (Medium): Stored XSS via template (CWE-116)
**False positive.** No template or Go code uses `template.HTML`, `safeHTML`, a `| html` pipeline, or any custom unescaping function. Product `.Description`, `.Title`, and all user or admin controlled fields render through plain `{{ }}` actions, which `html/template` contextually auto-escapes. An existing test (`TestTemplateAutoEscape`) already guards this. Evidence: `web/templates/product.html` and a grep of all templates and `.go` files.

### Finding 9 (Low): Information exposure via error messages (CWE-200)
**False positive.** The global error handler logs the error server-side and returns only a generic message to the client; the fallback path also sends a fixed generic string, not the raw error. An existing test (`TestErrorHandlerFriendlyMessage`) already asserts that internal error text does not reach the client. Evidence: `cmd/retrobytes/main.go` ErrorHandler.

## Verification

- `go build ./...` clean; `go test ./...` all green (existing plus new regression tests).
- Each fix was confirmed to fail without the change and pass with it.
- Smoke tested on port 8081: without seed env vars the demo admin login is rejected and no admin is seeded; with the env vars set all five accounts are seeded and reachable.

## New configuration (see the Build and Setup Guide for details)

| Variable | Default | Purpose |
|---|---|---|
| `SEED_DEMO` | unset (off) | Set to `true` to seed the four demo user accounts. |
| `ADMIN_EMAIL` | unset | Admin account email. Required (with password) to seed an admin. |
| `ADMIN_PASSWORD` | unset | Admin account password. No hardcoded default. |
| `COOKIE_SECURE` | `false` | Set to `true` behind HTTPS to mark session and CSRF cookies Secure. |
| `SESSION_TTL_HOURS` | `24` | Authenticated session lifetime in hours. |

## Documentation notes

- The Build and Setup Guide and User Guide (.docx) were updated for all of the above: the printed demo/admin passwords were removed, the new environment variables were documented, and the session, wishlist, logging, and seeding behavior was described.
- The previous .docx-derived PDF copies of these two guides were stale (they still showed the old demo passwords) and were removed. The .docx files are now the single source of truth. Re-export them to PDF from Word before submitting if a PDF copy is required.

