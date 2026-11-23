import type { Metadata } from "next";
import SimplePage from "@/components/SimplePage";
import AlertsForm from "./AlertsForm";

export const metadata: Metadata = {
  title: "Get alerts for your area · SentryAtlas",
  description: "Join the waiting list for SentryAtlas location alerts.",
  alternates: { canonical: "https://sentryatlas.com/alerts/" },
};

/** The waiting list's signup page. */
export default function Alerts() {
  return (
    <SimplePage>
      <header className="space-y-4">
        <h1 className="font-display text-4xl sm:text-5xl font-semibold text-foreground">
          Get alerts for your area
        </h1>
        <p className="text-lg">
          Location alerts aren&apos;t ready yet. Join the waiting list and we&apos;ll email you
          when they are.
        </p>
        <p className="text-sm text-foreground-faint">
          SentryAtlas is not an official warning service. For warnings where you are, follow your
          local authorities.
        </p>
      </header>
      <AlertsForm />
    </SimplePage>
  );
}
