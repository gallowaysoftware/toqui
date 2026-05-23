# Contributing

Thanks for wanting to contribute. Toqui is a small project run by one
maintainer, so the contribution process is correspondingly small.

## Quick links

- [Project layout](README.md#layout)
- [Self-host quickstart](README.md#quickstart-self-host)
- [Development quickstart](README.md#quickstart-development)
- [Deploy patterns](DEPLOYMENT.md)
- [Security policy](SECURITY.md)
- [Code of conduct](CODE_OF_CONDUCT.md)
- Architecture: [CLAUDE.md](CLAUDE.md) (frontend) + [backend/CLAUDE.md](backend/CLAUDE.md) (backend)

## Workflow

1. **Fork** the repo and create a feature branch off `main`.
2. **Make the change.** Keep PRs small and focused. One concern per PR.
3. **Run the relevant checks locally:**
   - Frontend: `pnpm typecheck && pnpm test`
   - Backend: `cd backend && go build ./... && go test ./...`
4. **Open a PR.** A clear title + a short summary of *why* is more
   valuable than a long description of *what* (the diff says what).
5. **CI runs typecheck + tests on both halves.** Fix anything it flags.
6. Reviews are best-effort — give it a few days before pinging.

## What's in scope

Toqui is a **single-tenant self-hosted app**. Each instance is run by
one operator, for themselves or a small group of users sharing the
operator's AI provider keys. New features should be designed for that
shape.

**In scope:**
- Bug fixes, performance work, accessibility, security
- New trip-planning / chat / itinerary features that work well for a
  small group of users
- New AI provider integrations (anything OpenAI-compatible should
  Just Work via `OPENAI_BASE_URL`; net-new provider APIs are also
  welcome — see `backend/internal/ai/` for the Provider interface)
- Better self-host docs, more deploy patterns
- Internationalization

**Out of scope** (these were removed in the SaaS-to-OSS pivot and won't
be reintroduced upstream — feel free to maintain a fork):
- Monetization (subscriptions, in-app purchases, ad networks, affiliate
  bias in recommendations)
- Multi-tenant admin panels, billing, customer-support tooling
- Analytics / telemetry that phones home anywhere by default
- Identity providers beyond optional Google OAuth (Facebook / Apple /
  third-party SSO)
- Waitlists, invite codes, capacity caps, gating signups

## Licensing

This project is [AGPL-3.0-or-later](LICENSE). By submitting a PR you
agree your contribution is licensed under the same terms.

There's no CLA. If you make a non-trivial contribution, please add a
`Signed-off-by:` line to your commits (a [DCO](https://developercertificate.org/)
sign-off) using `git commit -s`. This says you have the right to
contribute the change under the AGPL.

## Commit + PR style

- One conventional-commit prefix per commit: `feat:`, `fix:`, `chore:`,
  `docs:`, `refactor:`, `test:`.
- PR titles use the same prefixes (so the squash-merge commit message
  matches).
- The "why" goes in the commit body or PR description, not the title.
- Don't add unrelated cleanups to a feature/bug PR — open a separate
  `chore:` PR instead.

## Reporting issues

For non-security bugs / feature requests:
https://github.com/gallowaysoftware/toqui/issues

For security issues: see [SECURITY.md](SECURITY.md).
