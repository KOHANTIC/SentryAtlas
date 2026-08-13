# Security Policy

## Reporting a vulnerability

**Please do not open a public issue for security problems.**

Email **security@kohantic.com** with:

- what the issue is and where it lives (component, file, endpoint)
- how to reproduce it
- what an attacker could do with it

You'll get an acknowledgement within 5 business days. We'll confirm the issue,
agree a fix timeline with you, and credit you in the release notes unless you'd
rather stay anonymous.

## Scope

In scope:

- the Go API (`backend/`), including how it handles upstream responses
- the map application (`frontend/`) and the landing site (`landing/`)
- the deployment specs in `.do/` and the container images

Out of scope:

- vulnerabilities in the upstream data providers themselves (USGS, NASA EONET,
  NOAA/NWS, GDACS) — report those to the provider
- volumetric denial of service against the public deployment
- findings from automated scanners without a demonstrated impact

## Supported versions

This project has no release branches. Fixes land on `main` and deploy from
there; please report against the current `main`.

## Notes for reviewers

Two properties are worth knowing when assessing a report:

- **Event content is third-party data.** Titles and descriptions come from
  public feeds and are treated as untrusted: the map builds popups as DOM
  nodes with `textContent`, never as HTML.
- **No credentials exist.** All four upstream sources are public and keyless,
  so the service holds no API keys, tokens, or user data — there are no
  accounts and nothing is stored.
