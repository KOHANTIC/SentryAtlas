# SentryAtlas — Landing Page

Static marketing site for SentryAtlas. Built with Next.js using `output: "export"` to produce a fully static site.

## Prerequisites

- Node.js 22+

## Getting Started

```bash
npm install
npm run dev
```

Open [http://localhost:3000](http://localhost:3000) to preview.

## Building

```bash
npm run build
```

This produces a static `out/` directory containing the complete site, ready to deploy to any static hosting provider.

## Configuration

The privacy page (`/privacy/`) names whoever runs the site, read at build time, so a copy of this project never publishes another operator's details as its own:

| Variable | Example | Without it |
|---|---|---|
| `SITE_OPERATOR` | `Example Org LLC` | The page says this copy does not name its operator. |
| `SITE_PRIVACY_EMAIL` | `privacy@example.org` | No contact address is shown. |

On App Platform they are build-time environment variables of the landing component, set before a change that needs them is merged.

## Tech Stack

- Next.js 16 with static export
- React 19
- Tailwind CSS v4
- TypeScript
