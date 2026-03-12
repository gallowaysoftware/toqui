"use client";

import { useCallback, useRef } from "react";
import maplibregl from "maplibre-gl";
import { Map } from "./Map";
import type { Itinerary, ItineraryDay, ItineraryItem } from "@/gen/toqui/v1/trip_pb";
import { MapPin } from "lucide-react";
import { getDayColor } from "./colors";

export { getDayColor } from "./colors";

export interface ItineraryMapProps {
  itinerary?: Itinerary;
  isLoading?: boolean;
  className?: string;
}

/** Create an SVG marker element for a given color */
function createMarkerElement(color: string, label: string): HTMLDivElement {
  const el = document.createElement("div");
  el.className = "itinerary-marker";
  el.style.cursor = "pointer";
  el.innerHTML = `
    <svg width="28" height="36" viewBox="0 0 28 36" fill="none" xmlns="http://www.w3.org/2000/svg">
      <path d="M14 0C6.268 0 0 6.268 0 14c0 10.5 14 22 14 22s14-11.5 14-22C28 6.268 21.732 0 14 0z" fill="${color}"/>
      <circle cx="14" cy="14" r="6" fill="white"/>
      <text x="14" y="17" text-anchor="middle" font-size="10" font-weight="bold" fill="${color}">${label}</text>
    </svg>
  `;
  return el;
}

/** Build popup HTML for an itinerary item */
function buildPopupHTML(item: ItineraryItem, dayNumber: number, color: string): string {
  const typeLabel = item.type ? item.type.charAt(0).toUpperCase() + item.type.slice(1) : "";
  return `
    <div style="min-width: 180px; max-width: 260px;">
      <div style="display: flex; align-items: center; gap: 6px; margin-bottom: 4px;">
        <span style="display: inline-block; width: 10px; height: 10px; border-radius: 50%; background: ${color};"></span>
        <span style="font-size: 11px; color: var(--color-text-tertiary);">Day ${dayNumber}${typeLabel ? ` \u00B7 ${typeLabel}` : ""}</span>
      </div>
      <div style="font-weight: 600; font-size: 14px; margin-bottom: 4px; color: var(--color-text-primary);">${escapeHTML(item.title)}</div>
      ${item.description ? `<div style="font-size: 12px; color: var(--color-text-tertiary); line-height: 1.4;">${escapeHTML(item.description)}</div>` : ""}
    </div>
  `;
}

function escapeHTML(str: string): string {
  const div = document.createElement("div");
  div.textContent = str;
  return div.innerHTML;
}

/**
 * Map with itinerary markers grouped by day.
 * Items without coordinates are silently skipped.
 * When there are no locatable items, an empty state message is shown.
 */
export function ItineraryMap({ itinerary, isLoading, className }: ItineraryMapProps) {
  const markersRef = useRef<maplibregl.Marker[]>([]);

  /** Extract items that have valid coordinates */
  const getLocatableItems = useCallback((): Array<{
    item: ItineraryItem;
    day: ItineraryDay;
  }> => {
    if (!itinerary?.days) return [];
    const result: Array<{ item: ItineraryItem; day: ItineraryDay }> = [];
    for (const day of itinerary.days) {
      for (const item of day.items) {
        if (item.location && item.location.latitude !== 0 && item.location.longitude !== 0) {
          result.push({ item, day });
        }
      }
    }
    return result;
  }, [itinerary]);

  const handleMapReady = useCallback(
    (map: maplibregl.Map) => {
      // Clear any existing markers
      for (const m of markersRef.current) {
        m.remove();
      }
      markersRef.current = [];

      const locatable = getLocatableItems();
      if (locatable.length === 0) return;

      const bounds = new maplibregl.LngLatBounds();

      for (const { item, day } of locatable) {
        const loc = item.location!;
        const color = getDayColor(day.dayNumber);
        const lngLat: [number, number] = [loc.longitude, loc.latitude];

        const el = createMarkerElement(color, String(day.dayNumber));

        const popup = new maplibregl.Popup({
          offset: 25,
          closeButton: true,
          closeOnClick: false,
          maxWidth: "280px",
        }).setHTML(buildPopupHTML(item, day.dayNumber, color));

        const marker = new maplibregl.Marker({ element: el })
          .setLngLat(lngLat)
          .setPopup(popup)
          .addTo(map);

        markersRef.current.push(marker);
        bounds.extend(lngLat);
      }

      // Fit the map to show all markers with padding
      if (locatable.length === 1) {
        const loc = locatable[0].item.location!;
        map.flyTo({ center: [loc.longitude, loc.latitude], zoom: 14 });
      } else {
        map.fitBounds(bounds, { padding: 60, maxZoom: 15 });
      }
    },
    [getLocatableItems],
  );

  const locatable = getLocatableItems();
  const hasMarkers = locatable.length > 0;

  if (isLoading) {
    return (
      <div
        className={`flex items-center justify-center bg-[var(--color-surface-tertiary)] rounded-xl ${className ?? ""}`}
        data-testid="map-loading"
      >
        <div className="text-center text-[var(--color-text-tertiary)]">
          <div className="animate-spin rounded-full h-6 w-6 border-b-2 border-[var(--color-accent)] mx-auto mb-2" />
          <p className="text-sm">Loading map...</p>
        </div>
      </div>
    );
  }

  if (!hasMarkers) {
    return (
      <div
        className={`flex items-center justify-center bg-[var(--color-surface-tertiary)] rounded-xl ${className ?? ""}`}
        data-testid="map-empty"
      >
        <div className="text-center text-[var(--color-text-tertiary)] px-4">
          <MapPin className="mx-auto mb-2" size={24} />
          <p className="text-sm font-medium text-[var(--color-text-secondary)]">No locations yet</p>
          <p className="text-xs mt-1">Chat with the AI to add places to your itinerary</p>
        </div>
      </div>
    );
  }

  return (
    <div className={`relative ${className ?? ""}`}>
      <Map onMapReady={handleMapReady} className="rounded-xl overflow-hidden" />
      {/* Day legend */}
      <div className="absolute bottom-3 left-3 bg-[color-mix(in_srgb,var(--color-surface)_90%,transparent)] backdrop-blur-sm rounded-lg px-3 py-2 shadow-sm border border-[var(--color-border)]">
        <div className="flex flex-wrap gap-2">
          {itinerary?.days
            ?.filter((d) =>
              d.items.some(
                (i) => i.location && i.location.latitude !== 0 && i.location.longitude !== 0,
              ),
            )
            .map((day) => (
              <div
                key={day.id || day.dayNumber}
                className="flex items-center gap-1 text-xs text-[var(--color-text-secondary)]"
              >
                <span
                  className="inline-block w-2.5 h-2.5 rounded-full"
                  style={{ backgroundColor: getDayColor(day.dayNumber) }}
                />
                Day {day.dayNumber}
              </div>
            ))}
        </div>
      </div>
    </div>
  );
}
