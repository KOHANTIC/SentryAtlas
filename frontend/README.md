# SentryAtlas — Frontend

Interactive map application built with Next.js and MapLibre GL that visualizes real-time disaster events from the SentryAtlas backend API.

## Prerequisites

- Node.js 22+
- The [backend](../backend/) running on `http://localhost:8080`

## Getting Started

```bash
cp .env.example .env.local
npm install
npm run dev
```

Open [http://localhost:3000](http://localhost:3000) to see the map.

## Environment Variables

| Variable | Description |
|----------|-------------|
| `NEXT_PUBLIC_API_URL` | Backend API URL (default: `http://localhost:8080`) |

## Project Structure

```
src/
├── app/                 # Next.js app router (page, layout, global styles)
├── components/          # MapView, FilterPanel, Legend, EventDetail
├── hooks/               # useEvents — data fetching hook
└── lib/                 # API client, types, map styles, time utilities
```

## Tech Stack

- Next.js 16 with App Router
- React 19
- MapLibre GL for map rendering
- Tailwind CSS v4
- TypeScript (strict mode)
