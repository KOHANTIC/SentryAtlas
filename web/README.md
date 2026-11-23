# SentryAtlas web

The new site: one static Next.js export, replacing `frontend/` and `landing/` at the cutover (#40). Every design value comes from [`../brand/tokens.json`](../brand/BRAND.md), written to `src/app/tokens.css` before each build, and the API's types from [`../docs/api/v2/schema.json`](../docs/api/v2/README.md), written to `src/lib/api.gen.ts` (`npm run gen`).

```bash
npm ci
npm run dev        # http://localhost:3000
npm run build      # the export, in out/
npm test           # unit tests (Vitest)
npm run e2e        # every page in Chromium, desktop and phone, with axe (Playwright; needs a build)
```

## Deploy

Cloudflare Workers static assets serves `out/` (`wrangler.jsonc`), with the headers in `public/_headers`. Workers Builds builds every push: `main` goes to the Worker's own address, and each pull request gets a preview link. It serves sentryatlas.com since the cutover (#40, `docs/deploy/cutover.md`).
