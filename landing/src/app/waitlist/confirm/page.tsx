import type { Metadata } from "next";
import SimplePage from "@/components/SimplePage";
import ConfirmButton from "./ConfirmButton";

export const metadata: Metadata = {
  title: "Confirm · SentryAtlas",
  robots: { index: false },
};

/** Where a confirmation email's link lands. */
export default function Confirm() {
  return (
    <SimplePage>
      <h1 className="font-display text-4xl sm:text-5xl font-semibold text-foreground">
        Confirm your place on the waiting list
      </h1>
      <ConfirmButton />
    </SimplePage>
  );
}
