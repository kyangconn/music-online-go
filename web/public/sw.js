const CACHE_NAME = "music-online-shell-v1";
const APP_SHELL = [
  "/",
  "/manifest.webmanifest",
  "/icons/pwa-192.png",
  "/icons/pwa-512.png",
];
const CACHEABLE_DESTINATIONS = new Set(["font", "image", "script", "style"]);

self.addEventListener("install", (event) => {
  event.waitUntil(
    caches
      .open(CACHE_NAME)
      .then((cache) => cache.addAll(APP_SHELL))
      .then(() => self.skipWaiting()),
  );
});

self.addEventListener("activate", (event) => {
  event.waitUntil(
    caches
      .keys()
      .then((keys) => Promise.all(keys.filter((key) => key !== CACHE_NAME).map((key) => caches.delete(key))))
      .then(() => self.clients.claim()),
  );
});

self.addEventListener("fetch", (event) => {
  const { request } = event;
  const url = new URL(request.url);

  if (request.method !== "GET" || url.origin !== self.location.origin || url.pathname.startsWith("/api/")) {
    return;
  }

  if (request.mode === "navigate") {
    event.respondWith(
      fetch(request).catch(async () => {
        const shell = await caches.match("/");
        return shell ?? Response.error();
      }),
    );
    return;
  }

  if (!CACHEABLE_DESTINATIONS.has(request.destination) && url.pathname !== "/manifest.webmanifest") {
    return;
  }

  event.respondWith(
    caches.match(request).then((cached) => {
      const fromNetwork = () => fetch(request).then((response) => {
        if (response.ok) {
          const copy = response.clone();
          void caches
            .open(CACHE_NAME)
            .then((cache) => cache.put(request, copy))
            .catch(() => undefined);
        }
        return response;
      });
      if (!cached) return fromNetwork();
      void fromNetwork().catch(() => undefined);
      return cached;
    }),
  );
});
