# Contributing to SentryAtlas

Thanks for your interest in contributing! Here's how you can help.

## Reporting Bugs

Open a [bug report](https://github.com/KOHANTIC/SentryAtlas/issues/new?template=bug_report.md) with:

- A clear description of the problem
- Steps to reproduce
- What you expected to happen
- Your environment (OS, browser, relevant versions)

## Suggesting Features

Open a [feature request](https://github.com/KOHANTIC/SentryAtlas/issues/new?template=feature_request.md) describing the problem you'd like solved and your proposed approach.

## Development Setup

### Prerequisites

- Go 1.22+
- Node.js 22+
- Git

### Getting Started

```bash
git clone https://github.com/KOHANTIC/SentryAtlas.git
cd SentryAtlas
```

**Backend:**

```bash
cd backend
cp .env.example .env
go mod download
go run ./cmd/server/
```

**Frontend:**

```bash
cd frontend
cp .env.example .env.local
npm install
npm run dev
```

**Landing:**

```bash
cd landing
npm install
npm run dev
```

## Pull Requests

1. Fork the repo and create a branch from `main`
2. Make your changes
3. Test locally — make sure the backend starts, the frontend renders, and nothing is broken
4. Use [conventional commits](https://www.conventionalcommits.org/) for your commit messages (`feat:`, `fix:`, `refactor:`, `docs:`, etc.)
5. Open a PR with a clear description of what changed and why

## Code Style

- **Go** — format with `gofmt`. Keep it simple and idiomatic.
- **TypeScript** — follow the existing ESLint configuration (`npm run lint`).

## Code of Conduct

This project follows the [Contributor Covenant](CODE_OF_CONDUCT.md). By participating, you agree to uphold it.
