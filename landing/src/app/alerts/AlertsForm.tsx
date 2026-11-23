"use client";

import { useState } from "react";
import Link from "next/link";
import { WAITLIST_API } from "@/lib/waitlist";

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
      <div role="status" className="border border-border bg-surface-raised p-6 space-y-2">
        <p className="text-foreground font-semibold">Check your inbox.</p>
        <p>
          We sent a link to {state.email}. Open it within 48 hours to confirm.
        </p>
      </div>
    );
  }

  const emailError = state.kind === "error" && state.field === "email";
  const input = "w-full bg-surface-raised border border-border-strong px-3 py-2.5 text-foreground focus:outline-none focus-visible:ring-2 focus-visible:ring-accent-400";

  return (
    <form onSubmit={submit} noValidate className="space-y-6">
      <div className="space-y-2">
        <label htmlFor="email" className="block text-foreground font-medium">Email</label>
        <input
          id="email" name="email" type="email" autoComplete="email" required
          aria-invalid={emailError || undefined}
          aria-describedby={emailError ? "form-error" : undefined}
          className={input}
        />
      </div>

      <div className="space-y-2">
        <label htmlFor="area" className="block text-foreground font-medium">
          Area <span className="text-foreground-faint font-normal">(optional)</span>
        </label>
        <input id="area" name="area" type="text" maxLength={100} placeholder="A city, region or country" className={input} />
      </div>

      <fieldset className="space-y-3">
        <legend className="text-foreground font-medium">
          Types of event <span className="text-foreground-faint font-normal">(optional; none means all)</span>
        </legend>
        <div className="grid grid-cols-2 sm:grid-cols-3 gap-x-4 gap-y-2">
          {TYPES.map(([value, label]) => (
            <label key={value} className="flex items-center gap-2 min-h-6 cursor-pointer">
              <input type="checkbox" name="hazards" value={value} className="w-4 h-4 accent-accent-400" />
              {label}
            </label>
          ))}
        </div>
      </fieldset>

      <label className="flex items-start gap-3 cursor-pointer">
        <input type="checkbox" name="news" className="mt-1 w-4 h-4 accent-accent-400" />
        <span>Also send me occasional SentryAtlas product news.</span>
      </label>

      {state.kind === "error" && (
        <p id="form-error" role="alert" className="text-error">{state.message}</p>
      )}

      <div className="space-y-3">
        <button
          type="submit"
          disabled={state.kind === "sending"}
          className="text-base font-semibold bg-accent-400 text-primary-fg px-7 py-3.5 hover:bg-accent-500 disabled:opacity-60 cursor-pointer focus:outline-none focus-visible:ring-2 focus-visible:ring-offset-2 focus-visible:ring-offset-surface focus-visible:ring-accent-400"
        >
          {state.kind === "sending" ? "Sending…" : "Join the waiting list"}
        </button>
        <p className="text-sm text-foreground-faint">
          We&apos;ll email you a link to confirm.{" "}
          <Link href="/privacy/" className="underline underline-offset-2 hover:no-underline">Privacy</Link>
        </p>
      </div>
    </form>
  );
}
