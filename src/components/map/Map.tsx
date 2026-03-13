"use client";

import { useEffect, useRef, useCallback } from "react";
import maplibregl from "maplibre-gl";
import "maplibre-gl/dist/maplibre-gl.css";

const TILE_URL = "https://tile.openstreetmap.org/{z}/{x}/{y}.png";

const DEFAULT_CENTER: [number, number] = [0, 20];
const DEFAULT_ZOOM = 2;

export interface MapProps {
  /** Called once when the map instance is ready */
  onMapReady?: (map: maplibregl.Map) => void;
  /** CSS class for the outer wrapper */
  className?: string;
}

/**
 * Base MapLibre GL JS component.
 * Renders an interactive map using OpenStreetMap raster tiles.
 * Must be loaded via next/dynamic with ssr: false.
 */
export function Map({ onMapReady, className }: MapProps) {
  const containerRef = useRef<HTMLDivElement>(null);
  const mapRef = useRef<maplibregl.Map | null>(null);
  const onMapReadyRef = useRef(onMapReady);

  // Keep the callback ref up to date without retriggering the effect
  useEffect(() => {
    onMapReadyRef.current = onMapReady;
  });

  const initMap = useCallback(() => {
    if (!containerRef.current || mapRef.current) return;

    const map = new maplibregl.Map({
      container: containerRef.current,
      style: {
        version: 8,
        sources: {
          osm: {
            type: "raster",
            tiles: [TILE_URL],
            tileSize: 256,
            attribution:
              '&copy; <a href="https://www.openstreetmap.org/copyright">OpenStreetMap</a> contributors',
          },
        },
        layers: [
          {
            id: "osm-tiles",
            type: "raster",
            source: "osm",
            minzoom: 0,
            maxzoom: 19,
          },
        ],
      },
      center: DEFAULT_CENTER,
      zoom: DEFAULT_ZOOM,
    });

    map.addControl(new maplibregl.NavigationControl(), "top-right");

    mapRef.current = map;

    map.on("load", () => {
      onMapReadyRef.current?.(map);
    });
  }, []);

  useEffect(() => {
    initMap();

    return () => {
      if (mapRef.current) {
        mapRef.current.remove();
        mapRef.current = null;
      }
    };
  }, [initMap]);

  return (
    <div
      ref={containerRef}
      className={className}
      style={{ width: "100%", height: "100%" }}
      role="region"
      aria-label="Interactive map"
      data-testid="map-container"
    />
  );
}
