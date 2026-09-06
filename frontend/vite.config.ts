import { defineConfig } from 'vite';
import react from '@vitejs/plugin-react';
import fs from 'fs';
import path from 'path';

// Inject runtime-config.js script tag into built HTML
function injectRuntimeConfig() {
  return {
    name: 'inject-runtime-config',
    transformIndexHtml(html) {
      // Only inject if not already present
      if (!html.includes('src="/runtime-config.js"')) {
        return html.replace(
          '</body>',
          '<script src="/runtime-config.js"></script>\n</body>'
        );
      }
      return html;
    }
  };
}

export default defineConfig({
  plugins: [react(), injectRuntimeConfig()],
  server: {
    port: 3000,
    strictPort: true,
  },
  build: {
    outDir: 'dist',
    sourcemap: true,
  },
});
