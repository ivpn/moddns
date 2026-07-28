import path from "path"
import tailwindcss from "@tailwindcss/vite"
import react from "@vitejs/plugin-react"
import { defineConfig } from "vite"
import { VitePWA } from "vite-plugin-pwa"

// Unique per build. Injected into the bundle as __APP_BUILD_ID__ and emitted
// as /version.json; src/lib/swUpdate.ts polls the latter to detect deploys on
// browsers where the SW lifecycle never surfaces them (Safari/iOS).
const buildId = crypto.randomUUID()

// https://vite.dev/config/
export default defineConfig({
  define: {
    __APP_BUILD_ID__: JSON.stringify(buildId),
  },
  plugins: [
    react(),
    tailwindcss(),
    {
      name: 'moddns:emit-version-json',
      apply: 'build',
      generateBundle() {
        // Not precached: the PWA globPatterns exclude .json on purpose, so the
        // freshness poll always sees the server's current build id.
        this.emitFile({
          type: 'asset',
          fileName: 'version.json',
          source: JSON.stringify({ buildId }),
        })
      },
    },
    VitePWA({
      // 'prompt': the new SW stays in "waiting" until the app applies it via
      // the update flow in setupSWUpdate (src/lib/swUpdate.ts), so an open tab
      // never mixes old page code with a new SW whose precache dropped the old
      // chunks.
      registerType: 'prompt',
      workbox: {
        globPatterns: ['**/*.{js,css,html,ico,png,svg,webmanifest}'],
        cleanupOutdatedCaches: true,
        runtimeCaching: [
          {
            urlPattern: /^https:\/\/.*\.(?:png|jpg|jpeg|svg|gif|webp)$/,
            handler: 'CacheFirst',
            options: {
              cacheName: 'images',
              expiration: { maxEntries: 50, maxAgeSeconds: 30 * 24 * 60 * 60 },
            },
          },
          {
            urlPattern: /^https:\/\/.*\/api\//,
            handler: 'NetworkFirst',
            options: {
              cacheName: 'api',
              expiration: { maxEntries: 100, maxAgeSeconds: 5 * 60 },
            },
          },
        ],
      },
      manifest: false,
      includeAssets: ['favicon.ico', 'android-chrome-192x192.png', 'android-chrome-512x512.png'],
    }),
  ],
  resolve: {
    alias: {
      "@": path.resolve(__dirname, "./src"),
    },
  },
  build: {
    target: 'es2020',
    rollupOptions: {
      output: {
        // vite 8 bundles with rolldown, whose manualChunks takes the function
        // form (the object form is a rollup-only API). Same vendor split as before.
        manualChunks(id) {
          if (!id.includes('node_modules')) return undefined
          if (/[\\/]node_modules[\\/](react|react-dom|react-router|react-router-dom)[\\/]/.test(id)) {
            return 'vendor-react'
          }
          if (id.includes('/node_modules/@radix-ui/')) return 'vendor-radix'
          if (id.includes('/node_modules/@sentry/')) return 'vendor-sentry'
          return undefined
        },
      },
    },
  },
})
