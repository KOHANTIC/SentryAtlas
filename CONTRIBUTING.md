# Contributing to SentryAtlas

Issues and pull requests are welcome. Everyone taking part follows the [code of conduct](CODE_OF_CONDUCT.md).

## Before you start

- Planned work lives in [issues](https://github.com/KOHANTIC/SentryAtlas/issues), grouped into [milestones](https://github.com/KOHANTIC/SentryAtlas/milestones). Each milestone's description lists its issues in the order they are taken.
- For a bug, a typo or a small improvement, go ahead and open a pull request.
- For anything larger, comment on its issue or open one first, so the change is agreed before you write it.
- To report a security problem, do not open an issue. See [SECURITY.md](SECURITY.md).

## Setting up

You need Go 1.27 or newer and Node.js 22 or newer.

```bash
git clone https://github.com/KOHANTIC/SentryAtlas.git
cd SentryAtlas
```

**Backend** (http://localhost:8080):

```bash
cd backend
go run ./cmd/server/
```

Every setting has a default; `backend/.env.example` lists them. The server reads its environment, not a `.env` file.

**Frontend** (http://localhost:3000):

```bash
cd frontend
cp .env.example .env.local
npm install
npm run dev
```

**Landing** (both Next.js apps default to port 3000, so give it another):

```bash
cd landing
npm install
npm run dev -- --port 3002
```

## Sending a change

Every change, the maintainer's included, goes in through a pull request; `main` deploys both sites on every merge, and nobody pushes to it.

1. Start from an up-to-date `main` on a branch named `type/short-subject`, with the type from the commit messages below, as in `git switch -c fix/date-line-bbox`.
2. Keep a pull request to one issue. Its description says `Fixes #N`, or `Part of #N` when merging it must not close the issue.
3. Run the checks below, then open the pull request, titled `type: subject`. The template asks what changed and why, and a few ticks.
4. CI must pass. Its `gate` check is the one `main` requires, and the branch must be up to date with `main`. The weekly vulnerability scan also runs on pull requests and reports problems, but does not block a merge.
5. The maintainer merges it, by rebase so each commit keeps its message, or by squash for a small fix. The branch is deleted afterwards.

## Checks

Run what CI runs, in the parts you changed.

Backend, from `backend/`:

```bash
gofmt -l .                  # prints nothing when formatting is right
go build ./...
go vet ./...
go test -race -count=1 ./...
golangci-lint run           # if installed; CI uses v2.15.0
```

Frontend and landing, from `frontend/` or `landing/`:

```bash
npm run lint
npm run build
```

## Making a change

- Commit messages are `type: subject`, with the type one of `feat`, `fix`, `refactor`, `docs`, `test`, `chore` or `ci`, and a lowercase subject in the imperative, under about 70 characters. The body says why.
- Behavior changes come with tests. For a bug, write the test first and see it fail.
- Comments say why, not what.
- **Go:** formatted with `gofmt`, and kept simple and idiomatic.
- **TypeScript:** follows the ESLint configuration (`npm run lint`).

## Code of conduct

This project follows the [Contributor Covenant](CODE_OF_CONDUCT.md). By taking part, you agree to uphold it.
