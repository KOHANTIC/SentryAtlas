"use client";

import { useState } from "react";
import Link from "next/link";
import { API_BASE } from "@/lib/snapshot";

const WAITLIST_API = `${API_BASE}/api/v2/waitlist`;

const TYPES: [string, string][] = [
  ["earthquake", "Earthquake"], ["wildfire", "Wildfire"], ["volcano", "Volcano"],
  ["storm", "Storm"], ["flood", "Flood"], ["cyclone", "Cyclone"],
  ["tornado", "Tornado"], ["hurricane", "Hurricane"], ["winter_storm", "Winter storm"],
  ["tsunami", "Tsunami"], ["drought", "Drought"], ["iceberg", "Iceberg"],
  ["landslide", "Landslide"], ["weather", "Weather"], ["other", "Other"],
];

type State =
  | { kind: "idle" }
  | { kind: "sending" }
  | { kind: "sent"; email: string }
  | { kind: "error"; field?: "email"; message: string };

/** What a failed signup tells the person, by the API's status and error. */
function problem(status: number, error: string): { field?: "email"; message: string } {
  if (status === 400 && error.startsWith("email")) return { field: "email", message: "That doesn't look like an email address." };
  if (status === 400 && error.includes("domain")) return { field: "email", message: "That address can't receive email." };
  if (status === 429) return { message: "Too many tries from here. Try again in an hour." };
  if (status === 503 && error.includes("today")) return { message: "The waiting list is full for today. Try again tomorrow." };
  return { message: "That didn't work. Try again in a few minutes." };
}

/** The waiting-list form, with its sending, sent and error states. */
export default function AlertsForm() {
  const [state, setState] = useState<State>({ kind: "idle" });

  async function submit(e: React.FormEvent<HTMLFormElement>) {
    e.preventDefault();
    const form = new FormData(e.currentTarget);
    const email = String(form.get("email") ?? "").trim();
    setState({ kind: "sending" });
    try {
      const res = await fetch(WAITLIST_API, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({
          email,
          area: String(form.get("area") ?? "").trim(),
          hazards: form.getAll("hazards").map(String),
          news: form.get("news") === "on",
        }),
      });
      if (res.status === 202) {
        setState({ kind: "sent", email });
        return;
      }
      const body = await res.json().catch(() => ({}));
      setState({ kind: "error", ...problem(res.status, String(body.error ?? "")) });
    } catch {
      setState({ kind: "error", message: "That didn't work. Check your connection and try again." });
    }
  }

  if (state.kind === "sent") {
    return (
      <div role="status" className="sent">
        <p>Check your inbox.</p>
        <p>
          We sent a link to {state.email}. Open it within 48 hours to confirm.
        </p>
      </div>
    );
  }

  const emailError = state.kind === "error" && state.field === "email";

  return (
    <form onSubmit={submit} noValidate className="form">
      <div>
        <label htmlFor="email">Email</label>
        <input
          id="email" name="email" type="email" autoComplete="email" required
          aria-invalid={emailError || undefined}
          aria-describedby={emailError ? "form-error" : undefined}
         
        />
      </div>

      <div>
        <label htmlFor="area">
          Area <span className="muted">(optional)</span>
        </label>
        <input id="area" name="area" type="text" maxLength={100} placeholder="A city, region or country" />
      </div>

      <fieldset className="types">
        <legend>
          Types of event <span className="muted">(optional; none means all)</span>
        </legend>
        <div>
          {TYPES.map(([value, label]) => (
            <label key={value}>
              <input type="checkbox" name="hazards" value={value} />
              {label}
            </label>
          ))}
        </div>
      </fieldset>

      <label>
        <input type="checkbox" name="news" />
        <span>Also send me occasional SentryAtlas product news.</span>
      </label>

      {state.kind === "error" && (
        <p id="form-error" role="alert" className="error">{state.message}</p>
      )}

      <div>
        <button
          className="primary"
          type="submit"
          disabled={state.kind === "sending"}
         
        >
          {state.kind === "sending" ? "Sending…" : "Join the waiting list"}
        </button>
        <p className="muted">
          We&apos;ll email you a link to confirm.{" "}
          <Link href="/privacy/">Privacy</Link>
        </p>
      </div>
    </form>
  );
}
