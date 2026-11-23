import type { Event } from "./api.gen";
import { position } from "./filters";

const R = 6371; // Earth's mean radius, km

/** The great-circle distance between two [lon, lat] points, in km. */
export function distanceKm([lon1, lat1]: [number, number], [lon2, lat2]: [number, number]): number {
  const rad = Math.PI / 180;
  const dLat = (lat2 - lat1) * rad;
  const dLon = (lon2 - lon1) * rad;
  const a = Math.sin(dLat / 2) ** 2 + Math.cos(lat1 * rad) * Math.cos(lat2 * rad) * Math.sin(dLon / 2) ** 2;
  return 2 * R * Math.asin(Math.min(1, Math.sqrt(a)));
}

/**
 * How far to look around a point so the view holds some events: the distance to
 * the tenth nearest placed event, between 100 and 2,500 km.
 */
export function radiusFor(at: [number, number], events: readonly Event[], count = 10): number {
  const d = events.flatMap((e) => {
    const p = position(e);
    return p ? [distanceKm(at, p)] : [];
  });
  d.sort((a, b) => a - b);
  const nth = d[Math.min(count, d.length) - 1] ?? 2500;
  return Math.min(2500, Math.max(100, nth * 1.1));
}

/** A box of the given radius around a point, as [west, south, east, north]. */
export function boxAround([lon, lat]: [number, number], km: number): [number, number, number, number] {
  const dLat = (km / R) * (180 / Math.PI);
  const dLon = dLat / Math.max(0.05, Math.cos((lat * Math.PI) / 180));
  return [lon - dLon, Math.max(-85, lat - dLat), lon + dLon, Math.min(85, lat + dLat)];
}

/** "8 km", "120 km", "1,240 km". */
export const formatKm = (km: number) => `${Math.round(km).toLocaleString("en")} km`;
