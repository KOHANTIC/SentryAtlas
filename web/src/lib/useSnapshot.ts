"use client";

import { useSyncExternalStore } from "react";
import type { Snapshot } from "./api.gen";
import { createSnapshotStore, LOADING, snapshotURL, type SnapshotStore } from "./snapshot";

// One store per window for the whole page, so every component shares one fetch.
const stores = new Map<Snapshot["window"], SnapshotStore>();

function storeFor(window: Snapshot["window"]): SnapshotStore {
  let s = stores.get(window);
  if (!s) {
    s = createSnapshotStore(snapshotURL(window));
    stores.set(window, s);
  }
  return s;
}

/** The live snapshot for a window, kept fresh while the component is mounted. */
export function useSnapshot(window: Snapshot["window"]) {
  const store = storeFor(window);
  return useSyncExternalStore(store.subscribe, store.get, () => LOADING);
}
