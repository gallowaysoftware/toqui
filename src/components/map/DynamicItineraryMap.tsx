"use client";

import dynamic from "next/dynamic";
import type { ItineraryMapProps } from "./ItineraryMap";
import { MapPin } from "lucide-react";

/**
 * Dynamically imported ItineraryMap with SSR disabled.
 * MapLibre GL JS requires the DOM (window, document, WebGL),
 * so it must not render on the server.
 */
const DynamicItineraryMap = dynamic<ItineraryMapProps>(
  () => import("./ItineraryMap").then((mod) => mod.ItineraryMap),
  {
    ssr: false,
    loading: () => (
      <div
        className="flex items-center justify-center bg-[var(--color-surface-tertiary)] rounded-xl h-full w-full"
        data-testid="map-loading"
      >
        <div className="text-center text-[var(--color-text-tertiary)]">
          <MapPin className="mx-auto mb-2 animate-pulse" size={24} />
          <p className="text-sm">Loading map...</p>
        </div>
      </div>
    ),
  },
);

export default DynamicItineraryMap;
