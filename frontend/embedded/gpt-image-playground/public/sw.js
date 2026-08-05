const CACHE_NAME = 'lingqu-image-playground-v0.1.1'
const CACHE_PREFIX = 'lingqu-image-playground-'
const INDEX_CACHE_KEY = './index.html'
const APP_SHELL = ['./', './index.html', './manifest.webmanifest', './pwa-icon.svg']

async function isValidAppShell(response) {
  if (!response || !response.ok) return false
  const contentType = response.headers.get('content-type') || ''
  if (!contentType.includes('text/html')) return false

  const html = await response.clone().text()
  return html.includes('<div id="root"></div>')
      && html.includes('AI 图工坊')
    && !html.includes('window.__APP_CONFIG__')
    && !html.includes('<div id="app"></div>')
}

async function cacheAppShell() {
  const cache = await caches.open(CACHE_NAME)
  await Promise.all(APP_SHELL.map(async (asset) => {
    const response = await fetch(asset, { cache: 'reload' })
    if (!response.ok) return

    if (asset === './' || asset === './index.html') {
      if (await isValidAppShell(response)) {
        await cache.put(INDEX_CACHE_KEY, response.clone())
      }
      return
    }

    await cache.put(asset, response.clone())
  }))
}

self.addEventListener('install', (event) => {
  event.waitUntil(cacheAppShell())
  self.skipWaiting()
})

self.addEventListener('activate', (event) => {
  event.waitUntil(
    caches.keys().then((keys) =>
      Promise.all(
        keys
          .filter((key) => key.startsWith(CACHE_PREFIX) && key !== CACHE_NAME)
          .map((key) => caches.delete(key)),
      ),
    ),
  )
  self.clients.claim()
})

self.addEventListener('fetch', (event) => {
  const { request } = event

  if (request.method !== 'GET') return

  const url = new URL(request.url)
  if (url.origin !== self.location.origin) return
  const scopePath = new URL(self.registration.scope).pathname
  if (!url.pathname.startsWith(scopePath)) return

  if (request.mode === 'navigate') {
    event.respondWith(
      fetch(request)
        .then(async (response) => {
          if (await isValidAppShell(response)) {
            const copy = response.clone()
            caches.open(CACHE_NAME).then((cache) => cache.put(INDEX_CACHE_KEY, copy))
          }
          return response
        })
        .catch(() => caches.match(INDEX_CACHE_KEY)),
    )
    return
  }

  event.respondWith(
    caches.match(request).then((cached) => {
      if (cached) return cached

      return fetch(request).then((response) => {
        if (response.ok) {
          const copy = response.clone()
          caches.open(CACHE_NAME).then((cache) => cache.put(request, copy))
        }
        return response
      })
    }),
  )
})
