import { describe, expect, it } from "vitest";
import type { Event } from "./api.gen";
import { boxAround, distanceKm, formatKm, radiusFor } from "./near";

const at = (lon: number, lat: number) => ({ location: { center: [lon, lat] } }) as unknown as Event;

describe("near", () => {
  it("measures great-circle distances", () => {
    // San Francisco to Los Angeles is about 559 km.
    expect(Math.round(distanceKm([-122.42, 37.77], [-118.24, 34.05]))).toBeGreaterThan(550);
    expect(Math.round(distanceKm([-122.42, 37.77], [-118.24, 34.05]))).toBeLessThan(570);
    // Across the date line, the short way.
    expect(distanceKm([179.5, 0], [-179.5, 0])).toBeLessThan(120);
  });

  it("looks as far as the tenth nearest event, within 100 and 2,500 km", () => {
    // The tenth is a degree away, about 111 km, plus a tenth for margin.
    const close = Array.from({ length: 12 }, (_, i) => at(0.1 * (i + 1), 0));
    expect(radiusFor([0, 0], close)).toBeCloseTo(122.3, 0);
    // Very close events still get 100 km; very far ones are capped at 2,500.
    expect(radiusFor([0, 0], Array.from({ length: 12 }, () => at(0.01, 0)))).toBe(100);
    expect(radiusFor([0, 0], Array.from({ length: 12 }, (_, i) => at(5 * (i + 1), 0)))).toBe(2500);
    expect(radiusFor([0, 0], [])).toBe(2500);
  });

  it("makes a box around a point", () => {
    const [w, s, e, n] = boxAround([10, 50], 111);
    expect(n - 50).toBeCloseTo(1, 1);
    expect(50 - s).toBeCloseTo(1, 1);
    expect(e - 10).toBeGreaterThan(1.5); // a degree of longitude is shorter at 50°
    expect(w).toBeLessThan(10);
  });

  it("writes kilometres", () => {
    expect(formatKm(1240.4)).toBe("1,240 km");
  });
});
