import type { Metadata } from "next";
import AlertsForm from "@/components/AlertsForm";
import { TextPage } from "@/components/Site";

export const metadata: Metadata = {
  title: "Get alerts for your area",
  description: "Join the waiting list for SentryAtlas location alerts.",
  alternates: { canonical: "https://sentryatlas.com/alerts/" },
};

/** The waiting list's signup page, in the wording approved on 2026-11-23. */
export default function Alerts() {
  return (
    <TextPage label="Alerts" title="Get alerts for your area">
      <p className="lede">Location alerts aren&apos;t ready yet. Join the waiting list and we&apos;ll email you when they are.</p>
      <p className="muted">
        SentryAtlas is not an official warning service. For warnings where you are, follow your local authorities.
      </p>
      <AlertsForm />
    </TextPage>
  );
}
