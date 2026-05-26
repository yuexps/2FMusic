class MusicDB {
  private dbName = '2fmusic_local_cache'
  private dbVersion = 2
  private db: IDBDatabase | null = null

  async init(): Promise<IDBDatabase> {
    if (this.db) return this.db
    return new Promise((resolve, reject) => {
      const request = indexedDB.open(this.dbName, this.dbVersion)
      request.onerror = () => reject(request.error)
      request.onsuccess = () => {
        this.db = request.result
        resolve(request.result)
      }
      request.onupgradeneeded = () => {
        const db = request.result
        if (!db.objectStoreNames.contains('covers')) {
          db.createObjectStore('covers')
        }
        if (!db.objectStoreNames.contains('lyrics')) {
          db.createObjectStore('lyrics')
        }
        if (!db.objectStoreNames.contains('background')) {
          db.createObjectStore('background')
        }
      }
    })
  }

  async getCover(songId: string): Promise<Blob | null> {
    const db = await this.init()
    return new Promise((resolve) => {
      const transaction = db.transaction('covers', 'readonly')
      const store = transaction.objectStore('covers')
      const request = store.get(songId)
      request.onsuccess = () => resolve(request.result || null)
      request.onerror = () => resolve(null)
    })
  }

  async saveCover(songId: string, blob: Blob): Promise<void> {
    const db = await this.init()
    return new Promise((resolve, reject) => {
      const transaction = db.transaction('covers', 'readwrite')
      const store = transaction.objectStore('covers')
      const request = store.put(blob, songId)
      request.onsuccess = () => resolve()
      request.onerror = () => reject(request.error)
    })
  }

  async getLyrics(songId: string): Promise<string | null> {
    const db = await this.init()
    return new Promise((resolve) => {
      const transaction = db.transaction('lyrics', 'readonly')
      const store = transaction.objectStore('lyrics')
      const request = store.get(songId)
      request.onsuccess = () => resolve(request.result || null)
      request.onerror = () => resolve(null)
    })
  }

  async saveLyrics(songId: string, lyrics: string): Promise<void> {
    const db = await this.init()
    return new Promise((resolve, reject) => {
      const transaction = db.transaction('lyrics', 'readwrite')
      const store = transaction.objectStore('lyrics')
      const request = store.put(lyrics, songId)
      request.onsuccess = () => resolve()
      request.onerror = () => reject(request.error)
    })
  }

  async deleteCover(songId: string): Promise<void> {
    const db = await this.init()
    return new Promise((resolve, reject) => {
      const transaction = db.transaction('covers', 'readwrite')
      const store = transaction.objectStore('covers')
      const request = store.delete(songId)
      request.onsuccess = () => resolve()
      request.onerror = () => reject(request.error)
    })
  }

  async deleteLyrics(songId: string): Promise<void> {
    const db = await this.init()
    return new Promise((resolve, reject) => {
      const transaction = db.transaction('lyrics', 'readwrite')
      const store = transaction.objectStore('lyrics')
      const request = store.delete(songId)
      request.onsuccess = () => resolve()
      request.onerror = () => reject(request.error)
    })
  }

  async clearAll(): Promise<void> {
    const db = await this.init()
    return new Promise((resolve, reject) => {
      const transaction = db.transaction(['covers', 'lyrics', 'background'], 'readwrite')
      const coversStore = transaction.objectStore('covers')
      const lyricsStore = transaction.objectStore('lyrics')
      const bgStore = transaction.objectStore('background')

      let doneCount = 0
      const storeNames = ['covers', 'lyrics', 'background']
      const checkDone = () => {
        doneCount++
        if (doneCount >= storeNames.length) resolve()
      }

      const reqCovers = coversStore.clear()
      reqCovers.onsuccess = () => checkDone()
      reqCovers.onerror = () => reject(reqCovers.error)

      const reqLyrics = lyricsStore.clear()
      reqLyrics.onsuccess = () => checkDone()
      reqLyrics.onerror = () => reject(reqLyrics.error)

      const reqBg = bgStore.clear()
      reqBg.onsuccess = () => checkDone()
      reqBg.onerror = () => reject(reqBg.error)
    })
  }

  // --- 背景图片存储（blob + 时间戳统一存放在同一条记录） ---
  async getBackground(): Promise<{ blob: Blob; timestamp: string } | null> {
    const db = await this.init()
    return new Promise((resolve) => {
      const transaction = db.transaction('background', 'readonly')
      const store = transaction.objectStore('background')
      const request = store.get('local_bg')
      request.onsuccess = () => {
        const result = request.result
        if (!result) return resolve(null)
        resolve({ blob: result.blob, timestamp: result.timestamp || '' })
      }
      request.onerror = () => resolve(null)
    })
  }

  async saveBackground(blob: Blob, timestamp: string): Promise<void> {
    const db = await this.init()
    return new Promise((resolve, reject) => {
      const transaction = db.transaction('background', 'readwrite')
      const store = transaction.objectStore('background')
      const request = store.put({ blob, timestamp }, 'local_bg')
      request.onsuccess = () => resolve()
      request.onerror = () => reject(request.error)
    })
  }

  async updateBackgroundTimestamp(timestamp: string): Promise<void> {
    const db = await this.init()
    return new Promise((resolve, reject) => {
      const transaction = db.transaction('background', 'readwrite')
      const store = transaction.objectStore('background')
      const getReq = store.get('local_bg')
      getReq.onsuccess = () => {
        const existing = getReq.result
        if (!existing) return resolve()
        existing.timestamp = timestamp
        const putReq = store.put(existing, 'local_bg')
        putReq.onsuccess = () => resolve()
        putReq.onerror = () => reject(putReq.error)
      }
      getReq.onerror = () => reject(getReq.error)
    })
  }

  async deleteBackground(): Promise<void> {
    const db = await this.init()
    return new Promise((resolve, reject) => {
      const transaction = db.transaction('background', 'readwrite')
      const store = transaction.objectStore('background')
      const request = store.delete('local_bg')
      request.onsuccess = () => resolve()
      request.onerror = () => reject(request.error)
    })
  }
}

export const musicDB = new MusicDB()
