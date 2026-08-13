"use client";

import { useState } from "react";
import { EVENT_TYPES } from "@/lib/types";
import { EVENT_TYPE_COLORS, EVENT_TYPE_LABELS, SEVERITY_RAMP_GRADIENT } from "@/lib/mapStyles";

export default function Legend() {
  const [collapsed, setCollapsed] = useState(true);

  if (collapsed) {
    return (
      <button
        onClick={() => setCollapsed(false)}
        className="bg-surface-raised/95 backdrop-blur-sm border border-border rounded-lg shadow-lg px-3 py-2 text-xs font-medium text-foreground-muted hover:bg-surface-overlay transition-colors flex items-center gap-1.5 cursor-pointer"
        aria-label="Show legend"
        aria-expanded={false}
        aria-controls="map-legend"
      >
        <svg
          xmlns="http://www.w3.org/2000/svg"
          width="14"
          height="14"
          viewBox="0 0 24 24"
          fill="none"
          stroke="currentColor"
          strokeWidth="2"
          strokeLinecap="round"
          strokeLinejoin="round"
        >
          <circle cx="12" cy="12" r="10" />
          <line x1="12" y1="16" x2="12" y2="12" />
          <line x1="12" y1="8" x2="12.01" y2="8" />
        </svg>
        Legend
      </button>
    );
  }

  return (
    <div id="map-legend" className="bg-surface-raised/95 backdrop-blur-sm border border-border rounded-lg shadow-lg overflow-hidden">
      <div className="flex items-center justify-between px-3 py-2 border-b border-border">
        <span className="text-xs font-semibold text-foreground">Legend</span>
        <button
          onClick={() => setCollapsed(true)}
          className="text-foreground-muted hover:text-foreground transition-colors cursor-pointer"
          aria-label="Collapse legend"
          aria-expanded={true}
          aria-controls="map-legend"
        >
          <svg
            xmlns="http://www.w3.org/2000/svg"
            width="14"
            height="14"
            viewBox="0 0 24 24"
            fill="none"
            stroke="currentColor"
            strokeWidth="2"
            strokeLinecap="round"
            strokeLinejoin="round"
          >
            <line x1="18" y1="6" x2="6" y2="18" />
            <line x1="6" y1="6" x2="18" y2="18" />
          </svg>
        </button>
      </div>

      <div className="px-3 py-2 border-b border-border">
        <span className="text-[10px] font-medium text-foreground-muted uppercase tracking-wider">
          Severity Intensity
        </span>
        <div
          className="mt-1.5 h-2.5 "
          style={{ background: SEVERITY_RAMP_GRADIENT }}
        />
        <div className="flex justify-between mt-1">
          <span className="text-[10px] text-foreground-muted">Low</span>
          <span className="text-[10px] text-foreground-muted">High</span>
        </div>
      </div>

      <div className="px-3 py-2 space-y-1">
        {EVENT_TYPES.map((type) => (
          <div key={type} className="flex items-center gap-2">
            <span
              className="w-2.5 h-2.5 flex-shrink-0"
              style={{ backgroundColor: EVENT_TYPE_COLORS[type] }}
            />
            <span className="text-[11px] text-foreground-muted">
              {EVENT_TYPE_LABELS[type]}
            </span>
          </div>
        ))}
      </div>
    </div>
  );
}
