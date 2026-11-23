import tokens from "../../../brand/tokens.json";

export type Shape = "circle" | "square" | "diamond" | "triangle" | "triangle-down" | "hexagon";
export type Fill = "dotted" | "hollow" | "half" | "full" | "halo";

// Each shape's outline in a box of radius 5 around (0, 0), sized so the shapes look
// equally heavy; the circle has none. brand/BRAND.md § Events names each type's shape.
const OUTLINES: Record<Exclude<Shape, "circle">, [number, number][]> = {
  square: [[-4.3, -4.3], [4.3, -4.3], [4.3, 4.3], [-4.3, 4.3]],
  diamond: [[0, -5.8], [5.8, 0], [0, 5.8], [-5.8, 0]],
  triangle: [[0, -5.6], [5.8, 4.4], [-5.8, 4.4]],
  "triangle-down": [[0, 5.6], [5.8, -4.4], [-5.8, -4.4]],
  hexagon: [[0, -5.6], [4.85, -2.8], [4.85, 2.8], [0, 5.6], [-4.85, 2.8], [-4.85, -2.8]],
};

const R = tokens.mark.size / 2;
const STROKE = tokens.mark.stroke;
const HALO = tokens.mark["halo-gap"];

/** The shape's outline as an SVG path in the same box, for the legend and lists. */
export function svgPath(shape: Shape, scale = 1): string {
  if (shape === "circle") {
    const r = R * scale;
    return `M0,${-r}A${r},${r} 0 1,1 0,${r}A${r},${r} 0 1,1 0,${-r}Z`;
  }
  return "M" + OUTLINES[shape].map(([x, y]) => `${x * scale},${y * scale}`).join("L") + "Z";
}

function trace(ctx: CanvasRenderingContext2D, shape: Shape, scale: number) {
  ctx.beginPath();
  if (shape === "circle") {
    ctx.arc(0, 0, R * scale, 0, Math.PI * 2);
  } else {
    OUTLINES[shape].forEach(([x, y], i) => (i ? ctx.lineTo(x * scale, y * scale) : ctx.moveTo(x * scale, y * scale)));
    ctx.closePath();
  }
}

/** The canvas side in CSS pixels that holds any mark with its halo. */
export const MARK_BOX = 2 * Math.ceil(R + 1 + HALO + STROKE);

/**
 * Draws one mark: the fill says the severity (brand/BRAND.md § Severity). `ground`
 * fills the inside of hollow and half marks, so they read on any background.
 */
export function drawMark(ctx: CanvasRenderingContext2D, shape: Shape, fill: Fill, color: string, ground: string) {
  ctx.lineWidth = STROKE;
  ctx.strokeStyle = color;
  ctx.lineJoin = "round";
  if (fill === "dotted") {
    ctx.setLineDash([tokens.mark.dash, tokens.mark.dash]);
    trace(ctx, shape, 0.8);
    ctx.stroke();
    ctx.setLineDash([]);
    return;
  }
  trace(ctx, shape, 1);
  ctx.fillStyle = fill === "full" || fill === "halo" ? color : ground;
  ctx.fill();
  if (fill === "half") {
    ctx.save();
    ctx.beginPath();
    ctx.rect(-R * 2, -R * 2, R * 2, R * 4); // the left half
    ctx.clip();
    trace(ctx, shape, 1);
    ctx.fillStyle = color;
    ctx.fill();
    ctx.restore();
  }
  trace(ctx, shape, 1);
  ctx.stroke();
  if (fill === "halo") {
    ctx.lineWidth = STROKE / 1.4;
    trace(ctx, shape, (R + HALO) / R);
    ctx.stroke();
  }
}

/** One mark as pixels, for map.addImage. */
export function markImage(shape: Shape, fill: Fill, color: string, ground: string, pixelRatio: number) {
  const side = MARK_BOX * pixelRatio;
  const canvas = document.createElement("canvas");
  canvas.width = canvas.height = side;
  const ctx = canvas.getContext("2d")!;
  ctx.translate(side / 2, side / 2);
  ctx.scale(pixelRatio, pixelRatio);
  drawMark(ctx, shape, fill, color, ground);
  return ctx.getImageData(0, 0, side, side);
}
