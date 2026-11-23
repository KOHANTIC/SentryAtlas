// Serves ./out the way Workers static assets does (a folder answers with its
// index.html, a missing path with 404.html), for the end-to-end checks only.
import { createServer } from "node:http";
import { readFile, stat } from "node:fs/promises";
import path from "node:path";

const root = path.resolve("out");
const port = Number(process.env.PORT ?? 4173);
const types = { ".html": "text/html; charset=utf-8", ".js": "text/javascript", ".mjs": "text/javascript", ".css": "text/css", ".svg": "image/svg+xml", ".json": "application/json", ".txt": "text/plain", ".woff2": "font/woff2", ".ico": "image/x-icon", ".png": "image/png" };

async function file(urlPath) {
  const p = path.join(root, decodeURIComponent(urlPath));
  if (!p.startsWith(root)) return null;
  try {
    const s = await stat(p);
    return s.isDirectory() ? path.join(p, "index.html") : p;
  } catch {
    return null;
  }
}

createServer(async (req, res) => {
  const p = await file(new URL(req.url, "http://x").pathname);
  try {
    const body = await readFile(p ?? "");
    res.writeHead(200, { "Content-Type": types[path.extname(p)] ?? "application/octet-stream" });
    res.end(body);
  } catch {
    res.writeHead(404, { "Content-Type": types[".html"] });
    res.end(await readFile(path.join(root, "404.html")).catch(() => "Not found"));
  }
}).listen(port, () => console.log(`serving ${root} on ${port}`));
