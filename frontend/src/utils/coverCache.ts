import { musicDB } from './indexedDB'

export interface CoverCacheItem {
  url: string
  refCount: number
}

class CoverCacheManager {
  private cache = new Map<string, CoverCacheItem>()

  /**
   * 获取或新建指定 id 的图片 Blob URL。此方法是幂等的。
   */
  async getOrCreateUrl(id: string): Promise<string | null> {
    let cached = this.cache.get(id)
    if (cached) {
      return cached.url
    }

    const cachedBlob = await musicDB.getCover(id)
    if (cachedBlob) {
      // 异步 getCover 期间，可能别的并发逻辑已经创建成功，在此做个双重检查
      cached = this.cache.get(id)
      if (cached) {
        return cached.url
      }

      const url = URL.createObjectURL(cachedBlob)
      this.cache.set(id, { url, refCount: 0 })
      return url
    }

    return null
  }

  /**
   * 增加指定 id 的 Blob URL 的引用计数
   */
  retain(id: string, url: string) {
    const cached = this.cache.get(id)
    if (cached && cached.url === url) {
      cached.refCount++
    }
  }

  /**
   * 递减引用计数。当无任何图片或播放器使用时，彻底释放 Object URL 内存占用
   */
  release(id: string, url: string) {
    const cached = this.cache.get(id)
    if (cached && cached.url === url) {
      cached.refCount--
      if (cached.refCount <= 0) {
        URL.revokeObjectURL(cached.url)
        this.cache.delete(id)
      }
    }
  }

  /**
   * 强制从缓存池中删除并销毁 Blob URL 缓存
   */
  delete(id: string) {
    const cached = this.cache.get(id)
    if (cached) {
      URL.revokeObjectURL(cached.url)
      this.cache.delete(id)
    }
  }
}

export const coverCacheManager = new CoverCacheManager()
