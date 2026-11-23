"use client";

import { useState, useSyncExternalStore } from "react";
import Link from "next/link";
import { API_BASE } from "@/lib/snapshot";

const CONFIRM_API = `${API_BASE}/api/v2/waitlist/confirm`;

type State = "ready" | "sending" | "confirmed" | "expired" | "invalid" | "failed";

function onHashChange(cb: () => void) {
  window.addEventListener("hashchange", cb);
  return () => window.removeEventListener("hashchange", cb);
}

/** Reads the token after "#" and confirms it only when the person presses the button. */
export default function ConfirmButton() {
  // The token lives in the fragment, which the browser never sends to a server.
  // The static build has no fragment, so it renders nothing until the browser reads it.
  const token = useSyncExternalStore(onHashChange, () => window.location.hash.slice(1), () => null);
  const [result, setState] = useState<State>("ready");
  const state: State | "loading" = result !== "ready" ? result : token === null ? "loading" : token ? "ready" : "invalid";

  async function confirm() {
    setState("sending");
    try {
      const res = await fetch(CONFIRM_API, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ token }),
      });
      const next: State = res.status === 200 ? "confirmed" : res.status === 410 ? "expired" : res.status === 400 ? "invalid" : "failed";
      if (next === "confirmed") {
        // Drop the token from the address bar and history.
        window.history.replaceState(null, "", window.location.pathname);
      }
      setState(next);
    } catch {
      setState("failed");
    }
  }

  const again = (
    <Link href="/alerts/">
      Join again
    </Link>
  );

  switch (state) {
    case "loading":
      return null;
    case "confirmed":
      return (
        <p role="status">
          You&apos;re on the waiting list. We&apos;ll email you when alerts are ready.
        </p>
      );
    case "expired":
      return <p role="alert">This link has expired. {again}.</p>;
    case "invalid":
      return <p role="alert">This link isn&apos;t valid. {again}.</p>;
    default:
      return (
        <div>
          {state === "failed" && (
            <p role="alert" className="error">That didn&apos;t work. Try again in a few minutes.</p>
          )}
          <button
            className="primary"
            type="button"
            onClick={confirm}
            disabled={state === "sending"}
           
          >
            {state === "sending" ? "Confirming…" : "Confirm"}
          </button>
        </div>
      );
  }
}
