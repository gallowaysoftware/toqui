# Security policy

Thanks for taking the time to disclose a security issue responsibly.

## Reporting a vulnerability

Email **kyle@thegalloways.ca** with:

- A description of the issue and its impact.
- Reproduction steps (a `curl` invocation, a minimal repro repo, screenshots — whatever you have).
- The version / commit you found it on.
- Optionally, your suggested fix.

Please do **not** file public GitHub issues for security reports, and do
not post the vulnerability publicly until a fix is available.

I'll acknowledge your report within 7 days and aim to ship a fix within
30 days for high-severity issues, 90 days for everything else. If the
issue requires coordinating with a third party (e.g. upstream library
maintainers, an AI provider), the timeline may stretch — I'll keep you
posted.

## Scope

In scope:

- The code in this repository (`gallowaysoftware/toqui`): the Expo
  React Native frontend and the Go backend under `./backend/`.
- The `gallowaysoftware/toqui-site` repository (the transition page
  Astro site at `toqui.travel`).

Out of scope:

- Third-party services Toqui depends on at runtime (AI providers,
  Google OAuth, Postgres, Firestore). Report those to the relevant
  vendor.
- Issues in third-party dependencies — if you find one, report it
  upstream and let me know so I can pin / bump.
- Vulnerabilities that require a malicious or compromised operator
  (the operator already has root on the instance; we're not trying to
  defend against them in a single-tenant self-host).

## Disclosure

Once a fix is merged and released, I'll publish a GitHub Security
Advisory crediting you (unless you'd prefer to remain anonymous).
