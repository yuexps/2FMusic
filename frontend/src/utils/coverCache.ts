import { musicDB } from './indexedDB'

export interface CoverCacheItem {
  url: string
  refCount: number
}

class CoverCacheManager {
  private cache = new Map<string, CoverCacheItem>()

  /**
   * 获取或创建指定 id 的 Blob URL，幂等
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
   * 递减引用，无引用时释放 Object URL
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
   * 强制删除并销毁 Blob URL
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
