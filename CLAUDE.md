# SentryAtlas

Real-time disaster monitoring: a Go API and one site (`web/`) with the map. The README is the project description; this file carries only what a session needs before editing.

**Kyro uses he/him.**

## A push here is a publication

This repo is **public** (`KOHANTIC/SentryAtlas`, AGPL) and **`main` deploys on every merge**: the site at `sentryatlas.com` (Cloudflare Workers) and the API at `map.sentryatlas.com` (DigitalOcean). A ruleset on `main` requires a pull request, the `gate` check green and up to date, and linear history.

Who does what (Kyro, 2026-11-22): Claude commits on a `type/short-subject` branch and, behind an approval prompt each time, pushes the branch, opens and edits the pull request, and applies repo settings. **Kyro merges every pull request**, and runs installs, servers, `doctl`, DigitalOcean, Cloudflare and DNS changes. An autonomous run, in which Claude picked issues and merged on green CI, ran from the evening of 2026-11-22 to 2026-11-23 and ended at Kyro's word after his final review; it does not resume without a new grant from him. The root `CLAUDE.md` § Working with Kyro item 5 owns the publication rule and records this exception.

Everything committed here is public. Nothing from the private parts of the tree (`KYROKOHAN/Finance/`, `Lore/PERSONAL.md`, `Lore/COMPENSATION.md`, `KOHANTIC/TaxDocs|Contractors|Documents/`) belongs in this repo, in a commit message, an issue or a pull request.

## How work is tracked

- GitHub issues and milestones are the plan; live state is `gh issue list -R KOHANTIC/SentryAtlas`, never a copy. Each milestone's description lists its issues in order.
- An issue has a type (Task, Bug or Feature), one `area:` label (`api`, `data`, `map`, `site`, `design`, `infra`, `docs`), a milestone and the assignee `kyrokohan`. A parked idea has `deferred`, no milestone, and a dated line in its Notes saying what it waits on.
- An issue body has five headings: What, Why, Scope (a checklist), Out of scope, Notes. Issue and pull request text is public: no account, DNS or personal details. The private reasons behind product decisions are kept in `../../KOHANTIC/KOHANTIC.md`.
- Every issue gets a written plan before code, and the pull request description carries it. A pull request carries its issue's labels and milestone and says `Fixes #N`, or `Part of #N` when it must not close the issue.
- After a merge, read `main`'s CI job by job for the merged commit: a watch can print green before any check exists.

## Deploys

- `.do/app.yaml` is applied by hand, not on push; a merge deploys code only. A new environment variable is set on its component in DigitalOcean **before** the pull request that reads it is merged: on 2026-11-22 the backend ran without `CLIENT_IP_HEADER` until it was added there.
- The site's build variables (`SITE_OPERATOR`, `SITE_PRIVACY_EMAIL`) live in the Worker's Builds settings, for Production and for Previews Base. A branch's preview copies the base when it is first built, so a branch older than a new variable lacks it; production is what counts.
- The backend's startup line names `client_ip_header`, and it must say `Do-Connecting-IP`. The platform's edge overwrites that header with the visitor's address; `X-Forwarded-For`, `True-Client-IP`, `Forwarded` and `X-Client-IP` carry whatever the client sent. Empty puts every visitor in one rate-limit bucket.

## CI pins

- `gate` in `ci.yml` is the only required check. `govulncheck` in `vuln.yml` runs on pull requests, on `main` and weekly, and stays out of `gate` so a standard-library report cannot block every pull request until Go is updated.
- CI takes the Go release from the `FROM golang:X.Y-alpine` line of `backend/Dockerfile`.
- Dependabot moves the actions, which are pinned to a commit, but not the golangci-lint version in `ci.yml` or the govulncheck version in `vuln.yml`. Bump those by hand, and check a govulncheck release's `go.mod` first: it may need a newer Go than the backend's.

## Authorship framing

SentryAtlas lives under the KOHANTIC organization and is copyright KOHANTIC, unlike GoLineArt and getcat, which are Kyro's own repos. `../portfolio-v2/CLAUDE.md` § Content scope records Kyro's decision (2026-08-13) that the portfolio presents it as **his work**, with KOHANTIC mentioned only as where the repo lives. Do not rewrite it either way.

## Decided, do not re-raise

The README footer and the live sites say **"Made by the KOHANTIC team"**, which implies a headcount that `../../KOHANTIC/KOHANTIC.md` §0 rule 2 forbids. Kyro was asked on 2026-11-07 and chose to leave it. Do not change it and do not flag it again.

## Facts about Kyro

Any claim about his background in the README or the sites is owned by `../../KYROKOHAN/Lore/`. Never invent one.

## Screenshots

MapLibre stops painting when Chrome's window is occluded (requestAnimationFrame throttling), so a map screenshot from a background browser comes back blank even though the app is fine. In Kyro's own visible Chrome (Claude in Chrome, which he allowed for checking Cloudflare and previews on 2026-11-23) the map paints. `docs/screenshot.png` is his to choose.
