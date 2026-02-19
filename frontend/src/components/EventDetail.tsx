"use client";

import type { EventProperties, Severity } from "@/lib/types";
import { EVENT_TYPE_COLORS, EVENT_TYPE_LABELS } from "@/lib/mapStyles";

interface EventDetailProps {
  event: EventProperties;
  onClose: () => void;
}

const SEVERITY_STYLES: Record<Severity, string> = {
  extreme: "bg-red-600 text-white",
  severe: "bg-orange-500 text-white",
  moderate: "bg-yellow-500 text-white",
  minor: "bg-green-600 text-white",
};

export default function EventDetail({ event, onClose }: EventDetailProps) {
  const color = EVENT_TYPE_COLORS[event.event_type] ?? "#888";
  const label = EVENT_TYPE_LABELS[event.event_type] ?? event.event_type;
  const date = new Date(event.started_at).toLocaleString();

  return (
    <div className="bg-white/95 backdrop-blur-sm rounded-lg shadow-lg w-80 max-h-[60vh] overflow-y-auto">
      <div className="flex items-start justify-between p-4 border-b border-gray-200/60">
        <div className="flex items-center gap-2 min-w-0">
          <span
            className="w-3 h-3 rounded-full flex-shrink-0"
            style={{ backgroundColor: color }}
          />
          <span className="text-xs font-semibold text-gray-500 uppercase tracking-wider truncate">
            {label}
          </span>
          {event.severity && (
            <span
              className={`text-[10px] font-bold uppercase px-2 py-0.5 rounded-full flex-shrink-0 ${
                SEVERITY_STYLES[event.severity]
              }`}
            >
              {event.severity}
            </span>
          )}
        </div>
        <button
          onClick={onClose}
          className="text-gray-400 hover:text-gray-600 transition-colors ml-2 flex-shrink-0"
          aria-label="Close event detail"
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

      <div className="p-4 space-y-3">
        <h3 className="text-base font-semibold text-gray-900 leading-snug">
          {event.title}
        </h3>

        <div className="flex flex-wrap gap-x-4 gap-y-1 text-xs text-gray-500">
          {event.magnitude != null && (
            <span>
              Magnitude: <strong className="text-gray-800">{event.magnitude}</strong>
            </span>
          )}
          <span>{date}</span>
        </div>

        {event.description && (
          <p className="text-sm text-gray-600 leading-relaxed">
            {event.description.length > 400
              ? event.description.slice(0, 400) + "..."
              : event.description}
          </p>
        )}

        <div className="pt-2 border-t border-gray-100">
          <span className="text-[11px] text-gray-400">via {event.source}</span>
        </div>
      </div>
    </div>
  );
}
