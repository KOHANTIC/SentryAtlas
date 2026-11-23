// Shared by scripts/places.mjs, which writes the place files, and the search box,
// which reads them: both must cut a name the same way.

/** A name folded for matching: lowercase, accents dropped, only letters and digits kept. */
export function normalize(s) {
  return s.normalize("NFD").replace(/[̀-ͯ]/g, "").toLowerCase().replace(/[^a-z0-9]/g, "");
}

/** The file a name is listed in: the first two letters of its folded form. */
export function prefixOf(s) {
  const n = normalize(s);
  return n.length >= 2 ? n.slice(0, 2) : "";
}
