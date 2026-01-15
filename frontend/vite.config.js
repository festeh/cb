import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'

const apiTarget = `http://localhost:${process.env.API_PORT || 8080}`

export default defineConfig({
  plugins: [
    vue(),
    {
      name: 'log-api-target',
      configureServer() {
        console.log(`\n  API server: ${apiTarget}\n`)
      },
    },
  ],
  server: {
    proxy: {
      '/api': {
        target: apiTarget,
        changeOrigin: true,
        rewrite: (path) => path.replace(/^\/api/, ''),
      },
    },
  },
})
