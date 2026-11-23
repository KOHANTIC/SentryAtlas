import type { Metadata } from "next";
import { TextPage } from "@/components/Site";
import ConfirmButton from "./ConfirmButton";

export const metadata: Metadata = { title: "Confirm", robots: { index: false } };

/** Where a confirmation email's link lands. The URL is in emails already sent: keep it. */
export default function Confirm() {
  return (
    <TextPage label="Waiting list" title="Confirm your place on the waiting list">
      <ConfirmButton />
    </TextPage>
  );
}
