"use client";

import { useEffect, useId, useState } from "react";
import { countryName, searchPlaces, type Place } from "@/lib/places";

/**
 * Type a city's name and choose one to go there. A combobox: Down and Up move
 * through the suggestions, Enter chooses (the first if none is highlighted),
 * Escape closes. The names come from this site's own files; nothing typed
 * leaves the browser. GeoNames is credited in the map's credit line.
 */
export default function PlaceSearch({ onChoose }: { onChoose: (at: [number, number], name: string) => void }) {
  const id = useId();
  const [query, setQuery] = useState("");
  const [found, setFound] = useState<{ q: string; places: Place[] }>({ q: "", places: [] });
  // -1: nothing highlighted until Down is pressed; Enter then takes the first.
  const [active, setActive] = useState(-1);
  const [open, setOpen] = useState(false);

  useEffect(() => {
    let current = true;
    searchPlaces(query).then((places) => current && setFound({ q: query, places }));
    return () => {
      current = false;
    };
  }, [query]);

  const places = found.q === query ? found.places : [];
  const shown = open && places.length > 0;

  function choose(p: Place) {
    setOpen(false);
    setQuery(p[0]);
    onChoose([p[3], p[4]], p[0]);
  }

  function key(k: React.KeyboardEvent) {
    if (k.key === "ArrowDown" && places.length) {
      k.preventDefault();
      setOpen(true);
      setActive((a) => (shown ? Math.min(a + 1, places.length - 1) : 0));
    } else if (k.key === "ArrowUp" && shown) {
      k.preventDefault();
      setActive((a) => Math.max(a - 1, 0));
    } else if (k.key === "Enter" && shown) {
      k.preventDefault();
      choose(places[Math.min(Math.max(active, 0), places.length - 1)]);
    } else if (k.key === "Escape") {
      setOpen(false);
    }
  }

  return (
    <div className="place-search">
      <label htmlFor={`${id}-input`} className="label">Go to a place</label>
      <input
        id={`${id}-input`}
        type="search"
        role="combobox"
        aria-expanded={shown}
        aria-controls={`${id}-list`}
        aria-autocomplete="list"
        aria-activedescendant={shown && active >= 0 ? `${id}-${active}` : undefined}
        autoComplete="off"
        placeholder="A city"
        value={query}
        onChange={(e) => {
          setQuery(e.target.value);
          setActive(-1);
          setOpen(true);
        }}
        onKeyDown={key}
        onBlur={() => setOpen(false)}
      />
      <ul id={`${id}-list`} role="listbox" aria-label="Places" hidden={!shown}>
        {places.map((p, i) => (
          <li
            key={`${p[1]}-${p[2]}-${p[3]}`}
            id={`${id}-${i}`}
            role="option"
            aria-selected={i === active}
            onMouseDown={(e) => e.preventDefault()}
            onClick={() => choose(p)}
          >
            {p[0]} <span className="muted">{countryName(p[2])}</span>
          </li>
        ))}
      </ul>
    </div>
  );
}
