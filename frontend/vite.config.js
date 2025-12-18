import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'

const apiUrl = `http://localhost:${process.env.API_PORT || 8080}`

export default defineConfig({
  plugins: [vue()],
  server: {
    proxy: {
      '/images': apiUrl,
      '/upload': apiUrl,
      '/settings': apiUrl,
      '/flow': apiUrl,
      '/events': {
        target: apiUrl,
        changeOrigin: true,
      },
    },
  },
})
