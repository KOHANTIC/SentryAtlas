"use client";

import { useState } from "react";
import { EVENT_TYPES } from "@/lib/types";
import { EVENT_TYPE_COLORS, EVENT_TYPE_LABELS } from "@/lib/mapStyles";

export default function Legend() {
  const [collapsed, setCollapsed] = useState(true);

  if (collapsed) {
    return (
      <button
        onClick={() => setCollapsed(false)}
        className="bg-white/90 backdrop-blur-sm rounded-lg shadow-lg px-3 py-2 text-xs font-medium text-gray-600 hover:bg-white transition-colors flex items-center gap-1.5"
        aria-label="Show legend"
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
    <div className="bg-white/90 backdrop-blur-sm rounded-lg shadow-lg overflow-hidden">
      <div className="flex items-center justify-between px-3 py-2 border-b border-gray-200/60">
        <span className="text-xs font-semibold text-gray-700">Legend</span>
        <button
          onClick={() => setCollapsed(true)}
          className="text-gray-400 hover:text-gray-600 transition-colors"
          aria-label="Collapse legend"
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
      <div className="px-3 py-2 space-y-1">
        {EVENT_TYPES.map((type) => (
          <div key={type} className="flex items-center gap-2">
            <span
              className="w-2.5 h-2.5 rounded-full flex-shrink-0"
              style={{ backgroundColor: EVENT_TYPE_COLORS[type] }}
            />
            <span className="text-[11px] text-gray-600">
              {EVENT_TYPE_LABELS[type]}
            </span>
          </div>
        ))}
      </div>
    </div>
  );
}
