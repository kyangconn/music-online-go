const CACHE_PREFIX = "music-online-shell-";
const CACHE_NAME = `${CACHE_PREFIX}v2`;
const APP_SHELL = [
  "/",
  "/manifest.webmanifest",
  "/icons/pwa-192.png",
  "/icons/pwa-512.png",
];
const APP_SHELL_URLS = new Set(APP_SHELL.map((path) => new URL(path, self.location.origin).href));
const CACHEABLE_DESTINATIONS = new Set(["font", "image", "script", "style"]);

async function pruneRuntimeEntries(cache) {
  const entries = await cache.keys();
  await Promise.all(entries.filter((request) => !APP_SHELL_URLS.has(request.url)).map((request) => cache.delete(request)));
}

async function refreshNavigationShell(response) {
  const cache = await caches.open(CACHE_NAME);
  await pruneRuntimeEntries(cache);
  await cache.put("/", response.clone());
}

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
    Promise.all([
      caches
        .keys()
        .then((keys) => Promise.all(
          keys
            .filter((key) => key.startsWith(CACHE_PREFIX) && key !== CACHE_NAME)
            .map((key) => caches.delete(key)),
        )),
      caches.open(CACHE_NAME).then((cache) => pruneRuntimeEntries(cache)),
    ])
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
      fetch(request)
        .then(async (response) => {
          if (response.ok) {
            await refreshNavigationShell(response).catch(() => undefined);
          }
          return response;
        })
        .catch(async () => {
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
