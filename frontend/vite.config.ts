import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'
import tailwindcss from '@tailwindcss/vite'
import path from 'path'
import { promises as fs } from 'fs'

// 自定义插件：在构建前后同步模板 HTML 并清理 assets 目录
function backendIntegrationPlugin() {
  return {
    name: 'backend-integration',
    buildStart: async () => {
      const assetsPath = path.resolve(__dirname, '../www/assets')
      try {
        await fs.rm(assetsPath, { recursive: true, force: true })
        console.log(`Cleaned assets directory: ${assetsPath}`)
      } catch (err) {
        console.error('Failed to clean assets directory:', err)
      }
    }
  }
}

// https://vite.dev/config/
export default defineConfig({
  plugins: [tailwindcss(), vue(), backendIntegrationPlugin()],
  base: './',
  resolve: {
    alias: {
      '@': path.resolve(__dirname, 'src')
    }
  },
  build: {
    outDir: '../www',
    assetsDir: 'assets',
    emptyOutDir: false,
    rollupOptions: {
      input: {
        main: path.resolve(__dirname, 'index.html'),
        login: path.resolve(__dirname, 'login.html')
      }
    }
  },
  server: {
    port: 3000,
    proxy: {
      '/api': {
        target: 'http://localhost:23237',
        changeOrigin: true
      },
      '/api/ws': {
        target: 'ws://localhost:23237',
        ws: true,
        changeOrigin: true
      }
    }
  }
})
