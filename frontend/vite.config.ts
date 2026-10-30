import { sveltekit } from '@sveltejs/kit/vite';
import adapter from '@sveltejs/adapter-static';
import { vitePreprocess } from '@sveltejs/vite-plugin-svelte';
import tailwindcss from '@tailwindcss/vite';
import { defineConfig } from 'vite';
import { readFileSync } from 'node:fs';

const isTauri = !!process.env.TAURI_ENV_PLATFORM;

// Single source of truth for the app version (used by the in-app updater to
// compare against the latest GitHub release). Read from package.json at build
// time so it never drifts from the published version.
const pkg = JSON.parse(readFileSync(new URL('./package.json', import.meta.url), 'utf-8'));

export default defineConfig({
  define: {
    __APP_VERSION__: JSON.stringify(pkg.version),
  },

  plugins: [
    tailwindcss(),
    // SvelteKit 3 takes its config here; svelte.config.js is no longer read.
    sveltekit({
      preprocess: vitePreprocess(),
      // Kit 3 defaults to `#lib` (package.json subpath imports), but TypeScript
      // won't resolve the extensionless `#lib/*` targets svelte-check sees, so
      // keep `$lib` via the (deprecated, still supported) alias for now.
      alias: { $lib: 'src/lib' },
      adapter: adapter({
        fallback: 'index.html', // SPA mode: serve index.html for all unmatched routes
      }),
    }),
  ],

  // Tauri expects a fixed port for the dev server
  server: {
    port: 5173,
    strictPort: true,
    // Tauri uses localhost on Windows
    host: isTauri ? '0.0.0.0' : undefined,
  },

  // Env variables prefixed with TAURI_ are exposed to the frontend
  envPrefix: ['VITE_', 'TAURI_ENV_'],

  build: {
    // Tauri uses Chromium on Windows via WebView2 — target modern ES
    target: isTauri ? 'chrome105' : undefined,
  },
});
