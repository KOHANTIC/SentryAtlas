import { describe, expect, it } from "vitest";
import type { Snapshot } from "./api.gen";
import { createSnapshotStore, type Deps } from "./snapshot";

const snap = (generated: string): Snapshot => ({ generated, window: "7d", sources: [], events: [] });

/** A fake browser: manual timers, a visibility switch, and scripted answers. */
function fakeBrowser(answers: (() => Response)[]) {
  let now = 0;
  let hidden = false;
  let timers: { at: number; fn: () => void; id: number }[] = [];
  let nextId = 0;
  let visible: (() => void) | undefined;
  const calls: RequestInit[] = [];
  const deps: Deps = {
    fetch: async (_url, init) => {
      calls.push(init ?? {});
      const next = answers.shift();
      if (!next) throw new Error("no answer scripted");
      return next();
    },
    now: () => now,
    setTimeout: (fn, ms) => {
      timers.push({ at: now + ms, fn, id: ++nextId });
      return nextId;
    },
    clearTimeout: (id) => {
      timers = timers.filter((t) => t.id !== id);
    },
    hidden: () => hidden,
    onVisible: (fn) => {
      visible = fn;
      return () => (visible = undefined);
    },
  };
  const settle = () => new Promise((r) => setTimeout(r, 0));
  return {
    deps,
    calls,
    async advance(ms: number) {
      now += ms;
      for (const t of timers.filter((t) => t.at <= now)) {
        timers = timers.filter((x) => x !== t);
        t.fn();
        await settle();
      }
    },
    async show() {
      hidden = false;
      visible?.();
      await settle();
    },
    hide: () => (hidden = true),
    settle,
    pending: () => timers.length,
  };
}

const ok = (s: Snapshot, etag: string) => () => new Response(JSON.stringify(s), { headers: { ETag: etag } });

describe("createSnapshotStore", () => {
  it("loads, then revalidates every interval, skipping an unchanged snapshot", async () => {
    const b = fakeBrowser([ok(snap("a"), 'W/"1"'), ok(snap("b"), 'W/"1"'), ok(snap("c"), 'W/"2"')]);
    const store = createSnapshotStore("u", 60_000, b.deps);
    expect(store.get().status).toBe("loading");
    const stop = store.subscribe(() => {});
    await b.settle();
    expect(store.get()).toMatchObject({ status: "ready", snapshot: { generated: "a" }, checkedAt: 0 });
    expect(b.calls[0].cache).toBe("no-cache");

    await b.advance(60_000); // same ETag: the copy already held stays
    expect(store.get()).toMatchObject({ snapshot: { generated: "a" }, checkedAt: 60_000 });

    await b.advance(60_000); // new ETag: the new snapshot replaces it
    expect(store.get().snapshot?.generated).toBe("c");
    stop();
    expect(b.pending()).toBe(0);
  });

  it("keeps the last snapshot on a failure and backs off", async () => {
    const fail = () => new Response("", { status: 503 });
    const b = fakeBrowser([ok(snap("a"), "1"), fail, fail, ok(snap("b"), "2")]);
    const store = createSnapshotStore("u", 60_000, b.deps);
    store.subscribe(() => {});
    await b.settle();
    await b.advance(60_000);
    expect(store.get()).toMatchObject({ status: "ready", snapshot: { generated: "a" }, error: "the API answered 503" });
    await b.advance(60_000); // the retry after one failure comes one interval later
    expect(b.calls).toHaveLength(3);
    await b.advance(60_000); // after two, two intervals
    expect(b.calls).toHaveLength(3);
    await b.advance(60_000);
    expect(store.get().snapshot?.generated).toBe("b");
    expect(store.get().error).toBeUndefined();
  });

  it("says error when the first load fails", async () => {
    const b = fakeBrowser([() => { throw new TypeError("network down"); }]);
    const store = createSnapshotStore("u", 60_000, b.deps);
    store.subscribe(() => {});
    await b.settle();
    expect(store.get()).toMatchObject({ status: "error", error: "network down" });
  });

  it("pauses in a background tab and catches up when shown", async () => {
    const b = fakeBrowser([ok(snap("a"), "1"), ok(snap("b"), "2")]);
    const store = createSnapshotStore("u", 60_000, b.deps);
    store.subscribe(() => {});
    await b.settle();
    b.hide();
    await b.advance(10 * 60_000);
    expect(b.calls).toHaveLength(1);
    await b.show();
    expect(b.calls).toHaveLength(2);
    expect(store.get().snapshot?.generated).toBe("b");
  });

  it("does not check on show when the last check is recent", async () => {
    const b = fakeBrowser([ok(snap("a"), "1")]);
    const store = createSnapshotStore("u", 60_000, b.deps);
    store.subscribe(() => {});
    await b.settle();
    await b.show();
    expect(b.calls).toHaveLength(1);
  });
});
