# Cutover: one site at sentryatlas.com (#40)

The new site (`web/`, on Cloudflare Workers static assets) takes over sentryatlas.com. The old map host keeps its API and redirects its pages. The steps outside the repo are the maintainer's; the checks after each are scripted below.

## Before

- `main` builds and deploys the Worker `sentryatlas-web`, and its own address serves every page.
- The Worker's production build variables hold `SITE_OPERATOR` and `SITE_PRIVACY_EMAIL`.
- `WAITLIST_ORIGINS` on the API includes `https://sentryatlas.com` (the default does).

## 1. Rehearse on a spare name

Cloudflare, Workers & Pages, `sentryatlas-web`, Settings, Domains & Routes, Add, Custom domain: `next.sentryatlas.com`. Cloudflare creates its DNS record and certificate.

Check: every page answers, the headers are the site's, and the analytics beacon is injected.

```
for p in / /map/ /data/ /developers/ /privacy/ /terms/ /alerts/ /waitlist/confirm/ /og-image.png /sitemap.xml; do curl -s -o /dev/null -w "%{http_code} $p\n" "https://next.sentryatlas.com$p"; done
```

## 2. Switch sentryatlas.com

1. DNS, the `sentryatlas.com` record: copy its target somewhere safe (the way back needs it), then delete the record.
2. The Worker: Add, Custom domain: `sentryatlas.com`.

The apex is down for the minute between the two. Check as in step 1, against `https://sentryatlas.com`.

## 3. Redirect the old map host

Rules, Redirect Rules, two rules in this order:

1. **Old map share image**, a static redirect, 301, to `https://sentryatlas.com/og-image.png`:

   ```
   (http.host eq "map.sentryatlas.com" and http.request.uri.path eq "/og-image.png")
   ```

2. **Old map to the new map**, a static redirect, 302 (temporary at first), to `https://sentryatlas.com/map/`, query string not kept:

   ```
   (http.host eq "map.sentryatlas.com" and not starts_with(http.request.uri.path, "/api/") and not starts_with(http.request.uri.path, "/health") and http.request.uri.path ne "/og-image.png")
   ```

`/api/` and `/health` keep reaching the API: the public v1 URL, the health check (`.github/workflows/health.yml`) and the new site all use them.

Optional: `www.sentryatlas.com`, which answers 404 today, as a 301 to `https://sentryatlas.com` with the path kept.

## 4. Check

```
curl -s -o /dev/null -w "%{http_code} %{redirect_url}\n" https://map.sentryatlas.com/
curl -s -o /dev/null -w "%{http_code} %{redirect_url}\n" https://map.sentryatlas.com/og-image.png
curl -s -o /dev/null -w "%{http_code}\n" "https://map.sentryatlas.com/api/v1/events?limit=1"
curl -s -o /dev/null -w "%{http_code}\n" https://map.sentryatlas.com/health/stale
```

Expect `302 https://sentryatlas.com/map/`, `301 https://sentryatlas.com/og-image.png`, `200`, `200`. Then, in a browser on sentryatlas.com: the map draws, an event opens, and the waiting-list form sends (the API accepts it from this origin only).

Then remove the rehearsal's `next.sentryatlas.com` custom domain, and merge the pull request that drops `X-Robots-Tag: noindex` from `web/public/_headers`.

## The way back

1. The Worker: remove the custom domain `sentryatlas.com`.
2. DNS: recreate the `sentryatlas.com` record with the saved target, proxied.
3. Rules: turn off both redirect rules.

The old apps keep running until step 5, so this restores the site as it was.

## 5. Remove the old apps (days later, once stable)

In this order, because DigitalOcean builds a component from its folder on every push:

1. DigitalOcean: delete the app `sentryatlas-landing`; in the app `sentryatlas`, delete the `frontend` component and its `/` route, keeping `backend` with `/api` and `/health`.
2. Then merge the pull request that removes `frontend/`, `landing/`, their CI jobs and `.do/landing.yaml`, and the `frontend` component from `.do/app.yaml`.
3. Make the map host's redirect a 301 once nothing has needed the way back for a week.
