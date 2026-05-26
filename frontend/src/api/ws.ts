import { getBaseUrl } from '@/utils/path'

type WSMessageCallback = (data: any) => void

interface WSRequest {
  seq: number
  action: string
  data: any
}

interface WSResponse {
  seq?: number
  type: string
  action?: string
  success?: boolean
  data?: any
  error?: string
}

class WSClient {
  private ws: WebSocket | null = null
  private url: string = ''
  private callbacks: Map<string, Set<WSMessageCallback>> = new Map()
  private reconnectTimer: number | null = null
  private heartbeatTimer: number | null = null
  private isConnecting: boolean = false
  private seq = 0
  private pendingRequests: Map<number, { resolve: (val: any) => void; reject: (err: Error) => void; timer: number }> = new Map()
  
  // 离线/未连接请求排队队列
  private offlineQueue: Array<{
    action: string
    data: any
    resolve: (val: any) => void
    reject: (err: Error) => void
    seq: number
    timer: number
  }> = []

  private reconnectAttempt = 0
  private lastActiveTime = Date.now()

  constructor() {
    const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:'
    // 如果是开发服务器（例如 localhost:3000），开发代理会转发 /api/ws
    this.url = `${protocol}//${window.location.host}${getBaseUrl()}/api/ws`
  }

  connect() {
    if (this.ws || this.isConnecting) return
    this.isConnecting = true

    try {
      this.ws = new WebSocket(this.url)

      this.ws.onopen = () => {
        console.log('WebSocket connection established')
        this.isConnecting = false
        this.reconnectAttempt = 0 // 重置重连次数
        if (this.reconnectTimer) {
          clearTimeout(this.reconnectTimer)
          this.reconnectTimer = null
        }
        
        // 1. 成功建立连接后，依次冲刷发送离线排队队列中的请求
        const queue = [...this.offlineQueue]
        this.offlineQueue = []
        queue.forEach((item) => {
          if (this.ws && this.ws.readyState === WebSocket.OPEN) {
            console.log(`Sending queued request: ${item.action} (seq: ${item.seq})`)
            const payload: WSRequest = { seq: item.seq, action: item.action, data: item.data }
            this.pendingRequests.set(item.seq, { resolve: item.resolve, reject: item.reject, timer: item.timer })
            try {
              this.ws.send(JSON.stringify(payload))
            } catch (sendErr: any) {
              clearTimeout(item.timer)
              item.reject(sendErr)
              this.pendingRequests.delete(item.seq)
            }
          } else {
            clearTimeout(item.timer)
            item.reject(new Error('WebSocket disconnected during queue flush'))
          }
        })

        this.startHeartbeat()
        this.dispatch('open', null)
      }

      this.ws.onmessage = (event) => {
        this.lastActiveTime = Date.now() // 刷新活跃时间
        try {
          const payload: WSResponse = JSON.parse(event.data)
          if (!payload) return

          // 核心：请求-响应匹配
          if (payload.seq !== undefined && this.pendingRequests.has(payload.seq)) {
            const req = this.pendingRequests.get(payload.seq)!
            clearTimeout(req.timer)
            this.pendingRequests.delete(payload.seq)

            if (payload.success !== false) {
              req.resolve(payload.data)
            } else {
              req.reject(new Error(payload.error || 'Request failed'))
            }
            return
          }

          // 如果是 pong 消息，直接吞掉
          if (payload.type === 'pong') {
            return
          }

          // 普通广播消息
          if (payload.type) {
            this.dispatch(payload.type, payload.data)
          }
        } catch (e) {
          console.error('Failed to parse WebSocket message:', e)
        }
      }

      this.ws.onclose = () => {
        console.log('WebSocket connection closed, reconnecting...')
        this.cleanup()
        this.scheduleReconnect()
      }

      this.ws.onerror = (err) => {
        console.error('WebSocket error:', err)
        this.ws?.close()
      }
    } catch (e) {
      console.error('Failed to create WebSocket:', e)
      this.isConnecting = false
      this.scheduleReconnect()
    }
  }

  sendRequest(action: string, data: any = {}): Promise<any> {
    return new Promise((resolve, reject) => {
      const currentSeq = ++this.seq

      // 15 秒超时处理
      const timer = window.setTimeout(() => {
        // 在待挂起队列中检索并 reject
        if (this.pendingRequests.has(currentSeq)) {
          const req = this.pendingRequests.get(currentSeq)!
          req.reject(new Error(`WebSocket request timeout: ${action}`))
          this.pendingRequests.delete(currentSeq)
          return
        }

        // 如果仍在排队队列中，从排队队列中移除并 reject
        const queueIndex = this.offlineQueue.findIndex(item => item.seq === currentSeq)
        if (queueIndex !== -1) {
          const item = this.offlineQueue[queueIndex]
          item.reject(new Error(`WebSocket request timeout (queued): ${action}`))
          this.offlineQueue.splice(queueIndex, 1)
        }
      }, 15000)

      if (this.ws && this.ws.readyState === WebSocket.OPEN) {
        // 已建立连接，直接发送并记入待挂起 Map
        const payload: WSRequest = { seq: currentSeq, action, data }
        this.pendingRequests.set(currentSeq, { resolve, reject, timer })
        try {
          this.ws.send(JSON.stringify(payload))
        } catch (err: any) {
          clearTimeout(timer)
          reject(err)
          this.pendingRequests.delete(currentSeq)
        }
      } else {
        // 未建立连接（包括连接中和重连状态），放入离线等待队列
        console.log(`WebSocket not ready. Queueing request: ${action} (seq: ${currentSeq})`)
        this.offlineQueue.push({ action, data, resolve, reject, seq: currentSeq, timer })
        
        // 自动拉起连接，防止未调用 connect
        if (!this.ws && !this.isConnecting) {
          this.connect()
        }
      }
    })
  }

  subscribe(type: string, cb: WSMessageCallback) {
    if (!this.callbacks.has(type)) {
      this.callbacks.set(type, new Set())
    }
    this.callbacks.get(type)!.add(cb)

    return () => {
      this.callbacks.get(type)?.delete(cb)
    }
  }

  private dispatch(type: string, data: any) {
    const subs = this.callbacks.get(type)
    if (subs) {
      subs.forEach((cb) => {
        try {
          cb(data)
        } catch (e) {
          console.error(`Error in WS subscriber for ${type}:`, e)
        }
      })
    }
  }

  private startHeartbeat() {
    this.stopHeartbeat()
    this.lastActiveTime = Date.now()

    this.heartbeatTimer = window.setInterval(() => {
      if (!this.ws || this.ws.readyState !== WebSocket.OPEN) {
        this.stopHeartbeat()
        return
      }

      // 如果超过 40 秒没有收到任何消息（包括 pong 或广播），判定断线
      if (Date.now() - this.lastActiveTime > 40000) {
        console.warn('WebSocket heartbeat timeout, closing connection...')
        this.ws.close()
        return
      }

      // 发送 ping 帧
      try {
        this.ws.send(JSON.stringify({ action: 'ping' }))
      } catch (e) {
        console.error('Failed to send heartbeat ping:', e)
      }
    }, 20000) // 每 20 秒发送一次 ping
  }

  private stopHeartbeat() {
    if (this.heartbeatTimer) {
      clearInterval(this.heartbeatTimer)
      this.heartbeatTimer = null
    }
  }

  private scheduleReconnect() {
    if (this.reconnectTimer) return

    const baseDelay = 1000 // 基础 1 秒
    const maxDelay = 30000 // 最大 30 秒
    let delay = Math.min(maxDelay, baseDelay * Math.pow(2, this.reconnectAttempt))
    
    // 引入 0-30% 抖动 (Jitter)
    delay = delay + Math.random() * delay * 0.3

    console.log(`WebSocket reconnect scheduled in ${(delay / 1000).toFixed(2)}s (attempt ${this.reconnectAttempt + 1})`)

    this.reconnectTimer = window.setTimeout(() => {
      this.reconnectTimer = null
      this.reconnectAttempt++
      this.connect()
    }, delay)
  }

  private cleanup() {
    this.stopHeartbeat()
    if (this.ws) {
      this.ws.onopen = null
      this.ws.onmessage = null
      this.ws.onclose = null
      this.ws.onerror = null
      this.ws = null
    }
    this.isConnecting = false

    // 清理所有 pending 挂起请求并 reject
    this.pendingRequests.forEach((req) => {
      clearTimeout(req.timer)
      req.reject(new Error('WebSocket connection closed'))
    })
    this.pendingRequests.clear()

    // 清理所有离线等待队列并 reject
    this.offlineQueue.forEach((item) => {
      clearTimeout(item.timer)
      item.reject(new Error('WebSocket connection closed'))
    })
    this.offlineQueue = []
  }

  disconnect() {
    this.cleanup()
    if (this.reconnectTimer) {
      clearTimeout(this.reconnectTimer)
      this.reconnectTimer = null
    }
  }
}

export const wsClient = new WSClient()
