/** The imagery a link can turn on under the events, in the order a link writes them. */
export const LAYERS = ["radar", "satellite"] as const;
export type Layer = (typeof LAYERS)[number];

const MINUTE = 60_000;
const HOUR = 60 * MINUTE;

// NOAA nowCOAST's radar mosaics for the US states and territories, asked as one picture per tile.
const RADAR = "https://nowcoast.noaa.gov/geoserver/observations/weather_radar/ows";
const MOSAICS = [
  "base_reflectivity_mosaic",
  "alaska_base_reflectivity_mosaic",
  "hawaii_base_reflectivity_mosaic",
  "caribbean_base_reflectivity_mosaic",
  "guam_base_reflectivity_mosaic",
];

/** Which five minutes `now` falls in: a new value asks nowCOAST for its newest radar again. */
export const radarStamp = (now: number) => Math.floor(now / (5 * MINUTE));

/** The radar tile template for MapLibre; `_` changes with the stamp so no cache holds an old picture. */
export function radarTiles(stamp: number): string {
  const q = `service=WMS&version=1.3.0&request=GetMap&layers=${MOSAICS.join(",")}&styles=&format=image/png&transparent=true&crs=EPSG:3857&bbox={bbox-epsg-3857}&width=256&height=256&_=${stamp}`;
  return `${RADAR}?${q}`;
}

/**
 * The latest day NASA's daily imagery has finished: the UTC date 30 hours ago. The newest
 * day is still being filled in by later passes, so this is yesterday from 06:00 UTC.
 */
export const satelliteDay = (now: number) => new Date(now - 30 * HOUR).toISOString().slice(0, 10);

/** NASA GIBS's true-colour tiles from NOAA-20's VIIRS for a day, to zoom 9. */
export const satelliteTiles = (day: string) =>
  `https://gibs.earthdata.nasa.gov/wmts/epsg3857/best/VIIRS_NOAA20_CorrectedReflectance_TrueColor/default/${day}/GoogleMapsCompatible_Level9/{z}/{y}/{x}.jpg`;
