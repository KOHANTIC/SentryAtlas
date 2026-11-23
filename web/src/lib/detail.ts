"use client";

import { useEffect, useState } from "react";
import type { Event, SourceStatus } from "./api.gen";
import { API_BASE } from "./snapshot";

export type Detail =
  | { status: "loading" }
  | { status: "ok"; event: Event; source: SourceStatus }
  | { status: "ended" }
  | { status: "error" };

/** One event in full, with its long text. A 404 means it has left its source's feed. */
export async function fetchDetail(id: string, f: typeof fetch = fetch, signal?: AbortSignal): Promise<Detail> {
  try {
    const res = await f(`${API_BASE}/api/v2/events/${encodeURIComponent(id)}`, { signal });
    if (res.status === 404) return { status: "ended" };
    if (!res.ok) return { status: "error" };
    const body = (await res.json()) as { event: Event; source: SourceStatus };
    return { status: "ok", ...body };
  } catch (err) {
    if (signal?.aborted) throw err;
    return { status: "error" };
  }
}

/** The selected event's detail, fetched when the selection changes. */
export function useDetail(id: string | undefined): Detail | undefined {
  const [state, setState] = useState<{ id?: string; detail?: Detail }>({});
  useEffect(() => {
    if (!id) return;
    const ctl = new AbortController();
    fetchDetail(id, fetch, ctl.signal).then(
      (detail) => setState({ id, detail }),
      () => {},
    );
    return () => ctl.abort();
  }, [id]);
  if (!id) return undefined;
  return state.id === id ? state.detail : { status: "loading" };
}
