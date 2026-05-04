/**
 * Web stub for the MapLibre-using native map renderer.
 *
 * Metro's web target resolves `./ItineraryMapNative` to this file (Metro picks
 * `*.web.tsx` over the bare `.tsx` and over `*.native.tsx`). This keeps the
 * heavy `@maplibre/maplibre-react-native` package out of the web bundle
 * entirely — including out of the lazy chunk produced by the parent
 * `React.lazy(() => import("./ItineraryMapNative"))` call.
 *
 * The parent (`ItineraryMap.tsx`) short-circuits to a placeholder before
 * mounting the <Suspense> boundary on web, so this stub is never actually
 * rendered. We still export a valid React component so type-checking and
 * the lazy-import contract (default export must be a component) are
 * preserved on web.
 */

import type { ComponentType } from "react";

interface ItineraryMapNativeProps {
  markers: unknown;
  bounds: unknown;
  height: number;
}

const ItineraryMapNativeWebStub: ComponentType<ItineraryMapNativeProps> = () => null;

export default ItineraryMapNativeWebStub;
