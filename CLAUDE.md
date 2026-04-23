# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Commands

```bash
# Build
go build ./...

# Run (requires .env populated)
go run ./cmd/server

# Tailwind CSS (one-shot)
npm run css

# Tailwind CSS (watch mode during development)
npm run css:watch

# Run migrations (against Supabase or local Postgres)
psql $DATABASE_URL < migrations/001_init.sql

# Deploy Cloudflare Worker
cd worker && wrangler deploy
```

## Architecture

**Request flow:** All traffic to `*.fader.bio` hits the Cloudflare Worker (`worker/index.js`) first. The worker extracts the subdomain, injects it as `X-Fader-Subdomain`, and proxies to Railway. The Go app reads this header in `middleware.SubdomainFromHeader` and routes accordingly: empty/www → landing page, `admin` subdomain → admin panel, anything else → DJ profile lookup.

**Auth:** Supabase handles email/password auth. On login/register the Go app receives a JWT from `supa.Auth` and stores it in an `HttpOnly` cookie (`sb-token`). Every protected route validates this JWT via `middleware.RequireAuth` using the `SUPABASE_JWT_SECRET`. The JWT `sub` claim is the user's UUID, matching `users.id`.

**Profiles vs Users:** A `users` row is the account (one per email). A `profiles` row is a DJ page/subdomain. Free users have 1 profile; Pro users can have multiple (different personas). Profile `handle` = subdomain.

**Block editor:** The `/edit` page is HTMX-powered. Adding/updating/deleting blocks sends partial requests; the server returns rendered `block_editor.html` partials that swap in-place. Block ordering uses SortableJS on the client which POSTs to `PATCH /blocks/order`. Block content is stored as JSONB in `blocks.data` and unmarshalled in templates via custom `FuncMap` functions (`unmarshalBio`, `unmarshalSocial`, etc.) defined in `internal/handlers/render.go`.

**Click tracking:** All outbound links on public profiles use `/r/:blockID` — the server records the click then 302s to the real URL.

**Stripe:** Pro tier is a monthly subscription. `GET /billing/checkout` creates a Stripe Checkout session. Webhooks at `POST /webhooks/stripe` handle `checkout.session.completed` (set tier=pro), `customer.subscription.deleted` (set tier=free), and `customer.subscription.updated`.

**Templates:** Go `html/template` with a shared `base.html`. Profile themes (`minimal`, `dark`, `neon`) are separate full-page HTML files (`profile_minimal.html` etc.), not variants of base. Each theme file also defines its own `block_*` named template used for rendering blocks.

**Admin:** Single superuser identified by `ADMIN_USER_ID` env var (your Supabase user UUID). Protected by `admin.RequireAdmin` middleware. The admin subdomain (`admin.fader.bio`) is reserved.

## Environment Variables

See `.env.example`. Key ones:
- `SUPABASE_JWT_SECRET` — found in Supabase dashboard → Settings → API → JWT Secret
- `ADMIN_USER_ID` — your own Supabase user UUID (set after first login)
- `STRIPE_PRICE_ID` — the recurring price ID for the Pro plan
- `RAILWAY_ORIGIN` — set in Cloudflare Worker env vars (not in Go app)

## Supabase Storage (avatar uploads)

Avatar upload requires a Supabase Storage bucket named `avatars` with the following setup:

1. In Supabase dashboard → Storage → New bucket → name: `avatars`, Public: **on**
2. Add a policy allowing authenticated users to upload:
   - Policy name: `Users can upload their own avatars`
   - Operation: INSERT
   - Target roles: `authenticated`
   - USING expression: `true`
3. The Go app uploads using the user's JWT token (`sb-token` cookie) as the bearer token.
   Path format: `avatars/{profileID}-avatar` (no extension; content-type set via header).

## Password reset

The `/auth/forgot-password` flow calls Supabase GoTrue `POST /recover`. Supabase will email
the user a magic link that redirects to `{Site URL}/auth/reset-password#access_token=...&type=recovery`.

Configure **Site URL** in Supabase dashboard → Authentication → URL Configuration to match your
`BASE_DOMAIN` (e.g. `https://fader.bio`). Also add `https://fader.bio/auth/reset-password` to
the **Redirect URLs** allowlist.
