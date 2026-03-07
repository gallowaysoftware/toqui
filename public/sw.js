// Toqui Service Worker
// Provides offline support with app shell caching and itinerary data caching.

const CACHE_VERSION = "v1";
const APP_SHELL_CACHE = `toqui-app-shell-${CACHE_VERSION}`;
const ITINERARY_CACHE = `toqui-itineraries-${CACHE_VERSION}`;

// App shell resources to pre-cache on install
const APP_SHELL_URLS = ["/", "/offline.html"];

// Install: pre-cache app shell and offline fallback
self.addEventListener("install", (event) => {
  event.waitUntil(
    caches
      .open(APP_SHELL_CACHE)
      .then((cache) => cache.addAll(APP_SHELL_URLS))
      .then(() => self.skipWaiting()),
  );
});

// Activate: clean up old caches from previous versions
self.addEventListener("activate", (event) => {
  event.waitUntil(
    caches
      .keys()
      .then((keys) =>
        Promise.all(
          keys
            .filter(
              (key) => key !== APP_SHELL_CACHE && key !== ITINERARY_CACHE,
            )
            .map((key) => caches.delete(key)),
        ),
      )
      .then(() => self.clients.claim()),
  );
});

// Determine the caching strategy for a request
function getStrategy(request) {
  const url = new URL(request.url);

  // Only handle same-origin requests
  if (url.origin !== self.location.origin) {
    return "network-only";
  }

  // Itinerary API responses: cache-first (offline itinerary access)
  if (url.pathname.includes("/toqui.v1.TripService/")) {
    return "cache-first";
  }

  // Chat API responses: network-first (always want latest)
  if (url.pathname.includes("/toqui.v1.ChatService/")) {
    return "network-first";
  }

  // Navigation requests (HTML pages): network-first with offline fallback
  if (request.mode === "navigate") {
    return "network-first-navigate";
  }

  // Static assets (JS, CSS, images): cache-first
  if (
    url.pathname.match(/\.(js|css|png|jpg|jpeg|svg|ico|woff2?)$/) ||
    url.pathname.startsWith("/_next/static/")
  ) {
    return "cache-first";
  }

  // Everything else: network-first
  return "network-first";
}

// Cache-first: try cache, fall back to network (and update cache)
async function cacheFirst(request, cacheName) {
  const cached = await caches.match(request);
  if (cached) {
    return cached;
  }
  try {
    const response = await fetch(request);
    if (response.ok) {
      const cache = await caches.open(cacheName);
      cache.put(request, response.clone());
    }
    return response;
  } catch {
    return new Response("Network error", {
      status: 503,
      statusText: "Service Unavailable",
    });
  }
}

// Network-first: try network, fall back to cache
async function networkFirst(request, cacheName) {
  try {
    const response = await fetch(request);
    if (response.ok) {
      const cache = await caches.open(cacheName);
      cache.put(request, response.clone());
    }
    return response;
  } catch {
    const cached = await caches.match(request);
    if (cached) {
      return cached;
    }
    return new Response("Network error", {
      status: 503,
      statusText: "Service Unavailable",
    });
  }
}

// Network-first for navigation: try network, fall back to cache, then offline page
async function networkFirstNavigate(request) {
  try {
    const response = await fetch(request);
    if (response.ok) {
      const cache = await caches.open(APP_SHELL_CACHE);
      cache.put(request, response.clone());
    }
    return response;
  } catch {
    const cached = await caches.match(request);
    if (cached) {
      return cached;
    }
    // Show offline fallback page
    const offlinePage = await caches.match("/offline.html");
    if (offlinePage) {
      return offlinePage;
    }
    return new Response("Offline", {
      status: 503,
      statusText: "Service Unavailable",
    });
  }
}

// Fetch handler: route requests to the appropriate strategy
self.addEventListener("fetch", (event) => {
  // Skip non-GET requests (mutations should always go to network)
  if (event.request.method !== "GET") {
    return;
  }

  const strategy = getStrategy(event.request);

  switch (strategy) {
    case "cache-first":
      event.respondWith(
        cacheFirst(
          event.request,
          event.request.url.includes("/toqui.v1.TripService/")
            ? ITINERARY_CACHE
            : APP_SHELL_CACHE,
        ),
      );
      break;
    case "network-first":
      event.respondWith(networkFirst(event.request, APP_SHELL_CACHE));
      break;
    case "network-first-navigate":
      event.respondWith(networkFirstNavigate(event.request));
      break;
    // network-only: don't intercept, let browser handle normally
  }
});
