"use client";

import { useMemo, useSyncExternalStore } from "react";
import { filterEvents, freshness, PRESETS } from "./filters";
import { parse, subscribeURL } from "./urlState";
import { useSnapshot } from "./useSnapshot";

/**
 * The link's view, and the events it asks for: the snapshot for its window, cut to its
 * types and time span. The map and the embed share it, so one link shows the same events in both.
 */
export function useLinkedEvents() {
  // The link is the state: the view, the filters and the selected event all live in it.
  const search = useSyncExternalStore(subscribeURL, () => window.location.search, () => "");
  const view = useMemo(() => parse(search), [search]);
  const preset = PRESETS[view.time];
  const state = useSnapshot(preset.window);
  const { snapshot, checkedAt } = state;
  const events = useMemo(
    () =>
      filterEvents(snapshot?.events ?? [], {
        types: view.types && new Set(view.types),
        since: checkedAt === undefined ? undefined : checkedAt - (view.hours ? view.hours * 3_600_000 : preset.ms),
      }),
    [snapshot, checkedAt, view.types, view.hours, preset.ms],
  );
  const fresh = snapshot && checkedAt ? freshness(snapshot, checkedAt) : undefined;
  return { view, state, events, fresh };
}
