// Color math for the token checks: WCAG contrast, OKLab distance, and
// color-vision deficiency simulation (Machado, Oliveira and Fernandes 2009,
// severity 1, applied in linear RGB).

const linear = (c) => (c <= 0.04045 ? c / 12.92 : ((c + 0.055) / 1.055) ** 2.4);

/** rgb returns a #rrggbb color as linear RGB in 0..1. */
export function rgb(hex) {
  const m = /^#([0-9a-f]{6})$/i.exec(hex);
  if (!m) throw new Error(`not a #rrggbb color: ${hex}`);
  return [0, 2, 4].map((i) => linear(parseInt(m[1].slice(i, i + 2), 16) / 255));
}

/** contrast is the WCAG 2 contrast ratio of two colors. */
export function contrast(a, b) {
  const lum = ([r, g, bl]) => 0.2126 * r + 0.7152 * g + 0.0722 * bl;
  const [hi, lo] = [lum(rgb(a)), lum(rgb(b))].sort((x, y) => y - x);
  return (hi + 0.05) / (lo + 0.05);
}

const MACHADO = {
  protanopia: [[0.152286, 1.052583, -0.204868], [0.114503, 0.786281, 0.099216], [-0.003882, -0.048116, 1.051998]],
  deuteranopia: [[0.367322, 0.860646, -0.227968], [0.280085, 0.672501, 0.047413], [-0.01182, 0.04294, 0.968881]],
  tritanopia: [[1.255528, -0.076749, -0.178779], [-0.078411, 0.930809, 0.147602], [0.004733, 0.691367, 0.3039]],
};

/** VISION lists normal sight and the three simulated deficiencies. */
export const VISION = ["normal", ...Object.keys(MACHADO)];

const clamp = (x) => Math.min(1, Math.max(0, x));

function oklab([r, g, b]) {
  const l = Math.cbrt(0.4122214708 * r + 0.5363325363 * g + 0.0514459929 * b);
  const m = Math.cbrt(0.2119034982 * r + 0.6806995451 * g + 0.1073969566 * b);
  const s = Math.cbrt(0.0883024619 * r + 0.2817188376 * g + 0.6299787005 * b);
  return [
    0.2104542553 * l + 0.793617785 * m - 0.0040720468 * s,
    1.9779984951 * l - 2.428592205 * m + 0.4505937099 * s,
    0.0259040371 * l + 0.7827717662 * m - 0.808675766 * s,
  ];
}

/** distance is the OKLab distance (x100) between two colors as seen with the given vision. */
export function distance(a, b, vision = "normal") {
  const see = (hex) => {
    const c = rgb(hex);
    const m = MACHADO[vision];
    return oklab(m ? m.map((row) => clamp(row[0] * c[0] + row[1] * c[1] + row[2] * c[2])) : c);
  };
  const [p, q] = [see(a), see(b)];
  return 100 * Math.hypot(p[0] - q[0], p[1] - q[1], p[2] - q[2]);
}
