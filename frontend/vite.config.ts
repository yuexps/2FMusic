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
      const assetsPath = path.resolve(__dirname, 'dist/assets')
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
    outDir: 'dist',
    assetsDir: 'assets',
    emptyOutDir: true,
    rollupOptions: {
      input: {
        main: path.resolve(__dirname, 'index.html')
      },
      output: {
        // 生产级分包优化：将依赖包单独打包以完全消除打包体积过大警告，并极大地优化首屏加载性能
        manualChunks(id) {
          if (id.includes('node_modules')) {
            if (id.includes('naive-ui')) {
              return 'vendor-naive-ui'
            }
            if (id.includes('vue') || id.includes('vue-router') || id.includes('pinia') || id.includes('axios')) {
              return 'vendor-vue-core'
            }
            return 'vendor-libs'
          }
        }
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
