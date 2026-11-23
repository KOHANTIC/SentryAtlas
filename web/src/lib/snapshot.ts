import type { Snapshot } from "./api.gen";

/** The API the site reads; a fork or a local run sets NEXT_PUBLIC_API_BASE. */
export const API_BASE = process.env.NEXT_PUBLIC_API_BASE ?? "https://map.sentryatlas.com";

export type SnapshotState = {
  status: "loading" | "ready" | "error";
  snapshot?: Snapshot;
  /** When the API last answered (ms since the epoch). */
  checkedAt?: number;
  /** Why the last check failed; the last snapshot, if any, is kept. */
  error?: string;
};

/** What the store needs from the browser, replaced in tests. */
export type Deps = {
  fetch: typeof fetch;
  now(): number;
  setTimeout(fn: () => void, ms: number): unknown;
  clearTimeout(handle: unknown): void;
  hidden(): boolean;
  onVisible(fn: () => void): () => void;
};

function browserDeps(): Deps {
  return {
    fetch: (...a) => fetch(...a),
    now: () => Date.now(),
    setTimeout: (fn, ms) => setTimeout(fn, ms),
    clearTimeout: (h) => clearTimeout(h as ReturnType<typeof setTimeout>),
    hidden: () => document.visibilityState === "hidden",
    onVisible: (fn) => {
      const h = () => document.visibilityState === "visible" && fn();
      document.addEventListener("visibilitychange", h);
      return () => document.removeEventListener("visibilitychange", h);
    },
  };
}

export const LOADING: SnapshotState = { status: "loading" };
const MAX_BACKOFF = 5 * 60_000;

/**
 * Keeps one snapshot fresh. It checks every `every` ms while anyone listens and
 * the tab is visible. Each check revalidates the browser's cached copy, so an
 * unchanged snapshot costs a 304 from the edge, and an unchanged ETag skips
 * parsing. A failed check keeps the last snapshot and retries later, up to 5 minutes apart.
 */
export function createSnapshotStore(url: string, every = 60_000, deps: Deps = browserDeps()) {
  let state = LOADING;
  let etag: string | null = null;
  let timer: unknown;
  let failures = 0;
  let inFlight = false;
  let stopVisible: (() => void) | undefined;
  const listeners = new Set<() => void>();

  const set = (next: SnapshotState) => {
    state = next;
    listeners.forEach((l) => l());
  };

  const schedule = (ms: number) => {
    deps.clearTimeout(timer);
    timer = deps.setTimeout(check, ms);
  };

  async function check() {
    if (deps.hidden() || inFlight) return; // the visible event resumes it
    inFlight = true;
    try {
      const res = await deps.fetch(url, { cache: "no-cache" });
      if (!res.ok) throw new Error(`the API answered ${res.status}`);
      const tag = res.headers.get("ETag");
      if (tag && tag === etag && state.snapshot) {
        await res.body?.cancel();
        set({ ...state, status: "ready", checkedAt: deps.now(), error: undefined });
      } else {
        const snapshot = (await res.json()) as Snapshot;
        etag = tag;
        set({ status: "ready", snapshot, checkedAt: deps.now() });
      }
      failures = 0;
    } catch (err) {
      failures++;
      set({ ...state, status: state.snapshot ? "ready" : "error", error: err instanceof Error ? err.message : String(err) });
    } finally {
      inFlight = false;
    }
    if (listeners.size > 0) schedule(failures ? Math.min(every * 2 ** (failures - 1), MAX_BACKOFF) : every);
  }

  function start() {
    stopVisible = deps.onVisible(() => {
      if (state.checkedAt === undefined || deps.now() - state.checkedAt >= every) void check();
    });
    void check();
  }

  function stop() {
    deps.clearTimeout(timer);
    stopVisible?.();
  }

  return {
    get: () => state,
    subscribe(fn: () => void) {
      listeners.add(fn);
      if (listeners.size === 1) start();
      return () => {
        listeners.delete(fn);
        if (listeners.size === 0) stop();
      };
    },
  };
}

export type SnapshotStore = ReturnType<typeof createSnapshotStore>;

/** The snapshot URL for a window. */
export const snapshotURL = (window: Snapshot["window"]) => `${API_BASE}/api/v2/snapshot?window=${window}`;
