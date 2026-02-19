"use client";

import { useState } from "react";
import { EVENT_TYPES, EventType } from "@/lib/types";
import { EVENT_TYPE_COLORS, EVENT_TYPE_LABELS } from "@/lib/mapStyles";
import { TIME_PRESETS, getSinceDate, getActivePreset } from "@/lib/time";

interface FilterPanelProps {
  visibleTypes: EventType[];
  onVisibleTypesChange: (types: EventType[]) => void;
  since?: string;
  onSinceChange: (since?: string) => void;
}

export default function FilterPanel({
  visibleTypes,
  onVisibleTypesChange,
  since,
  onSinceChange,
}: FilterPanelProps) {
  const [collapsed, setCollapsed] = useState(false);
  const activePreset = getActivePreset(since);

  const toggleType = (type: EventType) => {
    const next = visibleTypes.includes(type)
      ? visibleTypes.filter((t) => t !== type)
      : [...visibleTypes, type];
    onVisibleTypesChange(next);
  };

  const selectAll = () => onVisibleTypesChange([...EVENT_TYPES]);
  const clearAll = () => onVisibleTypesChange([]);

  const setTimePreset = (preset: (typeof TIME_PRESETS)[number]["value"]) => {
    const isActive = activePreset === preset;
    onSinceChange(isActive ? undefined : getSinceDate(preset));
  };

  if (collapsed) {
    return (
      <button
        onClick={() => setCollapsed(false)}
        className="bg-white/90 backdrop-blur-sm rounded-lg shadow-lg p-3 hover:bg-white transition-colors cursor-pointer text-gray-800"
        aria-label="Open filters"
      >
        <svg
          xmlns="http://www.w3.org/2000/svg"
          width="20"
          height="20"
          viewBox="0 0 24 24"
          fill="none"
          stroke="currentColor"
          strokeWidth="2"
          strokeLinecap="round"
          strokeLinejoin="round"
        >
          <polygon points="22 3 2 3 10 12.46 10 19 14 21 14 12.46 22 3" />
        </svg>
      </button>
    );
  }

  return (
    <div className="bg-white/90 backdrop-blur-sm rounded-lg shadow-lg w-64 max-h-[calc(100vh-2rem)] overflow-y-auto">
      <div className="flex items-center justify-between p-3 border-b border-gray-200/60">
        <h2 className="text-sm font-semibold text-gray-800">Filters</h2>
        <button
          onClick={() => setCollapsed(true)}
          className="text-gray-400 hover:text-gray-600 transition-colors cursor-pointer"
          aria-label="Collapse filters"
        >
          <svg
            xmlns="http://www.w3.org/2000/svg"
            width="16"
            height="16"
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

      <div className="p-3 border-b border-gray-200/60">
        <div className="flex items-center justify-between mb-2">
          <span className="text-xs font-medium text-gray-500 uppercase tracking-wider">
            Time Range
          </span>
        </div>
        <div className="flex gap-1.5">
          {TIME_PRESETS.map((preset) => (
            <button
              key={preset.value}
              onClick={() => setTimePreset(preset.value)}
              className={`flex-1 text-xs font-medium py-1.5 rounded-md transition-colors cursor-pointer ${
                activePreset === preset.value
                  ? "bg-gray-800 text-white"
                  : "bg-gray-100 text-gray-600 hover:bg-gray-200"
              }`}
            >
              {preset.label}
            </button>
          ))}
        </div>
      </div>

      <div className="p-3">
        <div className="flex items-center justify-between mb-2">
          <span className="text-xs font-medium text-gray-500 uppercase tracking-wider">
            Event Types
          </span>
          <div className="flex gap-2">
            <button
              onClick={selectAll}
              className="text-[10px] text-blue-600 hover:text-blue-800 font-medium cursor-pointer"
            >
              All
            </button>
            <button
              onClick={clearAll}
              className="text-[10px] text-blue-600 hover:text-blue-800 font-medium cursor-pointer"
            >
              None
            </button>
          </div>
        </div>

        <div className="space-y-0.5">
          {EVENT_TYPES.map((type) => {
            const active = visibleTypes.includes(type);
            return (
              <button
                key={type}
                onClick={() => toggleType(type)}
                className={`flex items-center gap-2 w-full px-2 py-1.5 rounded-md text-left text-sm transition-colors cursor-pointer ${
                  active
                    ? "text-gray-800 hover:bg-gray-100"
                    : "text-gray-400 hover:bg-gray-50"
                }`}
              >
                <span
                  className="w-3 h-3 rounded-full flex-shrink-0 border border-white"
                  style={{
                    backgroundColor: active
                      ? EVENT_TYPE_COLORS[type]
                      : "#d1d5db",
                    boxShadow: active ? `0 0 0 1px ${EVENT_TYPE_COLORS[type]}40` : "none",
                  }}
                />
                <span className="truncate">{EVENT_TYPE_LABELS[type]}</span>
              </button>
            );
          })}
        </div>
      </div>
    </div>
  );
}
