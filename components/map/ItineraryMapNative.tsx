/**
 * Default-resolution shim for the lazy-loaded itinerary map renderer.
 *
 * Metro picks the platform-specific sibling at bundle time:
 *   - web    → `ItineraryMapNative.web.tsx` (a noop stub; maplibre is excluded)
 *   - native → `ItineraryMapNative.native.tsx` (the real maplibre renderer)
 *
 * This base file exists so that TypeScript (which doesn't understand Metro's
 * `*.web` / `*.native` extensions) and any tooling that does plain Node-style
 * resolution can still resolve `./ItineraryMapNative`. It mirrors the type
 * surface and runtime contract of the platform-specific files. It is not
 * shipped to any platform's bundle — Metro always prefers a platform-suffixed
 * sibling when one exists.
 */

import type { ComponentType } from "react";

interface ItineraryMapNativeProps {
  markers: unknown;
  bounds: unknown;
  height: number;
}

const ItineraryMapNativeShim: ComponentType<ItineraryMapNativeProps> = () => null;

export default ItineraryMapNativeShim;
