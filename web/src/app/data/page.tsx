import type { Metadata } from "next";
import { TextPage } from "@/components/Site";

export const metadata: Metadata = {
  title: "Sources",
  description: "Where SentryAtlas's events come from: USGS, NASA EONET, the National Weather Service and GDACS.",
  alternates: { canonical: "https://sentryatlas.com/data/" },
};

const SOURCES = [
  {
    name: "USGS",
    full: "U.S. Geological Survey, Earthquake Hazards Program",
    url: "https://earthquake.usgs.gov",
    what: "Earthquakes worldwide, and quarry blasts and explosions, the past 30 days.",
    every: "5 minutes",
  },
  {
    name: "NWS",
    full: "National Weather Service",
    url: "https://www.weather.gov",
    what: "US weather alerts in force now. Most name forecast zones rather than draw a shape; the map draws those zones from NWS's own zone files.",
    every: "2 minutes",
  },
  {
    name: "GDACS",
    full: "Global Disaster Alert and Coordination System",
    url: "https://www.gdacs.org",
    what: "Earthquakes, wildfires, floods, droughts, tropical cyclones and volcanoes worldwide, the past 30 days.",
    every: "10 minutes",
  },
  {
    name: "NASA EONET",
    full: "NASA Earth Observatory Natural Event Tracker",
    url: "https://eonet.gsfc.nasa.gov",
    what: "Wildfires, storms and icebergs with an observation in the past 30 days.",
    every: "10 minutes",
  },
];

/** Where the events come from, and who to credit. */
export default function Data() {
  return (
    <TextPage label="Sources" title="Where the events come from">
      <p className="lede">
        Every event on SentryAtlas comes from one of four public services, shown as they report it. When two of them report
        the same event, the map draws it once and lists the other report beside it.
      </p>
      <dl className="sources">
        {SOURCES.map((s) => (
          <div key={s.name}>
            <dt>
              <a href={s.url}>{s.name}</a> <span className="muted">{s.full}</span>
            </dt>
            <dd>
              {s.what} <span className="muted">Checked every {s.every}.</span>
            </dd>
          </div>
        ))}
      </dl>
      <h2>Map layers</h2>
      <p>
        Radar: NOAA nowCOAST&apos;s base reflectivity mosaics for the US states and territories, updated every few minutes.
        Satellite: NASA&apos;s VIIRS true-colour imagery from NOAA-20 for the latest full day.
      </p>
      <h2>Credit</h2>
      <p>
        Event data: USGS, NASA EONET, the National Weather Service and GDACS. NWS zone outlines: the National Weather
        Service. Basemap: <a href="https://openfreemap.org">OpenFreeMap</a>, © <a href="https://openmaptiles.org">OpenMapTiles</a>,
        data from <a href="https://www.openstreetmap.org/copyright">OpenStreetMap</a>. Place names:{" "}
        <a href="https://www.geonames.org/">GeoNames</a>, under <a href="https://creativecommons.org/licenses/by/4.0/">CC BY 4.0</a>.
        Radar: <a href="https://nowcoast.noaa.gov/">NOAA nowCOAST</a>. Satellite imagery: NASA&apos;s{" "}
        <a href="https://earthdata.nasa.gov/gibs">Global Imagery Browse Services (GIBS)</a>, part of NASA&apos;s Earth
        Science Data and Information System (ESDIS).
      </p>
      <h2>When a source is late</h2>
      <p>
        If a source stops answering, the map keeps its last update and says how long it has been silent. SentryAtlas is
        not an official warning service: for warnings where you are, follow your local authorities.
      </p>
    </TextPage>
  );
}
