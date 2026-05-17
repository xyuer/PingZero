import vue from '@vitejs/plugin-vue'
import { defineConfig } from 'vite'

export default defineConfig({
  plugins: [vue()],
  build: {
    outDir: '../cmd/client/frontend/dist',
    emptyOutDir: true
  }
})
