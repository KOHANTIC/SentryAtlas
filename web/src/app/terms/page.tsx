import type { Metadata } from "next";
import { TextPage } from "@/components/Site";
import { OPERATOR, PRIVACY_EMAIL } from "@/lib/operator";

export const metadata: Metadata = {
  title: "Terms",
  description: "The terms of using SentryAtlas, which is not an official warning service.",
  alternates: { canonical: "https://sentryatlas.com/terms/" },
};

const HISTORY = "https://github.com/KOHANTIC/SentryAtlas/commits/main/web/src/app/terms/page.tsx";

/** The terms of use. */
export default function Terms() {
  const who = OPERATOR ?? "whoever runs this copy";
  return (
    <TextPage label="Terms" title="Terms of use">
      <p className="lede">SentryAtlas shows what public agencies report about natural hazards. Using it means accepting these terms.</p>

      <h2>Not an official warning service</h2>
      <p>
        SentryAtlas is not an official warning service and does not replace one. Do not rely on it to keep you or anyone
        else safe. For warnings where you are, follow your local authorities and the agencies that issue them.
      </p>

      <h2>What you see</h2>
      <p>
        Every event comes from USGS, NASA EONET, the National Weather Service or GDACS, as they report it. Warning text is
        shown word for word. An event can be late, incomplete, out of date or wrong, and a source can stop answering.
        SentryAtlas is provided as it is, without any warranty, and it may change or stop at any time.
      </p>

      <h2>Responsibility</h2>
      <p>
        As far as the law allows, {who} is not liable for any loss or harm that comes from using SentryAtlas or relying on
        what it shows.
      </p>

      <h2>The API</h2>
      <p>
        The API is free to use within its rate limit. Do not try to get around the limit or overload the service. If you
        show its data, credit the agencies it comes from.
      </p>

      <h2>Embedding the map</h2>
      <p>You may show the map on your own site with the code from Embed on the map. Keep its credit bar visible.</p>

      <h2>The code</h2>
      <p>
        SentryAtlas is open source under the{" "}
        <a href="https://github.com/KOHANTIC/SentryAtlas/blob/main/LICENSE">GNU Affero General Public License 3.0</a>.
      </p>

      <h2>Contact</h2>
      {OPERATOR ? (
        <p>
          SentryAtlas at sentryatlas.com is run by {OPERATOR}.{PRIVACY_EMAIL && <> Write to <a href={`mailto:${PRIVACY_EMAIL}`}>{PRIVACY_EMAIL}</a>.</>}
        </p>
      ) : (
        <p>This copy of SentryAtlas does not name its operator.</p>
      )}

      <h2>Changes</h2>
      <p>
        These terms live in the project&apos;s public repository, so <a href={HISTORY}>every change to them is on record</a>.
      </p>
    </TextPage>
  );
}
