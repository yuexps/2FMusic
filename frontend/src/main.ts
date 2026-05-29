import { createApp } from 'vue'
import { createPinia } from 'pinia'
import App from './App.vue'
import router from './router'
import SvgIcon from './components/SvgIcon.vue'
import { coverCacheManager } from './utils/coverCache'
import { musicDB } from './utils/indexedDB'
import { getApiUrl } from './utils/path'
import './style.css'
import './components.css'

const app = createApp(App)
app.component('SvgIcon', SvgIcon)

function releaseElCache(el: HTMLImageElement) {
  const cachedId = (el as any)._cachedId
  const cachedUrl = (el as any)._cachedObjectUrl
  
  if (cachedId && cachedUrl) {
    coverCacheManager.release(cachedId, cachedUrl)
    delete (el as any)._cachedId
    delete (el as any)._cachedObjectUrl
  }
}

async function applyCachedSrc(el: HTMLImageElement, binding: any) {
  const { id, src } = binding.value || {}
  const oldVal = binding.oldValue || {}
  
  // 只有当明确存在旧值（非初次挂载），且新旧值完全一致时，才安全跳过重绘
  if (binding.oldValue && id === oldVal.id && src === oldVal.src) {
    return
  }

  // 记录请求 ID 避免异步时序错乱
  const currentReqId = (el as any)._lastReqId = ((el as any)._lastReqId || 0) + 1

  // 清除旧的引用
  releaseElCache(el)

  if (!id) {
    el.src = getApiUrl(src || '/ICON.PNG')
    return
  }

  const cacheEnabled = localStorage.getItem('2fmusic_cache_covers') === 'true'
  if (!cacheEnabled) {
    el.src = getApiUrl(src || '/ICON.PNG')
    return
  }

  try {
    const url = await coverCacheManager.getOrCreateUrl(id)
    
  // 若异步期间指令又触发新加载，抛弃本次回调
  if ((el as any)._lastReqId !== currentReqId) {
      return
    }

    if (url) {
      coverCacheManager.retain(id, url)
      el.src = url
      ;(el as any)._cachedId = id
      ;(el as any)._cachedObjectUrl = url
      return
    }

    // 无缓存时用 fetch 下载图片并入库，避免直接挂 src 触发浏览器下载
    if (src && !src.startsWith('blob:') && !src.startsWith('data:') && !src.endsWith('/ICON.PNG') && !src.endsWith('/ICON.png')) {
      el.src = getApiUrl('/ICON.PNG') // 暂时渲染默认占位符，不让 HTTP 链接挂到 src 上触发抢跑
      
      fetch(getApiUrl(src))
        .then(res => {
          if (!res.ok) throw new Error('Fetch failed')
          return res.blob()
        })
        .then(async blob => {
          // 保存到数据库
          await musicDB.saveCover(id, blob)
          
          // 从池子中重新加载刚刚保存的 Blob URL（它会被新建并插入共享池）
          const cachedUrl = await coverCacheManager.getOrCreateUrl(id)
          
          // 若在此异步期间，此 img 没有承接新歌曲的加载请求，则将其设置为此 Blob URL
          if ((el as any)._lastReqId === currentReqId && cachedUrl) {
            coverCacheManager.retain(id, cachedUrl)
            el.src = cachedUrl
            ;(el as any)._cachedId = id
            ;(el as any)._cachedObjectUrl = cachedUrl
          }
        })
        .catch(err => {
          console.debug('v-cached-src 获取和缓存图片失败，降级使用直链 src:', err)
          if ((el as any)._lastReqId === currentReqId) {
            el.src = getApiUrl(src)
          }
        })
    } else {
      el.src = getApiUrl(src || '/ICON.PNG')
    }
  } catch (e) {
    if ((el as any)._lastReqId === currentReqId) {
      el.src = getApiUrl(src || '/ICON.PNG')
    }
  }
}

app.directive('cached-src', {
  mounted: applyCachedSrc,
  updated: applyCachedSrc,
  unmounted(el: HTMLImageElement) {
    releaseElCache(el)
  }
})

app.use(createPinia())
app.use(router)

// 确保路由初始化就绪之后再进行挂载
router.isReady().then(() => {
  app.mount('#app')
})
