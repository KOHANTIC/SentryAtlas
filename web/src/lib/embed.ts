import { serialize, type ViewState } from "./urlState";

// The page's address goes inside an HTML attribute, where & starts a character reference.
const attr = (s: string) => s.replaceAll("&", "&amp;").replaceAll('"', "&quot;");

/** The code another site pastes to show the map: this view, its types and its window, never a selected event. */
export function embedCode(origin: string, view: ViewState): string {
  const src = `${origin}/embed/${serialize({ ...view, event: undefined })}`;
  return `<iframe src="${attr(src)}" title="SentryAtlas map of current events" width="100%" height="450" style="border:0" loading="lazy"></iframe>`;
}
